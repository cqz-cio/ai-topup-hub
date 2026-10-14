package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func (s *Service) execute(ctx context.Context, job *reconciliationdomain.Job) error {
	upstreamOrders, err := s.upstream.Open(job.ConnectionID)
	if err != nil {
		return fmt.Errorf("open upstream orders: %w", err)
	}
	orders, err := s.procurements.ListByConnectionAndTimeRange(job.ConnectionID, job.TimeRangeStart, job.TimeRangeEnd)
	if err != nil {
		return fmt.Errorf("list procurement orders: %w", err)
	}

	mismatches := make([]reconciliationdomain.Item, 0)
	skippedCount, errorCount, uncheckedAmountCount, matchedCount := 0, 0, 0, 0
	checkAmount := job.Type == constants.ReconciliationTypeAmount || job.Type == constants.ReconciliationTypeFull
	getOrder := upstreamOrders.Get
	if reader, ok := upstreamOrders.(reconciliationcontract.UpstreamOrderAmountReader); ok && checkAmount {
		getOrder = reader.GetWithAmount
	}
	for index := range orders {
		order := &orders[index]
		if order.UpstreamOrderID == 0 && strings.TrimSpace(order.UpstreamOrderNo) == "" {
			skippedCount++
			continue
		}
		detail, err := getOrder(ctx, order.UpstreamOrderID, order.UpstreamOrderNo)
		if err != nil || detail == nil || strings.TrimSpace(detail.Status) == "" {
			logger.Warnw("reconciliation_get_upstream_order_failed", "job_id", job.ID,
				"procurement_id", order.ID, "upstream_order_id", order.UpstreamOrderID, "error", err)
			errorCount++
			continue
		}
		amount, amountErr := decimal.NewFromString(strings.TrimSpace(detail.Amount))
		uncheckedAmount := checkAmount && (amountErr != nil || amount.IsNegative() || !sameCurrency(order, detail))
		if uncheckedAmount {
			uncheckedAmountCount++
		}
		if item := compareOrder(job, order, detail); item != nil {
			mismatches = append(mismatches, *item)
		} else if !uncheckedAmount {
			matchedCount++
		}
	}
	if len(mismatches) > 0 {
		if err := s.items.BatchCreate(mismatches); err != nil {
			return fmt.Errorf("batch create reconciliation items: %w", err)
		}
	}

	job.TotalCount = len(orders)
	job.MismatchedCount = len(mismatches)
	job.MatchedCount = matchedCount
	job.ResultJSON = marshalResult(map[string]any{
		"total": job.TotalCount, "matched": job.MatchedCount, "mismatched": job.MismatchedCount,
		"skipped": skippedCount, "errors": errorCount, "unchecked_amount": uncheckedAmountCount,
	})
	if skippedCount+errorCount+uncheckedAmountCount > 0 {
		return reconciliationcontract.ErrIncompleteResults
	}
	return nil
}

func compareOrder(job *reconciliationdomain.Job, order *reconciliationcontract.ProcurementOrder, detail *reconciliationcontract.UpstreamOrder) *reconciliationdomain.Item {
	checkStatus := job.Type == constants.ReconciliationTypeStatus || job.Type == constants.ReconciliationTypeFull
	checkAmount := job.Type == constants.ReconciliationTypeAmount || job.Type == constants.ReconciliationTypeFull
	statusMismatch := checkStatus && !reconciliationdomain.IsStatusConsistent(order.Status, detail.Status)

	amountMismatch := false
	var upstreamAmount money.Amount
	if checkAmount && detail.Amount != "" && sameCurrency(order, detail) {
		value, err := decimal.NewFromString(strings.TrimSpace(detail.Amount))
		if err == nil && !value.IsNegative() {
			upstreamAmount = money.FromDecimal(value)
			amountMismatch = !order.UpstreamAmount.Equal(value)
		}
	}

	mismatchType := ""
	switch {
	case statusMismatch && amountMismatch:
		mismatchType = constants.MismatchTypeBoth
	case statusMismatch:
		mismatchType = constants.MismatchTypeStatus
	case amountMismatch:
		mismatchType = constants.MismatchTypeAmount
	}
	if mismatchType == "" {
		return nil
	}
	return &reconciliationdomain.Item{
		JobID: job.ID, ProcurementOrderID: order.ID,
		LocalOrderNo: order.LocalOrderNo, UpstreamOrderNo: order.UpstreamOrderNo,
		LocalStatus: order.Status, UpstreamStatus: detail.Status,
		LocalAmount: order.UpstreamAmount, UpstreamAmount: upstreamAmount, MismatchType: mismatchType,
	}
}

func sameCurrency(order *reconciliationcontract.ProcurementOrder, detail *reconciliationcontract.UpstreamOrder) bool {
	// Older native-protocol records may not have stored a currency.
	return strings.TrimSpace(order.Currency) == "" || strings.EqualFold(strings.TrimSpace(order.Currency), strings.TrimSpace(detail.Currency))
}

func marshalResult(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
