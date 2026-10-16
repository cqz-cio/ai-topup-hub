package supportagent

import (
	"database/sql"
	"errors"
	_ "github.com/glebarez/go-sqlite"
	"sync"
	"time"
)

var ErrBudget = errors.New("support daily allowance reached")
var ErrDuplicate = errors.New("request already reserved")

// The ledger stores only opaque IDs, counts, usage and cost; never conversation
// text or secrets. Reservations survive cancellation, unknown results and restarts.
type Ledger struct {
	db *sql.DB
	mu sync.Mutex
}

func OpenLedger(path string) (*Ledger, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA busy_timeout=3000; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;
 CREATE TABLE IF NOT EXISTS support_usage (
 id TEXT PRIMARY KEY, day TEXT NOT NULL, session TEXT NOT NULL,
 cost INTEGER NOT NULL, input INTEGER NOT NULL DEFAULT 0,
 output INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL DEFAULT 'reserved');
 CREATE INDEX IF NOT EXISTS support_usage_day ON support_usage(day);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Ledger{db: db}, nil
}
func (l *Ledger) Close() error { return l.db.Close() }
func localDay(now time.Time) string {
	return now.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02")
}

func (l *Ledger) Reserve(id, session string, reserve int64, c Config, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var found int
	err = tx.QueryRow("SELECT COUNT(*) FROM support_usage WHERE id=?", id).Scan(&found)
	if err != nil {
		return err
	}
	if found != 0 {
		return ErrDuplicate
	}
	var cost int64
	var count, sessionCount int
	err = tx.QueryRow("SELECT COALESCE(SUM(cost),0), COUNT(*), COALESCE(SUM(CASE WHEN session=? THEN 1 ELSE 0 END),0) FROM support_usage WHERE day=?", session, localDay(now)).Scan(&cost, &count, &sessionCount)
	if err != nil {
		return err
	}
	if reserve <= 0 || cost+reserve > c.DailyBudget || count >= c.DailyCalls || sessionCount >= c.SessionDailyCalls {
		return ErrBudget
	}
	if _, err = tx.Exec("INSERT INTO support_usage(id,day,session,cost) VALUES(?,?,?,?)", id, localDay(now), session, reserve); err != nil {
		return err
	}
	return tx.Commit()
}

// Peak uncached Flash prices checked 2026-10-16: $0.30/M input and $1.20/M
// output. Cache/off-peak savings are deliberately ignored. This is an estimate,
// not the provider bill. Unknown usage retains the full reservation.
func usageCost(input, output int) int64 { return int64(input)*300 + int64(output)*1200 }
func (l *Ledger) Settle(id string, input, output int) error {
	if input <= 0 || output < 0 || input > 100000 || output > 10000 {
		return errors.New("untrusted usage")
	}
	_, err := l.db.Exec("UPDATE support_usage SET cost=?,input=?,output=?,state='completed' WHERE id=? AND state='reserved'", usageCost(input, output), input, output, id)
	return err
}
