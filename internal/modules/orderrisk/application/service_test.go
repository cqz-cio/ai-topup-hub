package application

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	orderriskcontract "github.com/dujiao-next/internal/modules/orderrisk/contract"
	orderriskdomain "github.com/dujiao-next/internal/modules/orderrisk/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type settingReaderStub struct {
	config settingssecurity.OrderRiskControlConfig
	err    error
}

func (s settingReaderStub) GetOrderRiskControlConfig() (settingssecurity.OrderRiskControlConfig, error) {
	return s.config, s.err
}

type countingSettingReader struct {
	config settingssecurity.OrderRiskControlConfig
	err    error
	calls  int
}

func (s *countingSettingReader) GetOrderRiskControlConfig() (settingssecurity.OrderRiskControlConfig, error) {
	s.calls++
	return s.config, s.err
}

type rateLimiterStub struct {
	calls  int
	input  orderriskcontract.CheckInput
	config settingssecurity.OrderRateLimitConfig
	err    error
}

func (s *rateLimiterStub) Check(input orderriskcontract.CheckInput, config settingssecurity.OrderRateLimitConfig) error {
	s.calls++
	s.input = input
	s.config = config
	return s.err
}

type pendingGateStub struct {
	lockedKeys       []string
	pendingByUser    int64
	pendingGuestIP   int64
	pendingMemberIP  int64
	pendingByProduct map[uint]int64
	err              error
}

func (s *pendingGateStub) LockRiskKeys(keys []string) error {
	s.lockedKeys = append([]string(nil), keys...)
	return s.err
}
func (s *pendingGateStub) CountPendingByUserID(uint) (int64, error) {
	return s.pendingByUser, s.err
}
func (s *pendingGateStub) CountPendingGuestByRiskIP(string) (int64, error) {
	return s.pendingGuestIP, s.err
}
func (s *pendingGateStub) CountPendingMemberByRiskIP(string) (int64, error) {
	return s.pendingMemberIP, s.err
}
func (s *pendingGateStub) SumPendingGuestQuantityByRiskIP(string, []uint) (map[uint]int64, error) {
	return s.pendingByProduct, s.err
}

func testConfig() settingssecurity.OrderRiskControlConfig {
	cfg := settingssecurity.DefaultOrderRiskControlConfig()
	cfg.Enabled = true
	return cfg
}

func TestCheckOrderAllowed_DisabledByDefault(t *testing.T) {
	svc := NewService(Options{Settings: settingReaderStub{config: settingssecurity.DefaultOrderRiskControlConfig()}})
	result, err := svc.CheckOrderAllowed(orderriskcontract.CheckInput{
		IsGuest:  true,
		ClientIP: "2001:db8:abcd:12::1234",
		Items:    []orderriskcontract.OrderItem{{ProductID: 1, Quantity: 999}},
	})
	if err != nil {
		t.Fatalf("expected nil when globally disabled, got %v", err)
	}
	if result.RiskIP != "2001:db8:abcd:12::/64" {
		t.Fatalf("expected normalized risk IP even while disabled, got %q", result.RiskIP)
	}
}

