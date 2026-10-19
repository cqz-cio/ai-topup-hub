// mail-latency-check sends clearly labelled probes only to the configured
// merchant mailbox. It opens SQLite read-only and has no business action hooks.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	notificationsmtp "github.com/dujiao-next/internal/modules/notification/infrastructure/smtp"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/mailbrand"
	"github.com/glebarez/sqlite"
	"github.com/hibiken/asynq"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type probe struct {
	Kind       string    `json:"kind"`
	StartedAt  time.Time `json:"started_at"`
	AcceptedAt time.Time `json:"accepted_at"`
	ElapsedMS  int64     `json:"elapsed_ms"`
	LogID      uint      `json:"log_id,omitempty"`
}

const priorProbeQuery = "select count(*) from notification_logs where is_test=1 and instr(cast(variables_json as text),?)>0"
const probeReceiptQuery = "select id,status,created_at from notification_logs where is_test=1 and biz_type='mail_latency_test' and instr(cast(variables_json as text),?)>0 order by id desc limit 1"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "existing configuration")
	send := flag.Bool("send", false, "send three labelled mail probes to the existing merchant recipient")
	token := flag.String("token", "", "unique test identifier (letters, digits and hyphens)")
	flag.Parse()
	if *configPath == "" {
		return errors.New("configuration required")
	}
	if *send {
		if len(*token) < 8 || len(*token) > 60 || strings.Trim(*token, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-") != "" {
			return errors.New("valid unique test token required")
		}
	}
	v := viper.New()
	v.SetConfigFile(*configPath)
	if err := v.ReadInConfig(); err != nil {
		return errors.New("configuration unreadable")
	}
	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return errors.New("invalid configuration")
	}
	if cfg.Database.Driver != "sqlite" || !cfg.Queue.Enabled {
		return errors.New("existing SQLite and queue required")
	}
	path, err := filepath.Abs(cfg.Database.DSN)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return errors.New("database missing")
	}
	db, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(path)+"?mode=ro"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return errors.New("database unreadable")
	}
	conn, err := db.DB()
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	read := func(key string, dest interface{}) error {
		var raw string
		if err := conn.QueryRow("select value_json from settings where key=?", key).Scan(&raw); err != nil {
			return errors.New("mail settings missing")
		}
		return json.Unmarshal([]byte(raw), dest)
	}
	var smtpSetting settingsmessaging.SMTPSetting
	var center settingsmessaging.NotificationCenterSetting
	if err := read(constants.SettingKeySMTPConfig, &smtpSetting); err != nil {
		return err
	}
	if err := read(constants.SettingKeyNotificationCenterConfig, &center); err != nil {
		return err
	}
	if !smtpSetting.Enabled || !center.Channels.Email.Enabled || !center.Scenes.OrderPaidSuccess || len(center.Channels.Email.Recipients) != 1 {
		return errors.New("requires enabled SMTP and one configured merchant recipient")
	}
	if !*send {
		return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"ready": true, "recipient_count": 1, "read_only": true})
	}
	var prior int
	if err := conn.QueryRow(priorProbeQuery, *token).Scan(&prior); err != nil {
		return err
	}
	if prior > 0 {
		return errors.New("test token already used; refusing duplicate test send")
	}
	client, err := queue.NewClient(&cfg.Queue)
	if err != nil {
		return err
	}
	defer client.Close()
	started := time.Now().UTC()
	payload := queue.NotificationDispatchPayload{EnqueuedAt: started, IsTest: true, Force: true, EventType: constants.NotificationEventOrderPaidSuccess, BizType: "mail_latency_test", Locale: constants.LocaleZhCN, Data: map[string]interface{}{
		"order_no": "MAILTEST-" + *token, "amount": "0.00", "currency": "CNY", "payment_channel": "邮件测速测试",
		"items_summary":    "仅验证邮件速度，没有创建订单、付款或采购。",
		"upstream_summary": "【邮件测速测试】此邮件不对应真实交易。标识：" + *token,
	}}
	if err := client.EnqueueNotificationDispatch(payload, asynq.TaskID("mail-latency-"+*token), asynq.Retention(time.Hour)); err != nil {
		return errors.New("cannot queue mail probe")
	}
	var reports []probe
	mailCfg := settingsmessaging.SMTPSettingToConfig(smtpSetting)
	sender := notificationsmtp.New(&mailCfg)
	brand := mailbrand.Brand{SiteName: "邮件测速测试 " + *token, SiteURL: "https://www.qzweb.cloud"}
	target := center.Channels.Email.Recipients[0]
	for _, kind := range []string{"verification", "delivery_attachment"} {
		begin := time.Now().UTC()
		if kind == "verification" {
			err = sender.SendVerifyCode(target, "TEST-NOT-VALID", "register", constants.LocaleZhCN, brand)
		} else {
			template := settingsmessaging.DefaultOrderEmailTemplateSetting()
			template.Templates.Delivered.ZHCN = settingsmessaging.OrderEmailLocalizedTemplate{
				Subject: "【邮件测速测试】交付附件 " + *token,
				Body:    "仅验证交付邮件及附件，没有创建订单、付款或采购。标识：" + *token,
			}
			err = sender.SendOrderStatusEmailWithTemplate(target, notificationcontract.OrderStatusEmailInput{OrderNo: "MAILTEST-" + *token, Status: constants.OrderStatusCompleted, Currency: "CNY", SiteName: brand.SiteName, SiteURL: brand.SiteURL, MailBrand: brand, AttachmentName: "mail-speed-test.txt", AttachmentContent: "这是邮件测速附件，不是卡密；没有创建订单、付款或采购。标识：" + *token}, constants.LocaleZhCN, &template)
		}
		if err != nil {
			return fmt.Errorf("%s probe failed: %w", kind, err)
		}
		end := time.Now().UTC()
		reports = append(reports, probe{Kind: kind, StartedAt: begin, AcceptedAt: end, ElapsedMS: end.Sub(begin).Milliseconds()})
	}
	for time.Since(started) < 55*time.Second {
		var id uint
		var status string
		var accepted time.Time
		err = conn.QueryRow(probeReceiptQuery, *token).Scan(&id, &status, &accepted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("cannot read mail receipt: %w", err)
		}
		if err == nil && status == "success" {
			reports = append(reports, probe{Kind: "merchant_queue", StartedAt: started, AcceptedAt: accepted.UTC(), ElapsedMS: accepted.Sub(started).Milliseconds(), LogID: id})
			return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"token": *token, "probes": reports, "test_mail_only": true, "database_mode": "read-only", "orders_created": 0, "purchases_submitted": 0})
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("merchant queue did not record acceptance within 55 seconds; inspect existing probe instead of resending")
}
