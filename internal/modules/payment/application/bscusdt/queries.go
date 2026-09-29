package bscusdt

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
)

// Status is read-only; RPC credentials and connection URLs are never returned.
func (s *Service) Status(ctx context.Context) (map[string]interface{}, error) {
	cursor, err := s.store.Cursor(ctx, s.stream)
	if err != nil && !errors.Is(err, chain.ErrNotFound) {
		return nil, err
	}
	if cursor == nil {
		cursor = &domain.ChainCursor{}
	}
	ready, review, err := s.store.Counts(ctx, s.stream)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"chain_id": ChainID, "token_contract": Token, "recipient": s.config.Recipient, "cny_per_usdt": s.rate.String(), "confirmations": s.config.Confirmations, "next_block": cursor.NextBlock, "last_scan_at": cursor.UpdatedAt, "pending_settlements": ready, "manual_review_count": review}, nil
}
func (s *Service) Transfers(ctx context.Context, state string, before uint64) ([]domain.ChainTransfer, error) {
	return s.store.Transfers(ctx, s.stream, state, before)
}
