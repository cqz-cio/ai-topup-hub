package config

// RPC credentials, if any, are supplied only in BSC_RPC_URL on the server.
type BSCUSDTConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Recipient     string `mapstructure:"recipient"`
	CNYPerUSDT    string `mapstructure:"cny_per_usdt"`
	Confirmations uint64 `mapstructure:"confirmations"`
}
