package application

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func (s *Service) CreditInTransaction(
	tx walletcontract.Transaction,
	input walletcontract.CreditInput,
) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if tx == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}
	if input.UserID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	reference := strings.TrimSpace(input.Reference)
	if reference == "" {
		return nil, nil, walletcontract.ErrTransactionCreateFailed
	}
	transactionType := strings.TrimSpace(input.Type)
	if transactionType == "" {
		transactionType = constants.WalletTxnTypeRecharge
	}
	repository := tx.Wallets()
	if repository == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}

	existing, err := repository.GetTransactionByReference(reference)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		account, accountErr := repository.GetAccountByUserID(input.UserID)
		if accountErr != nil {
			return nil, nil, accountErr
		}
		if account == nil {
			account, accountErr = ensureAccountForUpdate(repository, input.UserID, time.Now())
			if accountErr != nil {
				return nil, nil, accountErr
			}
		}
		return account, existing, nil
	}

	now := time.Now()
	account, err := ensureAccountForUpdate(repository, input.UserID, now)
	if err != nil {
		return nil, nil, err
	}
	before := account.Balance.Decimal.Round(2)
	after := before.Add(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateAccount(account); err != nil {
		return nil, nil, walletcontract.ErrAccountUpdateFailed
	}

	transaction := &walletdomain.Transaction{
		UserID: input.UserID, OrderID: input.OrderID,
		Type: transactionType, Direction: constants.WalletTxnDirectionIn,
		Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before),
		BalanceAfter: money.FromDecimal(after), Currency: normalizeCurrency(input.Currency),
		Reference: reference, Remark: cleanRemark(input.Remark, "钱包入账"),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateTransaction(transaction); err != nil {
		return nil, nil, walletcontract.ErrTransactionCreateFailed
	}
	return account, transaction, nil
}

func (s *Service) changeBalance(
	userID uint,
	delta decimal.Decimal,
	transactionType string,
	orderID *uint,
	reference, remark, currency string,
	operatorAdminID *uint,
) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if s.transactions == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}
	var accountResult *walletdomain.Account
	var transactionResult *walletdomain.Transaction
	if err := s.transactions.WithinTransaction(func(tx walletcontract.Transaction) error {
		repository := tx.Wallets()
		now := time.Now()
		account, err := ensureAccountForUpdate(repository, userID, now)
		if err != nil {
			return err
		}

		before := account.Balance.Decimal.Round(2)
		after := before.Add(delta).Round(2)
		if after.LessThan(decimal.Zero) {
			return walletcontract.ErrInsufficientBalance
		}
		direction := constants.WalletTxnDirectionIn
		amount := delta.Round(2)
		if delta.LessThan(decimal.Zero) {
			direction = constants.WalletTxnDirectionOut
			amount = delta.Abs().Round(2)
		}

		account.Balance = money.FromDecimal(after)
		account.UpdatedAt = now
		if err := repository.UpdateAccount(account); err != nil {
			return walletcontract.ErrAccountUpdateFailed
		}
		transaction := &walletdomain.Transaction{
			UserID: userID, OperatorAdminID: operatorAdminID, OrderID: orderID, Type: transactionType, Direction: direction,
			Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before),
			BalanceAfter: money.FromDecimal(after), Currency: normalizeCurrency(currency),
			Reference: strings.TrimSpace(reference), Remark: remark, CreatedAt: now, UpdatedAt: now,
		}
		if err := repository.CreateTransaction(transaction); err != nil {
			return walletcontract.ErrTransactionCreateFailed
		}
		accountResult, transactionResult = account, transaction
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return accountResult, transactionResult, nil
}

func ensureAccountForUpdate(repository walletcontract.Repository, userID uint, now time.Time) (*walletdomain.Account, error) {
	if repository == nil {
		return nil, walletcontract.ErrTransactionRequired
	}
	account, err := repository.GetAccountByUserIDForUpdate(userID)
	if err != nil {
		return nil, err
	}
	if account != nil {
		return account, nil
	}
	account = &walletdomain.Account{
		UserID: userID, Balance: money.FromDecimal(decimal.Zero), CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateAccount(account); err != nil {
		created, queryErr := repository.GetAccountByUserIDForUpdate(userID)
		if queryErr == nil && created != nil {
			return created, nil
		}
		return nil, walletcontract.ErrAccountCreateFailed
	}
	return account, nil
}
