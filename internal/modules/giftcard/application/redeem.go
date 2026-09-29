package application

import (
	"fmt"
	"strings"
	"time"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	"github.com/dujiao-next/internal/constants"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"

	"github.com/shopspring/decimal"
)

// RedeemGiftCard 在单一事务中完成礼品卡锁定、钱包入账和兑换状态更新。
func (s *Service) RedeemGiftCard(input RedeemInput) (*giftcarddomain.GiftCard, *walletdomain.Account, *walletdomain.Transaction, error) {
	if s == nil || s.redeemer == nil {
		return nil, nil, nil, giftcardcontract.ErrFetchFailed
	}
	code := strings.TrimSpace(strings.ToUpper(input.Code))
	if input.UserID == 0 || code == "" {
		return nil, nil, nil, giftcardcontract.ErrInvalid
	}

	var (
		resultCard *giftcarddomain.GiftCard
		resultAcc  *walletdomain.Account
		resultTxn  *walletdomain.Transaction
	)
	err := s.redeemer.WithinRedeemTransaction(func(tx giftcardcontract.RedeemTransaction) error {
		card, err := tx.GetByCodeForUpdate(code)
		if err != nil {
			return giftcardcontract.ErrFetchFailed
		}
		if card == nil {
			return giftcardcontract.ErrNotFound
		}
		switch card.Status {
		case giftcarddomain.GiftCardStatusRedeemed:
			return giftcardcontract.ErrRedeemed
		case giftcarddomain.GiftCardStatusDisabled:
			return giftcardcontract.ErrDisabled
		case giftcarddomain.GiftCardStatusActive:
		default:
			return giftcardcontract.ErrInvalid
		}
		if isGiftCardExpired(card.ExpiresAt, time.Now()) {
			return giftcardcontract.ErrExpired
		}
		if card.Amount.Decimal.Round(2).LessThanOrEqual(decimal.Zero) {
			return giftcardcontract.ErrInvalid
		}

		now := time.Now()
		account, txn, err := tx.CreditWallet(giftcardcontract.WalletCreditInput{
			UserID:    input.UserID,
			Amount:    card.Amount,
			Currency:  card.Currency,
			TxnType:   constants.WalletTxnTypeGiftCard,
			Reference: fmt.Sprintf("gift_card:%d", card.ID),
			Remark:    fmt.Sprintf("礼品卡兑换：%s", card.Code),
		})
		if err != nil {
			return err
		}

		card.Status = giftcarddomain.GiftCardStatusRedeemed
		card.RedeemedUserID = &input.UserID
		card.RedeemedAt = &now
		if txn != nil && txn.ID > 0 {
			card.WalletTxnID = &txn.ID
		}
		card.UpdatedAt = now
		if err := tx.UpdateCard(card); err != nil {
			return giftcardcontract.ErrUpdateFailed
		}
		resultCard = card
		resultAcc = account
		resultTxn = txn
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return resultCard, resultAcc, resultTxn, nil
}

func isGiftCardExpired(expiresAt *time.Time, now time.Time) bool {
	if expiresAt == nil || expiresAt.IsZero() {
		return false
	}
	return expiresAt.Before(now)
}
