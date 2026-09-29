package gormstore

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestOrderStoreRequiresGuestCredentialSecret(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New must reject an empty guest credential secret")
		}
	}()
	_ = New(openOrderTenantScopeTestDB(t), "")
}

func TestGuestCredentialIsHashedAtRestAndRawCredentialStillQueries(t *testing.T) {
	db := openOrderTenantScopeTestDB(t)
	repo := New(db, "test-guest-credential-secret-with-32-bytes")
	order := &orderdomain.Order{
		OrderNo:       "GUEST-HASHED",
		GuestEmail:    "guest@example.com",
		GuestPassword: "guest-password",
		Status:        constants.OrderStatusPendingPayment,
		Currency:      "USD",
		TotalAmount:   money.FromDecimal(decimal.NewFromInt(10)),
	}
	if err := repo.WithinTransaction(func(tx ordercontract.Transaction) error {
		return tx.Orders().Create(order, nil)
	}); err != nil {
		t.Fatalf("transactional Create failed: %v", err)
	}

	var stored orderdomain.Order
	if err := db.Select("id", "guest_password").First(&stored, order.ID).Error; err != nil {
		t.Fatalf("reload stored order: %v", err)
	}
	if stored.GuestPassword == "guest-password" || !strings.HasPrefix(stored.GuestPassword, guestCredentialHashPrefix) {
		t.Fatalf("guest credential was not irreversibly hashed at rest: %q", stored.GuestPassword)
	}

	got, err := repo.GetByOrderNoAndGuest("GUEST-HASHED", "guest@example.com", "guest-password")
	if err != nil || got == nil {
		t.Fatalf("raw credential should still authenticate, got order=%v err=%v", got, err)
	}
	got, err = repo.GetByOrderNoAndGuest("GUEST-HASHED", "guest@example.com", "wrong-password")
	if err != nil {
		t.Fatalf("wrong credential query failed unexpectedly: %v", err)
	}
	if got != nil {
		t.Fatalf("wrong credential must not authenticate")
	}
}

