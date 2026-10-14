package container

import "github.com/dujiao-next/internal/platform/runtimecap"

func (c *Container) RuntimeFeatures() runtimecap.Features {
	if c == nil || c.Config == nil {
		return runtimecap.Resolve(runtimecap.Options{})
	}
	return runtimecap.Resolve(runtimecap.Options{
		Mode:                c.RuntimeMode,
		QueueEnabled:        c.QueueClient != nil && c.QueueClient.Enabled(),
		ResellerEnabled:     c.Config.Reseller.Enabled,
		SubdomainBase:       c.Config.Reseller.SubdomainBase,
		AutoRechargeEnabled: c.AutoRechargeService != nil,
	})
}
