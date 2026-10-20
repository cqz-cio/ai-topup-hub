// Package tronusdt reads TRON mainnet without keys or transaction broadcasts.
// Complete solidified block receipts avoid skipping deposits when an account
// history index is delayed.
package tronusdt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
)

const MainnetGenesis = "00000000000000001ebf88508a03865c71d452e25f4d51194196a1d22b6653dc"
const DefaultEndpoint = "https://tron-rpc.publicnode.com"
const tokenHex = "a614f803b6fd780986a42c78ec9c7f77e6ded13c"

type RPC struct {
	endpoint, apiKey string
	client           *http.Client
	checkMu          sync.Mutex
	checkedAt        time.Time
	requestInterval  time.Duration
	requestGate      chan struct{}
	nextRequest      time.Time
}
type block struct {
	ID     string `json:"blockID"`
	Header struct {
		Raw struct {
			Number     uint64 `json:"number"`
			Timestamp  int64  `json:"timestamp"`
			ParentHash string `json:"parentHash"`
		} `json:"raw_data"`
	} `json:"block_header"`
	Transactions []struct {
		ID string `json:"txID"`
	} `json:"transactions"`
}
type transactionInfo struct {
	ID          string `json:"id"`
	BlockNumber uint64 `json:"blockNumber"`
	Timestamp   int64  `json:"blockTimeStamp"`
	Receipt     struct {
		Result string `json:"result"`
	} `json:"receipt"`
	Result string `json:"result"`
	Logs   []struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	} `json:"log"`
}

func NewRPC(endpoint, apiKey string) (*RPC, error) {
	return NewRPCWithInterval(endpoint, apiKey, 0)
}

// NewRPCWithInterval optionally spaces HTTP requests for a rate-limited backup.
// Waiting is cancellable and never extends the caller's scan deadline.
func NewRPCWithInterval(endpoint, apiKey string, interval time.Duration) (*RPC, error) {
	if interval < 0 || interval > 5*time.Second {
		return nil, chain.ErrRPC
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return nil, chain.ErrRPC
	}
	return &RPC{requestInterval: interval, requestGate: make(chan struct{}, 1), endpoint: strings.TrimRight(endpoint, "/"), apiKey: apiKey, client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (r *RPC) waitRequest(ctx context.Context) error {
	if r.requestInterval == 0 {
		return ctx.Err()
	}
	select {
	case r.requestGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-r.requestGate }()
	if wait := time.Until(r.nextRequest); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.nextRequest = time.Now().Add(r.requestInterval)
	return nil
}

func (r *RPC) call(ctx context.Context, path string, input, output any) error {
	if err := r.waitRequest(ctx); err != nil {
		return err
	}
	data, err := json.Marshal(input)
	if err != nil {
		return chain.ErrRPC
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+path, bytes.NewReader(data))
	if err != nil {
		return chain.ErrRPC
	}
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", r.apiKey)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return chain.ErrRPC
	} // Never expose credential-bearing URLs.
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return chain.ErrRPC
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (16<<20)+1))
	if err != nil || len(body) > 16<<20 || len(body) == 0 || string(body) == "null" {
		return chain.ErrRPC
	}
	var apiError struct {
		Error json.RawMessage `json:"Error"`
	}
	if json.Unmarshal(body, &apiError) == nil && len(apiError.Error) > 0 {
		return chain.ErrRPC
	}
	if json.Unmarshal(body, output) != nil {
		return chain.ErrRPC
	}
	return nil
}

