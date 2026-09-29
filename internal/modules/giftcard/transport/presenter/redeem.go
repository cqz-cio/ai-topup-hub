package presenter

import (
	"time"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	walletpresenter "github.com/dujiao-next/internal/modules/wallet/transport/presenter"
	"github.com/dujiao-next/internal/shared/money"
)

// GiftCardRedeemResp 礼品卡兑换结果响应
type GiftCardRedeemResp struct {
	GiftCard    GiftCardResp                          `json:"gift_card"`
	Wallet      walletpresenter.WalletAccountResp     `json:"wallet"`
	Transaction walletpresenter.WalletTransactionResp `json:"transaction"`
	WalletDelta money.Amount                          `json:"wallet_delta"`
}

// GiftCardResp 礼品卡响应（兑换后）
type GiftCardResp struct {
	ID         uint         `json:"id"`
	Name       string       `json:"name"`
	Code       string       `json:"code"`
	Amount     money.Amount `json:"amount"`
	Currency   string       `json:"currency"`
	Status     string       `json:"status"`
	RedeemedAt *time.Time   `json:"redeemed_at"`
}

// NewGiftCardResp 从 giftcarddomain.GiftCard 构造响应
func NewGiftCardResp(c *giftcarddomain.GiftCard) GiftCardResp {
	return GiftCardResp{
		ID:         c.ID,
		Name:       c.Name,
		Code:       c.Code,
		Amount:     c.Amount,
		Currency:   c.Currency,
		Status:     c.Status,
		RedeemedAt: c.RedeemedAt,
	}
	// 排除：BatchID、ExpiresAt、RedeemedUserID、WalletTxnID、CreatedAt、UpdatedAt、Batch
}

// NewGiftCardRedeemResp 构造完整兑换响应
func NewGiftCardRedeemResp(card *giftcarddomain.GiftCard, account *walletdomain.Account, txn *walletdomain.Transaction) GiftCardRedeemResp {
	return GiftCardRedeemResp{
		GiftCard:    NewGiftCardResp(card),
		Wallet:      walletpresenter.NewWalletAccountResp(account),
		Transaction: walletpresenter.NewWalletTransactionResp(txn),
		WalletDelta: card.Amount,
	}
}
