package chain

import "strings"

// Network fixes the chain and token together. Configuration may select a
// supported network, but cannot silently replace its token or decimal scale.
type Network struct {
	Name          string
	Provider      string
	ChannelType   string
	ChannelName   string
	ChainID       uint64
	Token         string
	TokenID       string
	Decimals      int32
	ScanBatchSize uint64
}

func ResolveNetwork(name string) (Network, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "bsc":
		// Catch up after an outage while the RPC adapter bounds individual log pages.
		return Network{Name: "bsc", Provider: Provider, ChannelType: "usdt-bep20", ChannelName: "USDT · BNB Smart Chain (BEP-20)", ChainID: ChainID, Token: Token, TokenID: "USDT", Decimals: 18, ScanBatchSize: 500}, nil
	case "plasma":
		// https://docs.usdt0.to/technical-documentation/deployments
		return Network{Name: "plasma", Provider: "plasmausdt", ChannelType: "usdt-plasma", ChannelName: "USDT · Plasma", ChainID: 9745, Token: "0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb", TokenID: "USDT0", Decimals: 6}, nil
	case "tron":
		return Network{Name: "tron", Provider: "tronusdt", ChannelType: "usdt-trc20", ChannelName: "USDT · TRON (TRC20)", ChainID: 728126428, Token: "0xa614f803b6fd780986a42c78ec9c7f77e6ded13c", TokenID: "USDT", Decimals: 6, ScanBatchSize: 10}, nil
	default:
		return Network{}, ErrRPC
	}
}
