package chain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
)

const TronUSDTContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// TronAddressHex validates a mainnet-format Base58Check address and returns the
// 20-byte payload used in TVM event topics. Base58 is case sensitive.
func TronAddressHex(address string) (string, error) {
	if len(address) != 34 || address[0] != 'T' {
		return "", ErrRPC
	}
	n := new(big.Int)
	for _, ch := range address {
		index := strings.IndexRune(base58Alphabet, ch)
		if index < 0 {
			return "", ErrRPC
		}
		n.Mul(n, big.NewInt(58)).Add(n, big.NewInt(int64(index)))
	}
	b := n.Bytes()
	if len(b) != 25 || b[0] != 0x41 {
		return "", ErrRPC
	}
	first := sha256.Sum256(b[:21])
	checksum := sha256.Sum256(first[:])
	if !bytes.Equal(checksum[:4], b[21:]) {
		return "", ErrRPC
	}
	return "0x" + hex.EncodeToString(b[1:21]), nil
}

func TronAddressBase58(address string) (string, error) {
	if !IsHex(address, 20) {
		return "", ErrRPC
	}
	payload, _ := hex.DecodeString(address[2:])
	b := append([]byte{0x41}, payload...)
	first := sha256.Sum256(b)
	checksum := sha256.Sum256(first[:])
	b = append(b, checksum[:4]...)
	n, radix, remainder := new(big.Int).SetBytes(b), big.NewInt(58), new(big.Int)
	var out []byte
	for n.Sign() > 0 {
		n.QuoRem(n, radix, remainder)
		out = append(out, base58Alphabet[remainder.Int64()])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}

func (n Network) NormalizeRecipient(address string) (string, error) {
	address = strings.TrimSpace(address)
	if n.Name == "tron" {
		value, err := TronAddressHex(address)
		if err != nil || value == "0x"+strings.Repeat("0", 40) {
			return "", ErrRPC
		}
		return address, nil
	}
	address = strings.ToLower(address)
	if !IsHex(address, 20) || address == "0x"+strings.Repeat("0", 40) {
		return "", ErrRPC
	}
	return address, nil
}

func (n Network) TokenAddress() string {
	if n.Name == "tron" {
		return TronUSDTContract
	}
	return n.Token
}

func (n Network) DisplayAddress(address string) string {
	if n.Name == "tron" {
		if value, err := TronAddressBase58(address); err == nil {
			return value
		}
	}
	return address
}
