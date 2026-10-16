package main

import (
	"flag"
	"github.com/dujiao-next/internal/modules/supportagent"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

func main() {
	configPath := flag.String("config", "work/codex-runtime/support-agent/private/config.json", "private local configuration")
	flag.Parse()
	config, err := supportagent.ReadConfig(*configPath)
	if err != nil {
		log.Fatal("support agent: private configuration invalid or unavailable")
	}
	ledger, err := supportagent.OpenLedger(filepath.Join(filepath.Dir(*configPath), "usage.db"))
	if err != nil {
		log.Fatal("support agent: usage ledger unavailable")
	}
	defer ledger.Close()
	service := supportagent.NewService(config, supportagent.NewDeepSeek(config), supportagent.NewCatalog(), ledger)
	server := &http.Server{Addr: "127.0.0.1:8085", Handler: service, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 50 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	log.Print("local support agent listening on 127.0.0.1:8085; credentials and messages are never logged")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("support agent listener stopped")
	}
}