func TestGuestCredentialHashLikeRawPasswordCannotBypassHashing(t *testing.T) {
	db := openOrderTenantScopeTestDB(t)
	repo := New(db, "test-guest-credential-secret-with-32-bytes")
	rawPassword := guestCredentialHashPrefix + strings.Repeat("a", 64)
	order := &orderdomain.Order{
		OrderNo:       "GUEST-HASH-LIKE-PASSWORD",
		GuestEmail:    "hash-like@example.com",
		GuestPassword: rawPassword,
		Status:        constants.OrderStatusPendingPayment,
		Currency:      "USD",
		TotalAmount:   money.FromDecimal(decimal.NewFromInt(10)),
	}
	if err := repo.Create(order, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var stored orderdomain.Order
	if err := db.Select("id", "guest_password").First(&stored, order.ID).Error; err != nil {
		t.Fatalf("reload stored order: %v", err)
	}
	if stored.GuestPassword == rawPassword || !isGuestCredentialHash(stored.GuestPassword) {
		t.Fatalf("hash-shaped raw password bypassed hashing: %q", stored.GuestPassword)
	}
	got, err := repo.GetByOrderNoAndGuest(order.OrderNo, order.GuestEmail, rawPassword)
	if err != nil || got == nil {
		t.Fatalf("hash-shaped raw password should authenticate after hashing, got order=%v err=%v", got, err)
	}
}

func TestBackfillGuestCredentialHashesMigratesLegacyPlaintext(t *testing.T) {
	db := openOrderTenantScopeTestDB(t)
	legacy := seedScopedOrder(t, db, "GUEST-LEGACY", 0, "legacy@example.com", "legacy-password", constants.OrderStatusPendingPayment, nil, nil)
	prefixedLegacy := seedScopedOrder(t, db, "GUEST-PREFIXED-LEGACY", 0, "prefixed@example.com", guestCredentialHashPrefix+"legacy-password", constants.OrderStatusPendingPayment, nil, nil)
	repo := New(db, "test-guest-credential-secret-with-32-bytes")

	migrated, err := repo.BackfillGuestCredentialHashes()
	if err != nil {
		t.Fatalf("BackfillGuestCredentialHashes failed: %v", err)
	}
	if migrated != 2 {
		t.Fatalf("migrated = %d, want 2", migrated)
	}

	var stored orderdomain.Order
	if err := db.Select("id", "guest_password").First(&stored, legacy.ID).Error; err != nil {
		t.Fatalf("reload migrated order: %v", err)
	}
	if stored.GuestPassword == "legacy-password" || !strings.HasPrefix(stored.GuestPassword, guestCredentialHashPrefix) {
		t.Fatalf("legacy credential was not hashed: %q", stored.GuestPassword)
	}
	got, err := repo.GetByOrderNoAndGuest("GUEST-LEGACY", "legacy@example.com", "legacy-password")
	if err != nil || got == nil {
		t.Fatalf("migrated credential should authenticate, got order=%v err=%v", got, err)
	}
	got, err = repo.GetByOrderNoAndGuest("GUEST-PREFIXED-LEGACY", "prefixed@example.com", guestCredentialHashPrefix+"legacy-password")
	if err != nil || got == nil {
		t.Fatalf("prefixed legacy credential should be migrated and authenticate, got order=%v err=%v", got, err)
	}
	var prefixedStored orderdomain.Order
	if err := db.Select("id", "guest_password").First(&prefixedStored, prefixedLegacy.ID).Error; err != nil {
		t.Fatalf("reload prefixed migrated order: %v", err)
	}
	if prefixedStored.GuestPassword == guestCredentialHashPrefix+"legacy-password" || !isGuestCredentialHash(prefixedStored.GuestPassword) {
		t.Fatalf("prefixed legacy credential was not hashed: %q", prefixedStored.GuestPassword)
	}
}

func TestBackfillGuestCredentialHashesProcessesBatchesAndSkipsMigratedRows(t *testing.T) {
	db := openOrderTenantScopeTestDB(t)
	repo := New(db, "test-guest-credential-secret-with-32-bytes")
	const plaintextCount = 501
	rows := make([]orderdomain.Order, 0, plaintextCount+1)
	for i := 0; i < plaintextCount; i++ {
		rows = append(rows, orderdomain.Order{
			OrderNo:       fmt.Sprintf("GUEST-BATCH-%04d", i),
			GuestEmail:    fmt.Sprintf("guest-%04d@example.com", i),
			GuestPassword: fmt.Sprintf("plain-%04d", i),
			Status:        constants.OrderStatusPendingPayment,
			Currency:      "USD",
			TotalAmount:   money.FromDecimal(decimal.NewFromInt(10)),
		})
	}
	migratedHash := repo.hashGuestCredential("already@example.com", "already-hashed")
	rows = append(rows, orderdomain.Order{
		OrderNo:       "GUEST-ALREADY-HASHED",
		GuestEmail:    "already@example.com",
		GuestPassword: migratedHash,
		Status:        constants.OrderStatusPendingPayment,
		Currency:      "USD",
		TotalAmount:   money.FromDecimal(decimal.NewFromInt(10)),
	})
	if err := db.CreateInBatches(&rows, 100).Error; err != nil {
		t.Fatalf("seed guest credential batches failed: %v", err)
	}

	migrated, err := repo.BackfillGuestCredentialHashes()
	if err != nil {
		t.Fatalf("BackfillGuestCredentialHashes failed: %v", err)
	}
	if migrated != plaintextCount {
		t.Fatalf("migrated = %d, want %d", migrated, plaintextCount)
	}

	var plaintextRows int64
	if err := db.Model(&orderdomain.Order{}).
		Where("user_id = 0 AND guest_password NOT LIKE ?", guestCredentialHashPrefix+"%").
		Count(&plaintextRows).Error; err != nil {
		t.Fatalf("count plaintext rows failed: %v", err)
	}
	if plaintextRows != 0 {
		t.Fatalf("plaintext rows remaining = %d", plaintextRows)
	}
	var stored orderdomain.Order
	if err := db.Where("order_no = ?", "GUEST-ALREADY-HASHED").First(&stored).Error; err != nil {
		t.Fatalf("reload already migrated order failed: %v", err)
	}
	if stored.GuestPassword != migratedHash {
		t.Fatal("backfill must not rehash an already migrated credential")
	}
}

func TestBackfillGuestCredentialHashesMigratesMalformedHashShapedPlaintext(t *testing.T) {
	db := openOrderTenantScopeTestDB(t)
	repo := New(db, "test-guest-credential-secret-with-32-bytes")
	malformed := guestCredentialHashPrefix + strings.Repeat("z", sha256.Size*2)
	order := orderdomain.Order{
		OrderNo:       "GUEST-MALFORMED-HASH-SHAPE",
		UserID:        0,
		GuestEmail:    "legacy@example.com",
		GuestPassword: malformed,
		Status:        constants.OrderStatusPendingPayment,
		Currency:      "CNY",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed malformed guest credential: %v", err)
	}

	migrated, err := repo.BackfillGuestCredentialHashes()
	if err != nil {
		t.Fatalf("BackfillGuestCredentialHashes failed: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("migrated = %d, want 1", migrated)
	}

	var stored orderdomain.Order
	if err := db.First(&stored, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if stored.GuestPassword == malformed {
		t.Fatal("malformed hash-shaped plaintext was not migrated")
	}
	if !isGuestCredentialHash(stored.GuestPassword) {
		t.Fatalf("migrated credential is not a valid hash: %q", stored.GuestPassword)
	}
}

// TestGuestCredentialBackfillCandidateSQLInlinesIntegerLiterals 锁定候选集 SQL 的形状。
// PostgreSQL 会把 SUBSTRING(x FROM $n) 里的未知类型占位符按 substring(text, text)
// 正则重载推断成 text，pgx 随后无法把整数编码成 text（cannot find encode plan）。
// 因此长度与起始位置必须内联成 SQL 字面量，绑定参数只能是字符串。
func TestGuestCredentialBackfillCandidateSQLInlinesIntegerLiterals(t *testing.T) {
	expectedLength := len(guestCredentialHashPrefix) + sha256.Size*2
	hashStart := len(guestCredentialHashPrefix) + 1
	prefixPattern := guestCredentialHashPrefix + "%"

	cases := []struct {
		name     string
		dialect  string
		wantSQL  string
		wantArgs []interface{}
	}{
		{
			name:    "postgres",
			dialect: "postgres",
			wantSQL: fmt.Sprintf(
				"(guest_password NOT LIKE ? OR LENGTH(guest_password) <> %d OR SUBSTRING(guest_password FROM %d) !~ ?)",
				expectedLength, hashStart),
			wantArgs: []interface{}{prefixPattern, "^[0-9A-Fa-f]+$"},
		},
		{
			name:    "sqlite",
			dialect: "sqlite",
			wantSQL: fmt.Sprintf(
				"(guest_password NOT LIKE ? OR LENGTH(guest_password) <> %d OR SUBSTR(guest_password, %d) GLOB ?)",
				expectedLength, hashStart),
			wantArgs: []interface{}{prefixPattern, "*[^0-9A-Fa-f]*"},
		},
		{
			name:     "unknown dialect scans every guest credential",
			dialect:  "mysql",
			wantSQL:  "1 = 1",
			wantArgs: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSQL, gotArgs := guestCredentialBackfillCandidateSQL(tc.dialect)
			if gotSQL != tc.wantSQL {
				t.Fatalf("SQL mismatch\n got: %s\nwant: %s", gotSQL, tc.wantSQL)
			}
			if len(gotArgs) != len(tc.wantArgs) {
				t.Fatalf("args length = %d, want %d (%v)", len(gotArgs), len(tc.wantArgs), gotArgs)
			}
			for i, want := range tc.wantArgs {
				if gotArgs[i] != want {
					t.Fatalf("args[%d] = %v, want %v", i, gotArgs[i], want)
				}
			}
			if placeholders := strings.Count(gotSQL, "?"); placeholders != len(gotArgs) {
				t.Fatalf("placeholder count = %d, but %d args supplied", placeholders, len(gotArgs))
			}
			// 回归点：任何非字符串绑定参数都会在 PostgreSQL 上触发编码失败。
			for i, arg := range gotArgs {
				if _, ok := arg.(string); !ok {
					t.Fatalf("args[%d] = %#v is not a string; integers must be inlined as SQL literals", i, arg)
				}
			}
		})
	}
}

// openGuestCredentialPostgresDB 在独立 schema 内准备一套订单表，避免污染 DSN 指向的库。
// 连接池限制为 1，SET search_path 才能对后续所有查询持续生效。
func openGuestCredentialPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sql.DB failed: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	schema := fmt.Sprintf("guest_credential_bf_%d", time.Now().UnixNano())
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema %s failed: %v", schema, err)
	}
	t.Cleanup(func() {
		if err := db.Exec("SET search_path TO public").Error; err != nil {
			t.Logf("reset search_path failed: %v", err)
		}
		if err := db.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error; err != nil {
			t.Logf("drop schema %s failed: %v", schema, err)
		}
		_ = sqlDB.Close()
	})
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatalf("set search_path to %s failed: %v", schema, err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("migrate orders in schema %s failed: %v", schema, err)
	}
	return db
}

