// Package runtimecap describes capabilities of the selected process profile.
// It does not enable features or change business settings.
package runtimecap

import "strings"

type Capability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type Features struct {
	ResellerSettlement Capability `json:"reseller_settlement"`
	Reconciliation     Capability `json:"reconciliation"`
	SystemSubdomain    Capability `json:"system_subdomain"`
	StockSyncSettings  Capability `json:"stock_sync_settings"`
	TelegramBroadcast  Capability `json:"telegram_broadcast"`
	InventoryAlerts    Capability `json:"inventory_alerts"`
	Redemption         Capability `json:"redemption"`
}

type Options struct {
	Mode                string
	QueueEnabled        bool
	ResellerEnabled     bool
	SubdomainBase       string
	AutoRechargeEnabled bool
}

func capability(available bool, reason string) Capability {
	if available {
		return Capability{Available: true}
	}
	return Capability{Reason: reason}
}

func Resolve(o Options) Features {
	full := o.Mode == "all" || o.Mode == "worker"
	commerce := o.QueueEnabled && (full || o.Mode == "storefront")
	features := Features{
		ResellerSettlement: capability(commerce && o.ResellerEnabled, "not_available"),
		Reconciliation:     capability(commerce, "not_available"),
		SystemSubdomain:    capability(o.ResellerEnabled && strings.TrimSpace(o.SubdomainBase) != "", "not_configured"),
		StockSyncSettings:  capability(full && o.QueueEnabled, "managed_externally"),
		TelegramBroadcast:  capability(full && o.QueueEnabled, "not_available"),
		InventoryAlerts:    capability(full && o.QueueEnabled, "not_available"),
		Redemption:         capability(full && o.AutoRechargeEnabled, "not_enabled"),
	}
	return features
}
