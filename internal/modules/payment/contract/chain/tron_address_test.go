package chain

import (
	"strings"
	"testing"
)

func TestTronAddressChecksumsAndCase(t *testing.T) {
	n, _ := ResolveNetwork("tron")
	address, err := n.NormalizeRecipient(TronUSDTContract)
	if err != nil || address != TronUSDTContract {
		t.Fatal("valid Base58 rejected")
	}
	hex, err := TronAddressHex(address)
	if err != nil || hex != n.Token {
		t.Fatal("wrong token payload")
	}
	back, err := TronAddressBase58(hex)
	if err != nil || back != address {
		t.Fatal("address roundtrip changed")
	}
	for _, invalid := range []string{strings.ToLower(address), address[:33] + "a", n.Token, "", "T" + strings.Repeat("0", 33)} {
		if _, err := n.NormalizeRecipient(invalid); err == nil {
			t.Fatal("invalid Base58 accepted", invalid)
		}
	}
	if AddressTopic(address) != AddressTopic(n.Token) {
		t.Fatal("TVM topic encoding mismatch")
	}
	if n.TokenAddress() != TronUSDTContract || n.Decimals != 6 {
		t.Fatal("incorrect network identity")
	}
}
