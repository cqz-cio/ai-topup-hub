package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dujiao-next/internal/queue"
	"github.com/glebarez/sqlite"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type queueStub struct {
	tasks map[string]queue.NotificationDispatchPayload
	fail  bool
}

func (q *queueStub) EnqueueNotificationDispatch(p queue.NotificationDispatchPayload, options ...asynq.Option) error {
	if q.fail {
		return errors.New("queue unavailable")
	}
	var id string
	for _, option := range options {
		if option.Type() == asynq.TaskIDOpt {
			id = option.Value().(string)
		}
	}
	if id == "" || p.EventType != "order_paid_success" || p.BizType != "order" || p.Force {
		panic("unsafe notification task")
	}
	if _, ok := q.tasks[id]; ok {
		return asynq.ErrTaskIDConflict
	}
	q.tasks[id] = p
	return nil
}

func fixture(t *testing.T) (*gorm.DB, repairPlan, *queueStub) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "repair.db")), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })
	statements := []string{
		`CREATE TABLE users(id INTEGER PRIMARY KEY, email TEXT, display_name TEXT, total_spent NUMERIC, deleted_at DATETIME)`,
		`CREATE TABLE orders(id INTEGER PRIMARY KEY, order_no TEXT, user_id INTEGER, parent_id INTEGER, status TEXT, currency TEXT, total_amount NUMERIC, refunded_amount NUMERIC, paid_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE order_items(id INTEGER PRIMARY KEY, order_id INTEGER, fulfillment_type TEXT, quantity INTEGER)`,
		`CREATE TABLE payments(id INTEGER PRIMARY KEY, order_id INTEGER, status TEXT, provider_type TEXT, channel_type TEXT, deleted_at DATETIME)`,
		`CREATE TABLE procurement_orders(id INTEGER PRIMARY KEY, local_order_id INTEGER, status TEXT, upstream_order_no TEXT, deleted_at DATETIME)`,
		`CREATE TABLE member_levels(id INTEGER PRIMARY KEY)`,
		`CREATE TABLE settings(key TEXT PRIMARY KEY, value_json TEXT)`,
		`CREATE TABLE notification_logs(id INTEGER PRIMARY KEY, event_type TEXT, biz_type TEXT, biz_id INTEGER, channel TEXT, recipient TEXT, status TEXT, is_test BOOLEAN)`,
		`CREATE TABLE wallet_accounts(id INTEGER PRIMARY KEY, balance NUMERIC)`,
		`CREATE TABLE wallet_transactions(id INTEGER PRIMARY KEY, amount NUMERIC)`,
		`INSERT INTO users VALUES(13,'customer@example.invalid','Customer',0,NULL)`,
		`INSERT INTO orders VALUES(41,'O41',13,NULL,'delivered','CNY',130,0,'2026-10-16 14:53:26',NULL),(43,'O43',13,NULL,'delivered','CNY',115,0,'2026-10-16 16:23:17',NULL)`,
		`INSERT INTO order_items VALUES(1,41,'upstream',1),(2,43,'upstream',1)`,
		`INSERT INTO payments VALUES(19,41,'success','xorpay','native',NULL),(20,43,'success','xorpay','native',NULL)`,
		`INSERT INTO procurement_orders VALUES(6,41,'fulfilled','SUP6',NULL),(7,43,'fulfilled','SUP7',NULL)`,
		`INSERT INTO settings VALUES('smtp_config','{"enabled":true}'),('notification_center_config','{"default_locale":"zh-CN","scenes":{"order_paid_success":true},"channels":{"email":{"enabled":true,"recipients":["merchant@example.invalid"]}}}')`,
		`INSERT INTO wallet_accounts VALUES(1,123.45)`,
		`INSERT INTO wallet_transactions VALUES(1,123.45)`,
	}
	for _, sql := range statements {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, repairPlan{Incident: "2026-10-16-paid-order-deadlock", UserID: 13, Before: "0", After: "245", Orders: []repairOrder{{41, "O41", 19, "130"}, {43, "O43", 20, "115"}}}, &queueStub{tasks: map[string]queue.NotificationDispatchPayload{}}
}

func snapshot(t *testing.T, db *gorm.DB, includeUser bool) string {
	t.Helper()
	result := map[string]interface{}{}
	tables := []string{"orders", "order_items", "payments", "procurement_orders", "wallet_accounts", "wallet_transactions", "member_levels", "settings", "notification_logs"}
	if includeUser {
		tables = append(tables, "users")
	}
	for _, table := range tables {
		var rows []map[string]interface{}
		if err := db.Table(table).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		result[table] = rows
	}
	data, _ := json.Marshal(result)
	return string(data)
}

