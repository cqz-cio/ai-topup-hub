package main

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Application JSON values are stored as BLOBs. The production SQLite driver's
// LIKE implementation does not match them unless they are cast to text.
func TestProbeFindsStoredReceiptAndPreventsRepeatedSend(t *testing.T) {
	for _, storage := range []string{"blob", "text"} {
		t.Run(storage, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			conn, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetMaxOpenConns(1)
			if _, err := conn.Exec("create table notification_logs(id integer primary key, status text, is_test numeric, biz_type text, variables_json json, created_at datetime)"); err != nil {
				t.Fatal(err)
			}
			const token = "LOCAL-MAIL-RECEIPT"
			var variables any = `{"order_no":"MAILTEST-LOCAL-MAIL-RECEIPT"}`
			if storage == "blob" {
				variables = []byte(variables.(string))
			}
			accepted := time.Date(2026, 10, 16, 22, 26, 58, 928227000, time.FixedZone("CST", 8*3600))
			if _, err := conn.Exec("insert into notification_logs values(1,'success',1,'mail_latency_test',?,?)", variables, accepted); err != nil {
				t.Fatal(err)
			}
			var prior int
			if err := conn.QueryRow(priorProbeQuery, token).Scan(&prior); err != nil || prior != 1 {
				t.Fatalf("duplicate-send guard: count=%d err=%v", prior, err)
			}
			var id uint
			var status string
			var received time.Time
			if err := conn.QueryRow(probeReceiptQuery, token).Scan(&id, &status, &received); err != nil {
				t.Fatal(err)
			}
			if id != 1 || status != "success" || !received.Equal(accepted) {
				t.Fatalf("receipt mismatch: %d %s %s", id, status, received)
			}
			if err := conn.QueryRow(priorProbeQuery, "ANOTHER-TOKEN").Scan(&prior); err != nil || prior != 0 {
				t.Fatalf("unrelated test matched: count=%d err=%v", prior, err)
			}
		})
	}
}
