package refund

import (
	"strings"
	"time"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

type paymentFeeReader interface {
	ListByOrderID(orderID uint) ([]paymentdomain.Payment, error)
}

type paymentFeeRefundSnapshot struct {
	rootOrderID   uint
	paymentAmount decimal.Decimal
	paymentFee    decimal.Decimal
}

// UpdatePaymentFeeRefundedInput updates the accounting fact attached to an
// existing manual refund record. It does not call a payment gateway.
type UpdatePaymentFeeRefundedInput struct {
	RefundRecordID     uint
	PaymentFeeRefunded bool
}

func (s *Service) UpdatePaymentFeeRefunded(input UpdatePaymentFeeRefundedInput) (*orderdomain.OrderRefundRecord, error) {
	if input.RefundRecordID == 0 {
		return nil, ErrOrderNotFound
	}
	if s == nil || s.orderStore == nil {
		return nil, ErrOrderFetchFailed
	}

	initial, err := s.orderStore.GetRefundRecordByID(input.RefundRecordID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if initial == nil {
		return nil, ErrOrderNotFound
	}
	if initial.Type != constants.OrderRefundTypeManual {
		return nil, ErrOrderStatusInvalid
	}
	initialOrder, err := s.orderStore.GetByID(initial.OrderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if initialOrder == nil {
		return nil, ErrOrderNotFound
	}

	feeSnapshot := paymentFeeRefundSnapshot{rootOrderID: paymentFeeRefundRootOrderID(initialOrder)}
	if input.PaymentFeeRefunded {
		feeSnapshot, err = s.loadPaymentFeeRefundSnapshot(initialOrder)
		if err != nil {
			return nil, err
		}
	}

	var updated *orderdomain.OrderRefundRecord
	err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		orders := tx.Orders()
		order, err := orders.GetByIDForUpdate(initial.OrderID)
		if err != nil {
			return ErrOrderFetchFailed
		}
		if order == nil {
			return ErrOrderNotFound
		}
		record, err := orders.GetRefundRecordByIDForUpdate(input.RefundRecordID)
		if err != nil {
			return ErrOrderFetchFailed
		}
		if record == nil || record.OrderID != order.ID {
			return ErrOrderNotFound
		}
		if record.Type != constants.OrderRefundTypeManual {
			return ErrOrderStatusInvalid
		}

		feeAmount := money.FromDecimal(decimal.Zero)
		if input.PaymentFeeRefunded {
			if paymentFeeRefundRootOrderID(order) != feeSnapshot.rootOrderID {
				return ErrOrderFetchFailed
			}
			feeAmount, err = resolvePaymentFeeRefundAmount(orders, feeSnapshot, record.Amount.Decimal, record.ID)
			if err != nil {
				return err
			}
		}
		now := time.Now()
		if err := orders.UpdateRefundRecordPaymentFee(record.ID, input.PaymentFeeRefunded, feeAmount, now); err != nil {
			return ErrOrderUpdateFailed
		}
		record.PaymentFeeRefunded = input.PaymentFeeRefunded
		record.PaymentFeeRefundedAmount = feeAmount
		record.UpdatedAt = now
		updated = record
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) loadPaymentFeeRefundSnapshot(order *orderdomain.Order) (paymentFeeRefundSnapshot, error) {
	if s == nil || s.payments == nil || order == nil {
		return paymentFeeRefundSnapshot{}, ErrOrderUpdateFailed
	}
	rootOrderID := paymentFeeRefundRootOrderID(order)
	if rootOrderID == 0 {
		return paymentFeeRefundSnapshot{}, ErrOrderNotFound
	}
	payments, err := s.payments.ListByOrderID(rootOrderID)
	if err != nil {
		return paymentFeeRefundSnapshot{}, ErrOrderFetchFailed
	}
	paymentAmount, paymentFee := refundablePaymentFeeSnapshot(payments, order.Currency)
	return paymentFeeRefundSnapshot{
		rootOrderID:   rootOrderID,
		paymentAmount: paymentAmount,
		paymentFee:    paymentFee,
	}, nil
}

func resolvePaymentFeeRefundAmount(
	orders ordercontract.Store,
	snapshot paymentFeeRefundSnapshot,
	refundAmount decimal.Decimal,
	excludeRefundRecordID uint,
) (money.Amount, error) {
	zero := money.FromDecimal(decimal.Zero)
	if orders == nil || snapshot.rootOrderID == 0 {
		return zero, ErrOrderUpdateFailed
	}
	if snapshot.paymentAmount.LessThanOrEqual(decimal.Zero) || snapshot.paymentFee.LessThanOrEqual(decimal.Zero) {
		return zero, nil
	}

	orderIDs := []uint{snapshot.rootOrderID}
	children, err := orders.ListChildren(snapshot.rootOrderID)
	if err != nil {
		return zero, ErrOrderFetchFailed
	}
	for _, child := range children {
		orderIDs = append(orderIDs, child.ID)
	}
	records, err := orders.ListRefundRecordsByOrderIDs(orderIDs)
	if err != nil {
		return zero, ErrOrderFetchFailed
	}
	refundedPrincipalBefore := decimal.Zero
	refundedFeeBefore := decimal.Zero
	for _, record := range records {
		if record.ID == excludeRefundRecordID || !record.PaymentFeeRefunded {
			continue
		}
		refundedPrincipalBefore = refundedPrincipalBefore.Add(record.Amount.Decimal)
		refundedFeeBefore = refundedFeeBefore.Add(record.PaymentFeeRefundedAmount.Decimal)
	}

	amount := orderdomain.CalculatePaymentFeeRefundAmount(
		snapshot.paymentAmount,
		snapshot.paymentFee,
		refundedPrincipalBefore,
		refundedFeeBefore,
		refundAmount,
	)
	return money.FromDecimal(amount), nil
}

func paymentFeeRefundRootOrderID(order *orderdomain.Order) uint {
	if order == nil {
		return 0
	}
	if order.ParentID != nil && *order.ParentID > 0 {
		return *order.ParentID
	}
	return order.ID
}

func refundablePaymentFeeSnapshot(payments []paymentdomain.Payment, currency string) (decimal.Decimal, decimal.Decimal) {
	paymentAmount := decimal.Zero
	paymentFee := decimal.Zero
	for _, payment := range payments {
		if payment.Status != constants.PaymentStatusSuccess ||
			payment.ProviderType == constants.PaymentProviderWallet ||
			payment.FeePolicy != constants.PaymentFeePolicyMerchantAbsorbed ||
			strings.TrimSpace(payment.ExceptionCode) != "" {
			continue
		}
		if currency != "" && payment.Currency != "" && !strings.EqualFold(currency, payment.Currency) {
			continue
		}
		amount := payment.Amount.Decimal.Round(2)
		fee := payment.FeeAmount.Decimal.Round(2)
		if amount.LessThanOrEqual(decimal.Zero) || fee.LessThanOrEqual(decimal.Zero) {
			continue
		}
		paymentAmount = paymentAmount.Add(amount)
		paymentFee = paymentFee.Add(fee)
	}
	return paymentAmount.Round(2), paymentFee.Round(2)
}
