// paid-order-repair repairs a reviewed historical incident without booting the
// application, invoking payment hooks, or exposing any purchase/refund method.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/queue"
	"github.com/glebarez/sqlite"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "existing server configuration")
	planPath := flag.String("plan", "", "reviewed incident plan JSON")
	apply := flag.Bool("apply", false, "apply statistic repair and enqueue missing merchant emails")
	flag.Parse()
	if *configPath == "" || *planPath == "" {
		return errors.New("config and plan are required; default is read-only")
	}
	v := viper.New()
	v.SetConfigFile(*configPath)
	if err := v.ReadInConfig(); err != nil {
		return errors.New("cannot read configuration")
	}
	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return errors.New("invalid configuration")
	}
	if cfg.Database.Driver != "sqlite" || !cfg.Queue.Enabled {
		return errors.New("requires existing SQLite database and notification queue")
	}
	path, err := filepath.Abs(cfg.Database.DSN)
	if err != nil {
		return err
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return errors.New("existing database file required")
	}
	mode := "ro"
	if *apply {
		mode = "rw"
	}
	db, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(path)+"?mode="+mode), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return errors.New("cannot open database")
	}
	conn, err := db.DB()
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	contents, err := os.ReadFile(*planPath)
	if err != nil {
		return err
	}
	var plan repairPlan
	if err := json.Unmarshal(contents, &plan); err != nil {
		return errors.New("invalid plan")
	}
	var notifier notificationQueue
	if *apply {
		client, err := queue.NewClient(&cfg.Queue)
		if err != nil {
			return errors.New("cannot initialize notification queue")
		}
		defer client.Close()
		if !client.Enabled() {
			return errors.New("notification queue is disabled")
		}
		notifier = client
	}
	report, err := repair(db, notifier, plan, *apply, os.Getenv("UPSTREAM_PURCHASE_ENABLED"))
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
