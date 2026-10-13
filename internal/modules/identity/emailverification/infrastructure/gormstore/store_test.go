package emailverificationstore

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := fmt.Sprintf("file:email_verification_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&emailverificationdomain.Code{}); err != nil {
		t.Fatalf("migrate verification code failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return New(db)
}

func TestStoreReturnsLatestActiveCodeAndPersistsAttempts(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	older := &emailverificationdomain.Code{
		Email: "buyer@example.com", Purpose: "login", Code: "111111",
		ExpiresAt: now.Add(time.Minute), SentAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute),
	}
	latest := &emailverificationdomain.Code{
		Email: "buyer@example.com", Purpose: "login", Code: "222222",
		ExpiresAt: now.Add(time.Minute), SentAt: now, CreatedAt: now,
	}
	deletedAt := now.Add(time.Second)
	deleted := &emailverificationdomain.Code{
		Email: "buyer@example.com", Purpose: "login", Code: "333333",
		ExpiresAt: now.Add(time.Minute), SentAt: now.Add(time.Minute), CreatedAt: now.Add(time.Minute), DeletedAt: &deletedAt,
	}
	for _, code := range []*emailverificationdomain.Code{older, latest, deleted} {
		if err := store.Create(code); err != nil {
			t.Fatalf("create verification code failed: %v", err)
		}
	}

	got, err := store.GetLatest("buyer@example.com", "login")
	if err != nil {
		t.Fatalf("get latest verification code failed: %v", err)
	}
	if got == nil || got.ID != latest.ID {
		t.Fatalf("unexpected latest verification code: %#v", got)
	}
	if accepted, err := store.ApplyVerificationAttempt(got.ID, false, now, 5); err != nil || !accepted {
		t.Fatalf("increment attempt failed: %v", err)
	}
	verifiedAt := now.Add(2 * time.Second)
	if accepted, err := store.ApplyVerificationAttempt(got.ID, true, verifiedAt, 5); err != nil || !accepted {
		t.Fatalf("mark verified failed: %v", err)
	}
	refreshed, err := store.GetLatest("buyer@example.com", "login")
	if err != nil {
		t.Fatalf("reload verification code failed: %v", err)
	}
	if refreshed.AttemptCount != 2 || refreshed.VerifiedAt == nil || !refreshed.VerifiedAt.Equal(verifiedAt) {
		t.Fatalf("verification state was not persisted: %#v", refreshed)
	}
}

func TestConcurrentCodeAttemptsAreBoundedAndSingleUse(t *testing.T) {
	for _, matches := range []bool{false, true} {
		store := newTestStore(t)
		now := time.Now()
		record := &emailverificationdomain.Code{Email: "buyer@example.com", Purpose: "register", Code: "123456", ExpiresAt: now.Add(time.Minute), SentAt: now}
		if err := store.Create(record); err != nil {
			t.Fatal(err)
		}
		var accepted atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, err := store.ApplyVerificationAttempt(record.ID, matches, now, 5)
				if err != nil {
					t.Errorf("attempt: %v", err)
				}
				if ok {
					accepted.Add(1)
				}
			}()
		}
		wg.Wait()
		want := int32(5)
		if matches {
			want = 1
		}
		if accepted.Load() != want {
			t.Fatalf("matches=%v accepted=%d want=%d", matches, accepted.Load(), want)
		}
		if ok, err := store.ApplyVerificationAttempt(record.ID, true, now, 5); err != nil || ok {
			t.Fatalf("consumed/exhausted code accepted: %v %v", ok, err)
		}
	}
}

func TestCodeMustBeLatestAndUnexpiredAtConsumption(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()
	old := &emailverificationdomain.Code{Email: "buyer@example.com", Purpose: "register", Code: "123456", ExpiresAt: now.Add(time.Minute), SentAt: now}
	if err := store.Create(old); err != nil {
		t.Fatal(err)
	}
	newer := *old
	newer.ID = 0
	if err := store.Create(&newer); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id uint
		at time.Time
	}{{old.ID, now}, {newer.ID, newer.ExpiresAt}} {
		ok, err := store.ApplyVerificationAttempt(tc.id, true, tc.at, 5)
		if err != nil || ok {
			t.Fatalf("stale/expired accepted=%v err=%v", ok, err)
		}
	}
}
