package bscusdt

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
)

var ErrReorg = errors.New("bsc_finalized_chain_changed_manual_review_required")

func (s *Service) RunOnce(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := s.chain.Check(ctx); err != nil {
		return err
	}
	// Settlement outbox is retried even when no new blocks arrive.
	if err := s.deliver(ctx); err != nil {
		return err
	}
	cursor, err := s.store.Cursor(ctx, s.stream)
	if err != nil {
		if errors.Is(err, chain.ErrNotFound) {
			return nil
		}
		return err
	}
	finalized, err := s.chain.Header(ctx, "finalized")
	if err != nil {
		return err
	}
	finalHeight, err := quantity(finalized.Number)
	if err != nil {
		return err
	}
	latest, err := s.chain.Header(ctx, "latest")
	if err != nil {
		return err
	}
	head, err := quantity(latest.Number)
	if err != nil || head < s.config.Confirmations || finalHeight > head {
		return ErrRPC
	}
	end := head - s.config.Confirmations + 1
	if finalHeight < end {
		end = finalHeight
	}
	if end < cursor.NextBlock {
		return nil
	}
	if cursor.PreviousHash != "" && cursor.NextBlock > 0 {
		previous, e := s.chain.Header(ctx, hexBlock(cursor.NextBlock-1))
		if e != nil {
			return e
		}
		if !strings.EqualFold(previous.Hash, cursor.PreviousHash) {
			return ErrReorg
		}
	}
	// Bound each request to 100 blocks; progress survives restarts and RPC errors.
	if end-cursor.NextBlock >= 100 {
		end = cursor.NextBlock + 99
	}
	boundary, err := s.chain.Header(ctx, hexBlock(end))
	if err != nil {
		return err
	}
	logs, err := s.chain.Logs(ctx, cursor.NextBlock, end, s.config.Recipient)
	if err != nil {
		return err
	}
	var transfers []domain.ChainTransfer
	for _, event := range logs {
		transfer, e := s.verify(ctx, event, cursor.NextBlock, end)
		if e != nil {
			return e
		} // Never advance past an incomplete/malformed RPC page.
		transfers = append(transfers, *transfer)
	}
	sort.Slice(transfers, func(i, j int) bool {
		if transfers[i].Block == transfers[j].Block {
			return transfers[i].LogIndex < transfers[j].LogIndex
		}
		return transfers[i].Block < transfers[j].Block
	})
	boundaryAfter, err := s.chain.Header(ctx, hexBlock(end))
	if err != nil {
		return err
	}
	if !strings.EqualFold(boundaryAfter.Hash, boundary.Hash) {
		return ErrReorg
	}
	err = s.store.ApplyScan(ctx, *cursor, end, boundary.Hash, transfers)
	if err != nil {
		return err
	}
	return s.deliver(ctx)
}

func (s *Service) verify(ctx context.Context, l chain.Log, from, to uint64) (*domain.ChainTransfer, error) {
	if l.Removed || !strings.EqualFold(l.Address, Token) || len(l.Topics) != 3 || !strings.EqualFold(l.Topics[0], TransferTopic) || !strings.EqualFold(l.Topics[2], addressTopic(s.config.Recipient)) || !isHex(l.Topics[1], 32) || !strings.HasPrefix(strings.ToLower(l.Topics[1]), "0x"+strings.Repeat("0", 24)) || !isHex(l.Data, 32) || !isHex(l.TransactionHash, 32) || !isHex(l.BlockHash, 32) {
		return nil, ErrRPC
	}
	n, e := quantity(l.BlockNumber)
	if e != nil || n < from || n > to {
		return nil, ErrRPC
	}
	idx, e := quantity(l.LogIndex)
	if e != nil {
		return nil, ErrRPC
	}
	units, ok := new(big.Int).SetString(l.Data[2:], 16)
	if !ok {
		return nil, ErrRPC
	}
	h, e := s.chain.Header(ctx, hexBlock(n))
	if e != nil {
		return nil, e
	}
	if !strings.EqualFold(h.Hash, l.BlockHash) {
		return nil, ErrReorg
	}
	stamp, e := quantity(h.Timestamp)
	if e != nil || stamp > uint64(time.Now().Add(30*time.Second).Unix()) {
		return nil, ErrRPC
	}
	r, e := s.chain.Receipt(ctx, l.TransactionHash)
	if e != nil {
		return nil, e
	}
	if r == nil || r.Status != "0x1" || !strings.EqualFold(r.TransactionHash, l.TransactionHash) || !strings.EqualFold(r.BlockHash, l.BlockHash) || r.BlockNumber != l.BlockNumber {
		return nil, ErrRPC
	}
	found := false
	for _, candidate := range r.Logs {
		if candidate.LogIndex == l.LogIndex && candidate.TransactionHash == l.TransactionHash && candidate.BlockHash == l.BlockHash && candidate.BlockNumber == l.BlockNumber && !candidate.Removed && strings.EqualFold(candidate.Address, l.Address) && candidate.Data == l.Data && len(candidate.Topics) == 3 && strings.EqualFold(strings.Join(candidate.Topics, "/"), strings.Join(l.Topics, "/")) {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrRPC
	}
	return &domain.ChainTransfer{Key: fmt.Sprintf("%d/%s/%d", ChainID, strings.ToLower(l.TransactionHash), idx), Stream: s.stream, TxHash: strings.ToLower(l.TransactionHash), LogIndex: idx, Block: n, BlockHash: strings.ToLower(l.BlockHash), Sender: "0x" + strings.ToLower(l.Topics[1][26:]), Units: units.String(), PaidAt: time.Unix(int64(stamp), 0).UTC(), State: "manual_review", Reason: "unmatched_amount"}, nil
}

func (s *Service) deliver(ctx context.Context) error {
	if s.settle == nil {
		return nil
	}
	pending, err := s.store.ReadyTransfers(ctx, s.stream)
	if err != nil {
		return err
	}
	var firstError error
	for _, v := range pending {
		invoice, err := s.store.InvoiceForTransfer(ctx, v)
		if err != nil {
			return err
		}
		// Revalidate inclusion on retry; never settle from a stale cached receipt.
		h, err := s.chain.Header(ctx, hexBlock(v.Block))
		if err != nil {
			return err
		}
		if !strings.EqualFold(h.Hash, v.BlockHash) {
			return ErrReorg
		}
		if err = s.settle(ctx, *invoice, v); err != nil {
			if firstError == nil {
				firstError = err
			}
			if updateErr := s.store.RetryTransfer(ctx, v.Key); updateErr != nil {
				return updateErr
			}
			continue
		}
		if err = s.store.DeliverTransfer(ctx, v.Key); err != nil {
			return err
		}
	}
	return firstError
}