func TestCheckPendingOrderAllowedDoesNotReadSettingsInsideSingleConnectionTransaction(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "risk-single-connection.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&settingsstore.SettingRecord{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderriskdomain.LockKey{},
	); err != nil {
		t.Fatalf("migrate risk tables: %v", err)
	}
	cfg := testConfig()
	if err := db.Create(&settingsstore.SettingRecord{
		Key:       constants.SettingKeyOrderRiskControlConfig,
		ValueJSON: settingssecurity.EncodeOrderRiskControlConfig(cfg),
	}).Error; err != nil {
		t.Fatalf("seed enabled risk config: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	settings := settingsapp.NewService(settingsstore.New(db))
	svc := NewService(Options{Settings: settings})
	orders := orderstore.New(db, "risk-single-connection-secret-with-32-bytes")
	input := orderriskcontract.CheckInput{
		IsGuest:  true,
		ClientIP: "1.2.3.4",
		Items:    []orderriskcontract.OrderItem{{ProductID: 7, Quantity: 1}},
	}
	prepared, err := svc.CheckOrderAllowed(input)
	if err != nil {
		t.Fatalf("prepare risk check: %v", err)
	}
	if !prepared.ConfigSnapshot.Enabled {
		t.Fatal("expected enabled risk config snapshot")
	}

	done := make(chan error, 1)
	go func() {
		done <- orders.WithinTransaction(func(tx ordercontract.Transaction) error {
			return svc.CheckPendingOrderAllowed(input, prepared, tx.Orders())
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("pending risk check: %v", err)
		}
	case <-time.After(2 * time.Second):
		stats := sqlDB.Stats()
		// 释放旧实现中等待第二条连接的 goroutine，避免失败测试泄漏。
		sqlDB.SetMaxOpenConns(2)
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatalf("pending risk check blocked inside transaction: in_use=%d open=%d wait_count=%d", stats.InUse, stats.OpenConnections, stats.WaitCount)
	}
}

func TestPendingRiskCheckReusesPreparedConfigSnapshot(t *testing.T) {
	cfg := testConfig()
	settings := &countingSettingReader{config: cfg}
	svc := NewService(Options{Settings: settings})
	input := orderriskcontract.CheckInput{IsGuest: true, ClientIP: "1.2.3.4"}

	prepared, err := svc.CheckOrderAllowed(input)
	if err != nil {
		t.Fatalf("prepare risk check: %v", err)
	}
	if err := svc.CheckPendingOrderAllowed(input, prepared, &pendingGateStub{}); err != nil {
		t.Fatalf("pending risk check: %v", err)
	}
	if settings.calls != 1 {
		t.Fatalf("settings reads=%d, want exactly one before the transaction", settings.calls)
	}
}

func TestCheckOrderAllowed_SettingsReadFailureFailsClosedOnlyForCreate(t *testing.T) {
	settingsErr := errors.New("settings unavailable")
	svc := NewService(Options{Settings: settingReaderStub{err: settingsErr}})

	preview, err := svc.CheckOrderAllowed(orderriskcontract.CheckInput{
		IsGuest:  true,
		ClientIP: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("preview-style check should preserve fail-open behavior, got %v", err)
	}
	if preview.ConfigSnapshot.Enabled {
		t.Fatal("preview fallback config must keep risk control disabled")
	}

	_, err = svc.CheckOrderAllowed(orderriskcontract.CheckInput{
		IsGuest:          true,
		ClientIP:         "1.2.3.4",
		ConsumeRateLimit: true,
	})
	if !errors.Is(err, settingsErr) {
		t.Fatalf("create-style check must fail closed, got %v", err)
	}
}

func TestCheckOrderAllowed_IPBlacklistCanonicalizesAddress(t *testing.T) {
	cfg := testConfig()
	cfg.Common.IPBlacklist = []string{"1.2.3.4", "10.0.0.0/8"}
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}})

	for _, ip := range []string{"1.2.3.4", "::ffff:1.2.3.4", "10.2.3.4"} {
		if _, err := svc.CheckOrderAllowed(orderriskcontract.CheckInput{IsGuest: true, ClientIP: ip}); !errors.Is(err, orderriskcontract.ErrIPBlacklisted) {
			t.Fatalf("expected %q to be blacklisted, got %v", ip, err)
		}
	}
}

func TestCheckOrderAllowed_GuestQuantityAggregatesProductAcrossSKUs(t *testing.T) {
	cfg := testConfig()
	cfg.Guest.MaxQuantityPerProductPerOrder = 2
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}})

	_, err := svc.CheckOrderAllowed(orderriskcontract.CheckInput{
		IsGuest:  true,
		ClientIP: "1.2.3.4",
		Items: []orderriskcontract.OrderItem{
			{ProductID: 7, Quantity: 1},
			{ProductID: 7, Quantity: 2},
		},
	})
	if !errors.Is(err, orderriskcontract.ErrProductQuantityLimit) {
		t.Fatalf("expected aggregate guest quantity to be rejected, got %v", err)
	}
}

func TestCheckOrderAllowed_MemberUsesIndependentQuantityPolicy(t *testing.T) {
	cfg := testConfig()
	cfg.Guest.MaxQuantityPerProductPerOrder = 1
	cfg.Member.MaxQuantityPerProductPerOrder = 5
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}})

	if _, err := svc.CheckOrderAllowed(orderriskcontract.CheckInput{
		UserID: 3,
		Items:  []orderriskcontract.OrderItem{{ProductID: 1, Quantity: 5}},
	}); err != nil {
		t.Fatalf("member quantity must not use guest limit: %v", err)
	}
	if _, err := svc.CheckOrderAllowed(orderriskcontract.CheckInput{
		UserID: 3,
		Items:  []orderriskcontract.OrderItem{{ProductID: 1, Quantity: 6}},
	}); !errors.Is(err, orderriskcontract.ErrProductQuantityLimit) {
		t.Fatalf("expected member quantity limit, got %v", err)
	}
}

