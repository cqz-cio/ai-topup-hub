package application

import (
	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	"github.com/shopspring/decimal"
)

func (s *Service) Recharge(input walletcontract.RechargeInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if input.UserID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	return s.changeBalance(
		input.UserID, amount, constants.WalletTxnTypeRecharge, nil,
		uniqueReference("recharge", input.UserID),
		cleanRemark(input.Remark, "用户充值"),
		normalizeCurrency(input.Currency),
		nil,
	)
}

func (s *Service) AdminAdjustBalance(input walletcontract.AdjustBalanceInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if input.UserID == 0 || input.OperatorAdminID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	delta := input.Delta.Decimal.Round(2)
	if delta.IsZero() {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	return s.changeBalance(
		input.UserID, delta, constants.WalletTxnTypeAdminAdjust, nil,
		uniqueReference("admin_adjust", input.UserID),
		cleanRemark(input.Remark, "管理员调整余额"),
		normalizeCurrency(input.Currency),
		&input.OperatorAdminID,
	)
}