func TestDryRunHasNoWritesOrMessages(t *testing.T) {
	db, plan, q := fixture(t)
	before := snapshot(t, db, true)
	r, err := repair(db, q, plan, false, "")
	if err != nil || r.Updated || len(r.Emails) != 2 || len(q.tasks) != 0 || before != snapshot(t, db, true) {
		t.Fatalf("dry-run mutated state: %+v %v", r, err)
	}
}

func TestRepairAndRepeatOnlyChangeStatisticAndQueueTwoEmails(t *testing.T) {
	db, plan, q := fixture(t)
	before := snapshot(t, db, false)
	for i := 0; i < 2; i++ {
		r, err := repair(db, q, plan, true, "false")
		if err != nil || r.Updated != (i == 0) || len(q.tasks) != 2 {
			t.Fatalf("attempt %d: %+v %v", i, r, err)
		}
	}
	var amount string
	db.Table("users").Select("total_spent").Where("id=13").Scan(&amount)
	if amount != "245" || before != snapshot(t, db, false) {
		t.Fatal("protected data changed or statistic incorrect")
	}
	for _, p := range q.tasks {
		if p.Data["item_count"] != "1" || p.Data["customer_email"] != "customer@example.invalid" {
			t.Fatal("incomplete merchant payload")
		}
	}
}

func TestQueueFailureCanResumeWithoutDoubleCounting(t *testing.T) {
	db, plan, q := fixture(t)
	q.fail = true
	if _, err := repair(db, q, plan, true, "false"); err == nil {
		t.Fatal("expected queue failure")
	}
	q.fail = false
	r, err := repair(db, q, plan, true, "false")
	if err != nil || r.Updated || len(q.tasks) != 2 {
		t.Fatalf("unsafe resume: %+v %v", r, err)
	}
}

func TestSuccessfulMailIsNeverRequeued(t *testing.T) {
	db, plan, q := fixture(t)
	db.Exec(`INSERT INTO notification_logs VALUES(1,'order_paid_success','order',41,'email','merchant@example.invalid','success',0)`)
	r, err := repair(db, q, plan, true, "false")
	if err != nil || len(q.tasks) != 1 || r.Emails[0].State != "already_sent" {
		t.Fatalf("sent email was repeated: %+v %v", r, err)
	}
}

func TestUnexpectedStateAbortsBeforeWritesOrEnqueue(t *testing.T) {
	for name, change := range map[string]string{
		"statistic conflict":     `UPDATE users SET total_spent=5`,
		"unpaid":                 `UPDATE orders SET paid_at=NULL WHERE id=43`,
		"refund":                 `UPDATE orders SET refunded_amount=1 WHERE id=43`,
		"other customer":         `UPDATE orders SET user_id=14 WHERE id=43`,
		"failed payment":         `UPDATE payments SET status='failed' WHERE id=20`,
		"incomplete procurement": `UPDATE procurement_orders SET status='pending' WHERE id=7`,
		"changed paid set":       `INSERT INTO orders VALUES(45,'O45',13,NULL,'delivered','CNY',1,0,'2026-10-16 17:00:00',NULL)`,
		"member tier":            `INSERT INTO member_levels VALUES(1)`,
		"recipient changed":      `UPDATE settings SET value_json='{"enabled":false}' WHERE key='smtp_config'`,
	} {
		t.Run(name, func(t *testing.T) {
			db, plan, q := fixture(t)
			if err := db.Exec(change).Error; err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, db, true)
			if _, err := repair(db, q, plan, true, "false"); err == nil {
				t.Fatal("expected refusal")
			}
			if before != snapshot(t, db, true) || len(q.tasks) != 0 {
				t.Fatal("changed data on refusal")
			}
		})
	}
}

func TestPurchaseShutdownIsMandatory(t *testing.T) {
	for _, flag := range []string{"", "true", "False"} {
		db, plan, q := fixture(t)
		before := snapshot(t, db, true)
		if _, err := repair(db, q, plan, true, flag); err == nil {
			t.Fatal("accepted absent explicit purchase shutdown")
		}
		if before != snapshot(t, db, true) || len(q.tasks) != 0 {
			t.Fatal("changed protected state")
		}
	}
}
