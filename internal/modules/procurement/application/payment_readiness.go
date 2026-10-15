package application

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	"github.com/shopspring/decimal"
)

func (s *Service) SetBalanceNotifier(notifier procurementcontract.BalanceNotifier) {
	s.balanceNotifier = notifier
}

type fundingNeed struct {
	connectionID uint
	amount       decimal.Decimal
	currency     string
}

func unavailableBalance() error {
	return &procurementcontract.PaymentBlock{Code: procurementcontract.BalanceUnavailable, NotificationStatus: "not_sent"}
}

// fundingNeeds uses the same immutable cost ceiling as SubmitToUpstream. A
// parent contains copies of its children's items: count leaves exactly once.
func (s *Service) fundingNeeds(order *procurementdomain.LocalOrder) ([]fundingNeed, error) {
	needs := map[uint]fundingNeed{}
	var walk func(*procurementdomain.LocalOrder) error
	walk = func(order *procurementdomain.LocalOrder) error {
		if len(order.Children) > 0 {
			for i := range order.Children {
				if err := walk(&order.Children[i]); err != nil {
					return err
				}
			}
			return nil
		}
		for _, item := range order.Items {
			if strings.TrimSpace(item.FulfillmentType) != constants.FulfillmentTypeUpstream {
				continue
			}
			if s.mappingRepo == nil || item.Quantity <= 0 || !item.CostPrice.Decimal.IsPositive() {
				return unavailableBalance()
			}
			id, found, err := s.mappingRepo.FindConnectionID(item.ProductID)
			if err != nil || !found || id == 0 {
				return unavailableBalance()
			}
			currency := strings.ToUpper(strings.TrimSpace(order.Currency))
			need, exists := needs[id]
			if currency == "" || (exists && need.currency != currency) {
				return unavailableBalance()
			}
			need.connectionID, need.currency = id, currency
			need.amount = need.amount.Add(item.CostPrice.Decimal.Mul(decimal.NewFromInt(int64(item.Quantity))))
			needs[id] = need
		}
		return nil
	}
	if err := walk(order); err != nil {
		return nil, err
	}
	result := make([]fundingNeed, 0, len(needs))
	for _, need := range needs {
		result = append(result, need)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].connectionID < result[j].connectionID })
	return result, nil
}

// CheckPaymentBalance is read-only at the supplier. It must run before wallet
// debits, invoice creation AND reuse; receipts for money already paid bypass it.
func (s *Service) CheckPaymentBalance(ctx context.Context, orderID uint) error {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil || order == nil {
		return unavailableBalance()
	}
	needs, err := s.fundingNeeds(order)
	if err != nil {
		return err
	}
	if len(needs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, need := range needs {
		if s.connections == nil {
			return unavailableBalance()
		}
		connection, err := s.connections.Open(need.connectionID)
		if err != nil {
			return unavailableBalance()
		}
		reader, ok := connection.(procurementcontract.BalanceReader)
		if !ok {
			return unavailableBalance()
		}
		amount, currency, err := reader.ReadBalance(ctx)
		if err != nil || !strings.EqualFold(strings.TrimSpace(currency), need.currency) {
			return unavailableBalance()
		}
		balance, err := decimal.NewFromString(strings.TrimSpace(amount))
		if err != nil || balance.IsNegative() {
			return unavailableBalance()
		}
		if balance.LessThan(need.amount) {
			status := "not_sent"
			if s.balanceNotifier != nil {
				status = s.balanceNotifier.NotifyBalanceShortage(ctx, need.connectionID, need.amount.StringFixed(2), balance.StringFixed(2), need.currency)
			}
			return &procurementcontract.PaymentBlock{Code: procurementcontract.BalanceInsufficient, NotificationStatus: status, NoticeID: need.connectionID}
		}
	}
	return nil
}

// ReadPaymentBalanceNotice only reads delivery state; polling never queries the
// supplier, sends an email, creates a payment, or submits procurement.
func (s *Service) ReadPaymentBalanceNotice(ctx context.Context, orderID, noticeID uint) string {
	if s.balanceNotifier == nil {
		return "not_sent"
	}
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil || order == nil {
		return "not_sent"
	}
	needs, err := s.fundingNeeds(order)
	if err != nil {
		return "not_sent"
	}
	for _, need := range needs {
		if need.connectionID != noticeID {
			continue
		}
		status := s.balanceNotifier.BalanceAlertStatus(ctx, need.connectionID)
		if status != "not_sent" {
			return status
		}
	}
	return "not_sent"
}
