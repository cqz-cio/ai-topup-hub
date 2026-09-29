package application

import (
	"time"

	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func (s *Service) GetAccount(userID uint) (*walletdomain.Account, error) {
	if userID == 0 {
		return nil, walletcontract.ErrAccountNotFound
	}
	account, err := s.repository.GetAccountByUserID(userID)
	if err != nil || account != nil {
		return account, err
	}
	now := time.Now()
	account = &walletdomain.Account{
		UserID: userID, Balance: money.FromDecimal(decimal.Zero), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.CreateAccount(account); err != nil {
		created, queryErr := s.repository.GetAccountByUserID(userID)
		if queryErr == nil && created != nil {
			return created, nil
		}
		return nil, walletcontract.ErrAccountCreateFailed
	}
	return account, nil
}

func (s *Service) ListTransactions(filter walletcontract.TransactionListFilter) ([]walletdomain.Transaction, int64, error) {
	return s.repository.ListTransactions(filter)
}

func (s *Service) ListRechargeOrdersAdmin(filter walletcontract.RechargeListFilter) ([]walletdomain.RechargeOrder, int64, error) {
	return s.repository.ListRechargeOrdersAdmin(filter)
}

func (s *Service) ListUserRechargeOrders(userID uint, page, pageSize int, status, rechargeNo string) ([]walletdomain.RechargeOrder, int64, error) {
	if userID == 0 {
		return nil, 0, walletcontract.ErrAccountNotFound
	}
	return s.repository.ListRechargeOrdersAdmin(walletcontract.RechargeListFilter{
		Page: page, PageSize: pageSize, UserID: userID, Status: status, RechargeNo: rechargeNo,
	})
}

func (s *Service) StatsUserRechargeOrders(userID uint, rechargeNo string) (map[string]int64, error) {
	if userID == 0 {
		return nil, walletcontract.ErrAccountNotFound
	}
	return s.repository.StatsRechargeOrders(walletcontract.RechargeListFilter{UserID: userID, RechargeNo: rechargeNo})
}

func (s *Service) GetRechargeOrderByRechargeNo(userID uint, rechargeNo string) (*walletdomain.RechargeOrder, error) {
	if userID == 0 {
		return nil, walletcontract.ErrRechargeNotFound
	}
	order, err := s.repository.GetRechargeOrderByRechargeNo(userID, rechargeNo)
	if err != nil || order != nil {
		return order, err
	}
	return nil, walletcontract.ErrRechargeNotFound
}

func (s *Service) GetRechargeOrderByPaymentIDAndUser(paymentID, userID uint) (*walletdomain.RechargeOrder, error) {
	if paymentID == 0 || userID == 0 {
		return nil, walletcontract.ErrRechargeNotFound
	}
	order, err := s.repository.GetRechargeOrderByPaymentIDAndUser(paymentID, userID)
	if err != nil || order != nil {
		return order, err
	}
	return nil, walletcontract.ErrRechargeNotFound
}

func (s *Service) GetBalancesByUserIDs(userIDs []uint) (map[uint]money.Amount, error) {
	result := make(map[uint]money.Amount, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	accounts, err := s.repository.GetAccountsByUserIDs(userIDs)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		result[account.UserID] = account.Balance
	}
	return result, nil
}
