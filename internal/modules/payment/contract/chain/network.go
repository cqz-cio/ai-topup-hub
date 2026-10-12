package chain

import "strings"

// Network fixes the chain and token together. Configuration may select a
// supported network, but cannot silently replace its token or decimal scale.
type Network struct {
	Name        string
	Provider    string
	ChannelType string
	ChannelName string
	ChainID     uint64
	Token       string
	TokenID     string
	Decimals    int32
}

func ResolveNetwork(name string) (Network, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "bsc":
		return Network{Name: "bsc", Provider: Provider, ChannelType: "usdt-bep20", ChannelName: "USDT · BNB Smart Chain (BEP-20)", ChainID: ChainID, Token: Token, TokenID: "USDT", Decimals: 18}, nil
	case "plasma":
		// https://docs.usdt0.to/technical-documentation/deployments
		return Network{Name: "plasma", Provider: "plasmausdt", ChannelType: "usdt-plasma", ChannelName: "USDT · Plasma", ChainID: 9745, Token: "0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb", TokenID: "USDT0", Decimals: 6}, nil
	default:
		return Network{}, ErrRPC
	}
}
