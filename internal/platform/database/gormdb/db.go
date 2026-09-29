package gormdb

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite" // 纯 Go SQLite 驱动（基于 modernc.org/sqlite）
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// DBPoolConfig 数据库连接池配置
type DBPoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
	ConnMaxIdleTimeSeconds int
}

// InitDB 初始化数据库连接
func InitDB(driver, dsn string, pool DBPoolConfig, mode string) error {
	var err error
	normalized := strings.ToLower(strings.TrimSpace(driver))
	var dialector gorm.Dialector
	switch normalized {
	case "", "sqlite":
		// glebarez/sqlite 是基于 modernc.org/sqlite 的纯 Go 驱动
		// 追加 PRAGMA 参数避免并发写入时 busy-spin 导致 CPU 飙升
		dialector = sqlite.Open(appendSQLitePragmas(dsn))
	case "postgres", "postgresql":
		dialector = postgres.Open(dsn)
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger:  newGORMLogger(mode, nil),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	applyDBPool(sqlDB, pool)

	// SQLite 额外执行 PRAGMA 确保 WAL 模式生效
	if normalized == "" || normalized == "sqlite" {
		DB.Exec("PRAGMA journal_mode=WAL")
		DB.Exec("PRAGMA busy_timeout=5000")
		DB.Exec("PRAGMA synchronous=NORMAL")
	}
	return nil
}

func newGORMLogger(mode string, writer gormlogger.Writer) gormlogger.Interface {
	if !strings.EqualFold(strings.TrimSpace(mode), "release") {
		return gormlogger.Default.LogMode(gormlogger.Info)
	}
	if writer == nil {
		writer = log.New(os.Stdout, "\r\n", log.LstdFlags)
	}
	return gormlogger.New(writer, gormlogger.Config{
		SlowThreshold:             2 * time.Second,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})
}

// appendSQLitePragmas 在 SQLite DSN 中追加关键 PRAGMA 参数
func appendSQLitePragmas(dsn string) string {
	// glebarez/sqlite 支持 ?_pragma=key=value 形式
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep +
		"_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)"
}

func applyDBPool(sqlDB *sql.DB, pool DBPoolConfig) {
	if sqlDB == nil {
		return
	}
	if pool.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns >= 0 {
		sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetimeSeconds > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(pool.ConnMaxLifetimeSeconds) * time.Second)
	}
	if pool.ConnMaxIdleTimeSeconds > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(pool.ConnMaxIdleTimeSeconds) * time.Second)
	}
}