func seedPostgresGuestOrder(t *testing.T, db *gorm.DB, orderNo, guestEmail, guestPassword string) orderdomain.Order {
	t.Helper()
	order := orderdomain.Order{
		OrderNo:       orderNo,
		UserID:        0,
		GuestEmail:    guestEmail,
		GuestPassword: guestPassword,
		Status:        constants.OrderStatusPendingPayment,
		Currency:      "USD",
		TotalAmount:   money.FromDecimal(decimal.NewFromInt(10)),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order %s failed: %v", orderNo, err)
	}
	return order
}

// TestBackfillGuestCredentialHashesOnPostgres 是 pgx 参数编码失败的端到端回归测试。
// 候选集 SQL 一旦把整数改回绑定参数，这里会直接以
// "unable to encode 13 into text format for text (OID 25)" 失败。
func TestBackfillGuestCredentialHashesOnPostgres(t *testing.T) {
	db := openGuestCredentialPostgresDB(t)
	repo := New(db, "test-guest-credential-secret-with-32-bytes")

	plaintext := seedPostgresGuestOrder(t, db, "PG-GUEST-PLAINTEXT", "plain@example.com", "legacy-password")
	prefixed := seedPostgresGuestOrder(t, db, "PG-GUEST-PREFIXED", "prefixed@example.com", guestCredentialHashPrefix+"legacy-password")
	// 前缀与长度都对、只有摘要体不是十六进制 —— 唯一必须走 SUBSTRING 正则分支才能捞出的形态。
	malformed := seedPostgresGuestOrder(t, db, "PG-GUEST-MALFORMED", "malformed@example.com", guestCredentialHashPrefix+strings.Repeat("z", sha256.Size*2))
	alreadyHashed := repo.hashGuestCredential("hashed@example.com", "already-hashed")
	untouched := seedPostgresGuestOrder(t, db, "PG-GUEST-HASHED", "hashed@example.com", alreadyHashed)

	migrated, err := repo.BackfillGuestCredentialHashes()
	if err != nil {
		t.Fatalf("BackfillGuestCredentialHashes on postgres failed: %v", err)
	}
	if migrated != 3 {
		t.Fatalf("migrated = %d, want 3", migrated)
	}

	for _, tc := range []struct {
		name string
		id   uint
		was  string
	}{
		{"plaintext", plaintext.ID, "legacy-password"},
		{"prefixed plaintext", prefixed.ID, guestCredentialHashPrefix + "legacy-password"},
		{"malformed hash shape", malformed.ID, guestCredentialHashPrefix + strings.Repeat("z", sha256.Size*2)},
	} {
		var stored orderdomain.Order
		if err := db.Select("id", "guest_password").First(&stored, tc.id).Error; err != nil {
			t.Fatalf("reload %s order failed: %v", tc.name, err)
		}
		if stored.GuestPassword == tc.was {
			t.Fatalf("%s credential was not migrated: %q", tc.name, stored.GuestPassword)
		}
		if !isGuestCredentialHash(stored.GuestPassword) {
			t.Fatalf("%s credential is not a valid hash: %q", tc.name, stored.GuestPassword)
		}
	}

	var untouchedStored orderdomain.Order
	if err := db.Select("id", "guest_password").First(&untouchedStored, untouched.ID).Error; err != nil {
		t.Fatalf("reload already hashed order failed: %v", err)
	}
	if untouchedStored.GuestPassword != alreadyHashed {
		t.Fatal("backfill must not rehash an already migrated credential")
	}

	got, err := repo.GetByOrderNoAndGuest("PG-GUEST-PLAINTEXT", "plain@example.com", "legacy-password")
	if err != nil || got == nil {
		t.Fatalf("migrated credential should still authenticate, got order=%v err=%v", got, err)
	}
}