// Cache identity checks to bound idle traffic. Settlement uses fresh blocks.
func (r *RPC) Check(ctx context.Context) error {
	r.checkMu.Lock()
	defer r.checkMu.Unlock()
	if time.Since(r.checkedAt) < 5*time.Minute {
		return nil
	}
	var genesis block
	if err := r.call(ctx, "/walletsolidity/getblockbynum", map[string]any{"num": 0}, &genesis); err != nil {
		return err
	}
	if genesis.ID != MainnetGenesis || genesis.Header.Raw.Number != 0 {
		return chain.ErrRPC
	}
	var decimals struct {
		Result struct {
			OK bool `json:"result"`
		} `json:"result"`
		Values []string `json:"constant_result"`
	}
	if err := r.call(ctx, "/wallet/triggerconstantcontract", map[string]any{"owner_address": chain.TronUSDTContract, "contract_address": chain.TronUSDTContract, "function_selector": "decimals()", "visible": true}, &decimals); err != nil {
		return err
	}
	if !decimals.Result.OK || len(decimals.Values) != 1 || decimals.Values[0] != fmt.Sprintf("%064x", 6) {
		return chain.ErrRPC
	}
	r.checkedAt = time.Now()
	return nil
}
func validateBlock(b *block) error {
	n := b.Header.Raw.Number
	if !chain.IsHex("0x"+b.ID, 32) || !strings.HasPrefix(b.ID, fmt.Sprintf("%016x", n)) || (n > 0 && (b.Header.Raw.Timestamp <= 0 || !chain.IsHex("0x"+b.Header.Raw.ParentHash, 32))) {
		return chain.ErrRPC
	}
	seen := map[string]bool{}
	for _, tx := range b.Transactions {
		if !chain.IsHex("0x"+tx.ID, 32) || seen[tx.ID] {
			return chain.ErrRPC
		}
		seen[tx.ID] = true
	}
	return nil
}
func normalizedHeader(b *block) *chain.Header {
	return &chain.Header{Number: chain.HexBlock(b.Header.Raw.Number), Hash: "0x" + b.ID, Timestamp: chain.HexBlock(uint64(b.Header.Raw.Timestamp / 1000))}
}
func (r *RPC) readBlock(ctx context.Context, tag string) (*block, error) {
	path := "/walletsolidity/getblockbynum"
	input := map[string]any{}
	var requested uint64
	var err error
	switch tag {
	case "latest":
		path = "/wallet/getnowblock"
	case "finalized":
		path = "/walletsolidity/getnowblock"
	default:
		requested, err = chain.Quantity(tag)
		if err != nil {
			return nil, err
		}
		input["num"] = requested
	}
	var b block
	if err = r.call(ctx, path, input, &b); err != nil {
		return nil, err
	}
	if err = validateBlock(&b); err != nil {
		return nil, err
	}
	if strings.HasPrefix(tag, "0x") && b.Header.Raw.Number != requested {
		return nil, chain.ErrRPC
	}
	return &b, nil
}
func (r *RPC) Header(ctx context.Context, tag string) (*chain.Header, error) {
	b, err := r.readBlock(ctx, tag)
	if err != nil {
		return nil, err
	}
	return normalizedHeader(b), nil
}
func normalizeReceipt(info transactionInfo, b *block) (*chain.Receipt, error) {
	if !chain.IsHex("0x"+info.ID, 32) || info.BlockNumber != b.Header.Raw.Number || info.Timestamp != b.Header.Raw.Timestamp {
		return nil, chain.ErrRPC
	}
	found := false
	for _, tx := range b.Transactions {
		if tx.ID == info.ID {
			found = true
			break
		}
	}
	if !found {
		return nil, chain.ErrRPC
	}
	h := normalizedHeader(b)
	out := &chain.Receipt{TransactionHash: "0x" + info.ID, BlockNumber: h.Number, BlockHash: h.Hash, Status: "0x0"}
	if info.Receipt.Result != "SUCCESS" || (info.Result != "" && info.Result != "SUCCESS" && info.Result != "SUCESS") {
		return out, nil
	}
	out.Status = "0x1"
	for i, event := range info.Logs {
		if !strings.EqualFold(event.Address, tokenHex) || len(event.Topics) == 0 || !strings.EqualFold("0x"+event.Topics[0], chain.TransferTopic) {
			continue
		}
		if len(event.Topics) != 3 || !chain.IsHex("0x"+event.Data, 32) {
			return nil, chain.ErrRPC
		}
		topics := make([]string, 3)
		for j, topic := range event.Topics {
			if !chain.IsHex("0x"+topic, 32) || (j > 0 && !strings.HasPrefix(topic, strings.Repeat("0", 24))) {
				return nil, chain.ErrRPC
			}
			topics[j] = "0x" + strings.ToLower(topic)
		}
		out.Logs = append(out.Logs, chain.Log{Address: "0x" + tokenHex, Topics: topics, Data: "0x" + strings.ToLower(event.Data), TransactionHash: out.TransactionHash, BlockNumber: h.Number, BlockHash: h.Hash, LogIndex: chain.HexBlock(uint64(i))})
	}
	return out, nil
}
func (r *RPC) Logs(ctx context.Context, from, to uint64, recipient string) ([]chain.Log, error) {
	topic := chain.AddressTopic(recipient)
	if from > to || to-from >= 10 || topic == "" {
		return nil, chain.ErrRPC
	}
	var logs []chain.Log
	var previousHash string
	for number := from; number <= to; number++ {
		// Public nodes need not expose the optional batch-block endpoint.
		b, err := r.readBlock(ctx, chain.HexBlock(number))
		if err != nil {
			return nil, err
		}
		if previousHash != "" && b.Header.Raw.ParentHash != previousHash {
			return nil, chain.ErrRPC
		}
		previousHash = b.ID
		var infos []transactionInfo
		if err := r.call(ctx, "/walletsolidity/gettransactioninfobyblocknum", map[string]any{"num": b.Header.Raw.Number}, &infos); err != nil {
			return nil, err
		}
		// Missing or partial receipts must not advance the persisted cursor.
		if len(infos) != len(b.Transactions) {
			return nil, chain.ErrRPC
		}
		seen := map[string]bool{}
		for _, info := range infos {
			if seen[info.ID] {
				return nil, chain.ErrRPC
			}
			seen[info.ID] = true
			receipt, err := normalizeReceipt(info, b)
			if err != nil {
				return nil, err
			}
			for _, event := range receipt.Logs {
				if event.Topics[2] == topic {
					logs = append(logs, event)
				}
			}
		}
	}
	return logs, nil
}
func (r *RPC) Receipt(ctx context.Context, hash string) (*chain.Receipt, error) {
	if !chain.IsHex(hash, 32) {
		return nil, chain.ErrRPC
	}
	var info transactionInfo
	if err := r.call(ctx, "/walletsolidity/gettransactioninfobyid", map[string]any{"value": hash[2:]}, &info); err != nil {
		return nil, err
	}
	if info.ID != hash[2:] {
		return nil, chain.ErrRPC
	}
	b, err := r.readBlock(ctx, chain.HexBlock(info.BlockNumber))
	if err != nil {
		return nil, err
	}
	return normalizeReceipt(info, b)
}
