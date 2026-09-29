package container

import (
	"fmt"
	rechargebootstrap "github.com/dujiao-next/internal/bootstrap/autorecharge"
	rechargeapp "github.com/dujiao-next/internal/modules/autorecharge/application"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/infrastructure/aicdk"
	rechargestore "github.com/dujiao-next/internal/modules/autorecharge/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"net/url"
	"os"
)

func (c *Container) initAutoRecharge() error {
	if !c.Config.AutoRecharge.Enabled {
		return nil
	}
	u, err := url.Parse(c.Config.AutoRecharge.RedeemURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("auto_recharge.redeem_url must be an HTTPS redemption page without credentials or query parameters")
	}
	bindings := make([]contract.Binding, 0, len(c.Config.AutoRecharge.Bindings))
	skus := make(map[uint]bool)
	for _, b := range c.Config.AutoRecharge.Bindings {
		skus[b.SKUID] = true
		bindings = append(bindings, contract.Binding{SKUID: b.SKUID, Plan: b.Plan, Region: b.Region, RegionVersion: b.RegionVersion, Channel: b.Channel, CardRule: b.CardRule, CancelAfterSuccess: b.CancelAfterSuccess})
	}
	svc, err := rechargeapp.New(rechargeapp.Options{Store: rechargestore.New(gormdb.DB), Orders: &rechargebootstrap.Orders{DB: gormdb.DB, Store: c.OrderStore, Fulfillment: c.FulfillmentService, RechargeSKUs: skus, RedeemURL: c.Config.AutoRecharge.RedeemURL}, Partner: aicdk.New(os.Getenv("AICDK_API_KEY")), Secret: c.Config.App.SecretKey, Bindings: bindings})
	if err != nil {
		return err
	}
	c.AutoRechargeService = svc
	c.PaymentService.SetAutoRechargeService(svc)
	return nil
}
