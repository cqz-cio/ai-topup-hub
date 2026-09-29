package config

type AutoRechargeConfig struct {
	RedeemURL string                `mapstructure:"redeem_url"`
	Enabled   bool                  `mapstructure:"enabled"`
	Bindings  []AutoRechargeBinding `mapstructure:"bindings"`
}
type AutoRechargeBinding struct {
	SKUID              uint   `mapstructure:"sku_id"`
	Plan               string `mapstructure:"plan"`
	Region             string `mapstructure:"region"`
	RegionVersion      int    `mapstructure:"region_version"`
	Channel            string `mapstructure:"channel"`
	CardRule           string `mapstructure:"card_rule"`
	CancelAfterSuccess bool   `mapstructure:"cancel_after_success"`
}
