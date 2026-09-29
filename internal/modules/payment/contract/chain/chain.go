package chain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"strconv"
	"strings"
	"time"
)

const Token = "0x55d398326f99059ff775485246999027b3197955"
const TransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
const Provider = "bscusdt"
const ChainID = 56

var ErrRPC = errors.New("bsc_rpc_unavailable_or_invalid")
var ErrNotFound = errors.New("chain_record_not_found")

type Header struct {
	Number    string `json:"number"`
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
}
type Log struct {
	Address         string   `json:"address"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	BlockNumber     string   `json:"blockNumber"`
	BlockHash       string   `json:"blockHash"`
	TransactionHash string   `json:"transactionHash"`
	LogIndex        string   `json:"logIndex"`
	Removed         bool     `json:"removed"`
}
type Receipt struct {
	Status          string `json:"status"`
	TransactionHash string `json:"transactionHash"`
	BlockHash       string `json:"blockHash"`
	BlockNumber     string `json:"blockNumber"`
	Logs            []Log  `json:"logs"`
}
type Reader interface {
	Check(context.Context) error
	Header(context.Context, string) (*Header, error)
	Logs(context.Context, uint64, uint64, string) ([]Log, error)
	Receipt(context.Context, string) (*Receipt, error)
}
type Store interface {
	Invoice(context.Context, uint) (*domain.ChainInvoice, error)
	PayableUntil(context.Context, uint) (*time.Time, error)
	CreateInvoice(context.Context, *domain.ChainInvoice, *domain.ChainCursor) (bool, error)
	Cursor(context.Context, string) (*domain.ChainCursor, error)
	ApplyScan(context.Context, domain.ChainCursor, uint64, string, []domain.ChainTransfer) error
	ReadyTransfers(context.Context, string) ([]domain.ChainTransfer, error)
	InvoiceForTransfer(context.Context, domain.ChainTransfer) (*domain.ChainInvoice, error)
	RetryTransfer(context.Context, string) error
	DeliverTransfer(context.Context, string) error
	Counts(context.Context, string) (int64, int64, error)
	Transfers(context.Context, string, string, uint64) ([]domain.ChainTransfer, error)
}

func Quantity(v string) (uint64, error) {
	if !strings.HasPrefix(v, "0x") || len(v) < 3 || len(v) > 18 {
		return 0, ErrRPC
	}
	n, err := strconv.ParseUint(v[2:], 16, 64)
	if err != nil {
		return 0, ErrRPC
	}
	return n, nil
}
func HexBlock(n uint64) string { return fmt.Sprintf("0x%x", n) }
func IsHex(v string, n int) bool {
	if len(v) != 2+n*2 || !strings.HasPrefix(v, "0x") {
		return false
	}
	_, err := hex.DecodeString(v[2:])
	return err == nil
}
func AddressTopic(v string) string { return "0x" + strings.Repeat("0", 24) + strings.ToLower(v[2:]) }
