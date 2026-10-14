package runtimecap

import "testing"

func TestStorefrontAvailability(t *testing.T) {
	f := Resolve(Options{Mode: "storefront", QueueEnabled: true, ResellerEnabled: true})
	if !f.ResellerSettlement.Available || !f.Reconciliation.Available {
		t.Fatal("commerce jobs must be available")
	}
	for name, c := range map[string]Capability{"domain": f.SystemSubdomain, "stock": f.StockSyncSettings, "broadcast": f.TelegramBroadcast, "inventory": f.InventoryAlerts, "redemption": f.Redemption} {
		if c.Available || c.Reason == "" {
			t.Fatalf("%s incorrectly enabled: %+v", name, c)
		}
	}
}

func TestUnavailableRuntimeFailsClosed(t *testing.T) {
	for _, mode := range []string{"", "api", "checkout", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			f := Resolve(Options{Mode: mode, QueueEnabled: true, ResellerEnabled: true, AutoRechargeEnabled: true})
			if f.ResellerSettlement.Available || f.Reconciliation.Available || f.TelegramBroadcast.Available || f.InventoryAlerts.Available || f.Redemption.Available {
				t.Fatalf("unsupported mode advertised background features: %+v", f)
			}
		})
	}
	f := Resolve(Options{Mode: "storefront", ResellerEnabled: true})
	if f.Reconciliation.Available || f.ResellerSettlement.Available {
		t.Fatal("queue disabled")
	}
}

func TestConfiguredFullRuntime(t *testing.T) {
	f := Resolve(Options{Mode: "all", QueueEnabled: true, ResellerEnabled: true, SubdomainBase: " shop.example.test ", AutoRechargeEnabled: true})
	if !f.SystemSubdomain.Available || !f.StockSyncSettings.Available || !f.Redemption.Available {
		t.Fatalf("configured features unavailable: %+v", f)
	}
	if Resolve(Options{Mode: "all", ResellerEnabled: true, SubdomainBase: " "}).SystemSubdomain.Available {
		t.Fatal("blank subdomain configured")
	}
}
