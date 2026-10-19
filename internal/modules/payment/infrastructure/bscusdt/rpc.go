package bscusdt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	endpoint        string
	logsEndpoint    string
	receiptEndpoint string
	client          *http.Client
	network         chain.Network
}

func NewRPC(endpoint string) (*RPC, error) {
	return NewRPCWithReceiptEndpoint(endpoint, "")
}

// NewRPCWithReceiptEndpoint supports nodes whose public log and receipt APIs
// have different access limits. Settlement still compares each receipt with
// the finalized block and transfer log returned by the primary endpoint.
func NewRPCWithReceiptEndpoint(endpoint, receiptEndpoint string) (*RPC, error) {
	return NewRPCForNetwork(endpoint, receiptEndpoint, "bsc")
}

func NewRPCForNetwork(endpoint, receiptEndpoint, networkName string) (*RPC, error) {
	return NewRPCWithEndpoints(endpoint, "", receiptEndpoint, networkName)
}

// NewRPCWithEndpoints supports a separate event provider when the header and
// receipt node does not serve historical logs. Every provider is validated
// against the same chain and token; the scanner still verifies each event.
func NewRPCWithEndpoints(endpoint, logsEndpoint, receiptEndpoint, networkName string) (*RPC, error) {
	network, err := chain.ResolveNetwork(networkName)
	if err != nil {
		return nil, err
	}
	if !validEndpoint(endpoint) || (logsEndpoint != "" && !validEndpoint(logsEndpoint)) || (receiptEndpoint != "" && !validEndpoint(receiptEndpoint)) {
		return nil, ErrRPC
	}
	return &RPC{endpoint: endpoint, logsEndpoint: logsEndpoint, receiptEndpoint: receiptEndpoint, network: network, client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func validEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}
func (r *RPC) call(ctx context.Context, method string, params interface{}, out interface{}) error {
	endpoint := r.endpoint
	if method == "eth_getLogs" && r.logsEndpoint != "" {
		endpoint = r.logsEndpoint
	}
	if method == "eth_getTransactionReceipt" && r.receiptEndpoint != "" {
		endpoint = r.receiptEndpoint
	}
	return r.callEndpoint(ctx, endpoint, method, params, out)
}

func (r *RPC) callEndpoint(ctx context.Context, endpoint, method string, params interface{}, out interface{}) error {
	raw, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return ErrRPC
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return ErrRPC
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PiStarPaymentMonitor/1.0")
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
	seen := make(map[string]bool)
	for _, endpoint := range []string{r.endpoint, r.logsEndpoint, r.receiptEndpoint} {
		if endpoint == "" || seen[endpoint] {
			continue
		}
		seen[endpoint] = true
		if err := r.checkEndpoint(ctx, endpoint); err != nil {
			return err
		}
	}
	return nil
}

func (r *RPC) checkEndpoint(ctx context.Context, endpoint string) error {
	var chain, decimals string
	if err := r.callEndpoint(ctx, endpoint, "eth_chainId", []interface{}{}, &chain); err != nil {
		return err
	}
	n, err := quantity(chain)
	if err != nil || n != r.network.ChainID {
		return ErrRPC
	}
	if err = r.callEndpoint(ctx, endpoint, "eth_call", []interface{}{map[string]string{"to": r.network.Token, "data": "0x313ce567"}, "latest"}, &decimals); err != nil {
		return err
	}
	if !isHex(decimals, 32) || decimals != fmt.Sprintf("0x%064x", r.network.Decimals) {
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
	if to < from {
		return nil, ErrRPC
	}
	var logs []chain.Log
	for start := from; ; {
		end := to
		// Public event providers may accept only 50 blocks per request.
		if to-start >= 50 {
			end = start + 49
		}
		var page []chain.Log
		if err := r.call(ctx, "eth_getLogs", []interface{}{map[string]interface{}{"fromBlock": hexBlock(start), "toBlock": hexBlock(end), "address": r.network.Token, "topics": []interface{}{TransferTopic, nil, addressTopic(recipient)}}}, &page); err != nil {
			return nil, err // Never expose a partial scan as a successful page.
		}
		for _, event := range page {
			block, err := quantity(event.BlockNumber)
			if err != nil || block < start || block > end {
				return nil, ErrRPC
			}
		}
		logs = append(logs, page...)
		if end == to {
			return logs, nil
		}
		start = end + 1
	}
}
func (r *RPC) Receipt(ctx context.Context, hash string) (*chain.Receipt, error) {
	var receipt chain.Receipt
	if err := r.call(ctx, "eth_getTransactionReceipt", []interface{}{hash}, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}