func TestCheckOrderAllowed_GuestRateLimitUsesRiskIPOnlyWhenConsuming(t *testing.T) {
	cfg := testConfig()
	limited := &orderriskcontract.RateLimitedError{RetryAfter: 42}
	limiter := &rateLimiterStub{err: limited}
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}, RateLimiter: limiter})
	input := orderriskcontract.CheckInput{IsGuest: true, ClientIP: "2001:db8:1:2::abcd"}

	if _, err := svc.CheckOrderAllowed(input); err != nil {
		t.Fatalf("preview-style check must not consume rate limit: %v", err)
	}
	if limiter.calls != 0 {
		t.Fatalf("expected no rate limiter call, got %d", limiter.calls)
	}
	input.ConsumeRateLimit = true
	if _, err := svc.CheckOrderAllowed(input); err != limited {
		t.Fatalf("expected limiter error, got %v", err)
	}
	if limiter.calls != 1 || limiter.input.RiskIP != "2001:db8:1:2::/64" || limiter.input.UserID != 0 {
		t.Fatalf("unexpected guest limiter input: calls=%d input=%+v", limiter.calls, limiter.input)
	}
}

func TestCheckPendingOrderAllowed_GuestLocksIPAndCountsOrders(t *testing.T) {
	cfg := testConfig()
	cfg.Guest.MaxPendingOrdersPerIP = 2
	gate := &pendingGateStub{pendingGuestIP: 2}
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}})

	err := svc.CheckPendingOrderAllowed(orderriskcontract.CheckInput{
		IsGuest: true,
		RiskIP:  "1.2.3.4",
		Items:   []orderriskcontract.OrderItem{{ProductID: 1, Quantity: 1}},
	}, orderriskcontract.CheckResult{ConfigSnapshot: cfg}, gate)
	if !errors.Is(err, orderriskcontract.ErrTooManyPendingOrders) {
		t.Fatalf("expected guest pending order limit, got %v", err)
	}
	if !reflect.DeepEqual(gate.lockedKeys, []string{"guest:ip:1.2.3.4"}) {
		t.Fatalf("unexpected lock keys: %#v", gate.lockedKeys)
	}
}

func TestCheckPendingOrderAllowed_GuestCountsPendingProductQuantity(t *testing.T) {
	cfg := testConfig()
	cfg.Guest.MaxPendingOrdersPerIP = 10
	cfg.Guest.MaxPendingQuantityPerIPProduct = 2
	gate := &pendingGateStub{pendingByProduct: map[uint]int64{7: 1}}
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}})

	err := svc.CheckPendingOrderAllowed(orderriskcontract.CheckInput{
		IsGuest: true,
		RiskIP:  "1.2.3.4",
		Items: []orderriskcontract.OrderItem{
			{ProductID: 7, Quantity: 1},
			{ProductID: 7, Quantity: 1},
		},
	}, orderriskcontract.CheckResult{ConfigSnapshot: cfg}, gate)
	if !errors.Is(err, orderriskcontract.ErrPendingProductQuantityLimit) {
		t.Fatalf("expected pending product quantity limit, got %v", err)
	}
}

func TestCheckPendingOrderAllowed_MemberUsesUserAndOptionalIP(t *testing.T) {
	cfg := testConfig()
	cfg.Member.MaxPendingOrdersPerUser = 5
	cfg.Member.MaxPendingOrdersPerIP = 3
	gate := &pendingGateStub{pendingByUser: 4, pendingMemberIP: 3}
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}})

	err := svc.CheckPendingOrderAllowed(
		orderriskcontract.CheckInput{UserID: 9, RiskIP: "1.2.3.4"},
		orderriskcontract.CheckResult{ConfigSnapshot: cfg},
		gate,
	)
	if !errors.Is(err, orderriskcontract.ErrTooManyPendingOrders) {
		t.Fatalf("expected member IP pending limit, got %v", err)
	}
	if !reflect.DeepEqual(gate.lockedKeys, []string{"member:user:9", "member:ip:1.2.3.4"}) {
		t.Fatalf("unexpected member lock keys: %#v", gate.lockedKeys)
	}
}

func TestCheckOrderAllowed_GuestReturnsExpiryOverride(t *testing.T) {
	cfg := testConfig()
	cfg.Guest.PaymentExpireMinutes = 8
	svc := NewService(Options{Settings: settingReaderStub{config: cfg}})

	result, err := svc.CheckOrderAllowed(orderriskcontract.CheckInput{IsGuest: true, ClientIP: "1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	if result.PaymentExpireMinutes != 8 {
		t.Fatalf("expected guest expiry 8, got %d", result.PaymentExpireMinutes)
	}
}
