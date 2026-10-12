package config

// RPC credentials, if any, are supplied only in network-specific server
// environment variables (BSC_RPC_URL / PLASMA_RPC_URL).
type BSCUSDTConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Recipient     string `mapstructure:"recipient"`
	CNYPerUSDT    string `mapstructure:"cny_per_usdt"`
	Confirmations uint64 `mapstructure:"confirmations"`
}
