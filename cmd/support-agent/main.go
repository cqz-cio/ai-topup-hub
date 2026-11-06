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
	allowModel := flag.Bool("allow-model", false, "explicitly allow paid model calls (disabled by default)")
	publicOrigin := flag.String("public-origin", "", "exact HTTPS origin served by the local reverse proxy")
	flag.Parse()
	config := supportagent.OfflineConfig()
	var ledger *supportagent.Ledger
	var provider supportagent.Provider
	if *publicOrigin != "" && (!supportagent.ValidatePublicOrigin(*publicOrigin) || *allowModel) {
		log.Fatal("support agent: public mode requires an exact HTTPS origin and offline rules")
	}
	if *allowModel {
		var err error
		config, err = supportagent.ReadConfig(*configPath)
		if err != nil {
			log.Fatal("support agent: private configuration invalid or unavailable")
		}
		config.AllowModel = true
		ledger, err = supportagent.OpenLedger(filepath.Join(filepath.Dir(*configPath), "usage.db"))
		if err != nil {
			log.Fatal("support agent: usage ledger unavailable")
		}
		defer ledger.Close()
		provider = supportagent.NewDeepSeek(config)
	}
	config.PublicOrigin = *publicOrigin
	service := supportagent.NewService(config, provider, supportagent.NewCatalog(), ledger)
	server := &http.Server{Addr: "127.0.0.1:8085", Handler: service, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 50 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	log.Print("local support agent listening on 127.0.0.1:8085; credentials and messages are never logged")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("support agent listener stopped")
	}
}
