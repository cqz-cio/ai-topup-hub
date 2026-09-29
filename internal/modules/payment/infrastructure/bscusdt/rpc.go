package bscusdt

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Token = chain.Token
const TransferTopic = chain.TransferTopic
const Provider = chain.Provider
const ChainID = chain.ChainID

var ErrRPC = chain.ErrRPC

var quantity = chain.Quantity
var hexBlock = chain.HexBlock
var isHex = chain.IsHex
var addressTopic = chain.AddressTopic

type RPC struct {
	endpoint string
	client   *http.Client
}

func NewRPC(endpoint string) (*RPC, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, ErrRPC
	}
	return &RPC{endpoint: endpoint, client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (r *RPC) call(ctx context.Context, method string, params interface{}, out interface{}) error {
	raw, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return ErrRPC
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(raw))
	if err != nil {
		return ErrRPC
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return ErrRPC
	} // Endpoint URLs can contain API keys.
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrRPC
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil || len(body) > 8<<20 {
		return ErrRPC
	}
	var envelope struct {
		ID      int             `json:"id"`
		Version string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.ID != 1 || envelope.Version != "2.0" || (len(envelope.Error) > 0 && string(envelope.Error) != "null") || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return ErrRPC
	}
	if json.Unmarshal(envelope.Result, out) != nil {
		return ErrRPC
	}
	return nil
}
func (r *RPC) Check(ctx context.Context) error {
	var chain, decimals string
	if err := r.call(ctx, "eth_chainId", []interface{}{}, &chain); err != nil {
		return err
	}
	n, err := quantity(chain)
	if err != nil || n != ChainID {
		return ErrRPC
	}
	if err = r.call(ctx, "eth_call", []interface{}{map[string]string{"to": Token, "data": "0x313ce567"}, "latest"}, &decimals); err != nil {
		return err
	}
	if !isHex(decimals, 32) || decimals != "0x"+strings.Repeat("0", 62)+"12" {
		return ErrRPC
	}
	return nil
}
func (r *RPC) Header(ctx context.Context, tag string) (*chain.Header, error) {
	var h chain.Header
	if err := r.call(ctx, "eth_getBlockByNumber", []interface{}{tag, false}, &h); err != nil {
		return nil, err
	}
	number, err := quantity(h.Number)
	if err != nil {
		return nil, ErrRPC
	}
	if strings.HasPrefix(tag, "0x") {
		requested, parseErr := quantity(tag)
		if parseErr != nil || requested != number {
			return nil, ErrRPC
		}
	}
	if _, err := quantity(h.Timestamp); err != nil || !isHex(h.Hash, 32) {
		return nil, ErrRPC
	}
	return &h, nil
}
func (r *RPC) Logs(ctx context.Context, from, to uint64, recipient string) ([]chain.Log, error) {
	var logs []chain.Log
	err := r.call(ctx, "eth_getLogs", []interface{}{map[string]interface{}{"fromBlock": hexBlock(from), "toBlock": hexBlock(to), "address": Token, "topics": []interface{}{TransferTopic, nil, addressTopic(recipient)}}}, &logs)
	return logs, err
}
func (r *RPC) Receipt(ctx context.Context, hash string) (*chain.Receipt, error) {
	var receipt chain.Receipt
	if err := r.call(ctx, "eth_getTransactionReceipt", []interface{}{hash}, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}
