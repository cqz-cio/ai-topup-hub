package contract

import (
	"time"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// ListFilter 礼品卡仓储列表筛选。
type ListFilter struct {
	Code           string
	Status         string
	BatchNo        string
	RedeemedUserID uint
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	RedeemedFrom   *time.Time
	RedeemedTo     *time.Time
	ExpiresFrom    *time.Time
	ExpiresTo      *time.Time
	Page           int
	PageSize       int
}

// Repository 是礼品卡管理用例的数据端口。
type Repository interface {
	CreateBatch(batch *giftcarddomain.GiftCardBatch, cards []giftcarddomain.GiftCard) error
	GetByID(id uint) (*giftcarddomain.GiftCard, error)
	List(filter ListFilter) ([]giftcarddomain.GiftCard, int64, error)
	ListByIDs(ids []uint) ([]giftcarddomain.GiftCard, error)
	Update(card *giftcarddomain.GiftCard) error
	Delete(id uint) error
	BatchUpdateStatus(ids []uint, status string, updatedAt time.Time) (int64, error)
	WithinTransaction(fn func(repo Repository) error) error
}

// WalletCreditInput 描述礼品卡兑换需要的钱包入账事实。
type WalletCreditInput struct {
	UserID    uint
	Amount    money.Amount
	Currency  string
	TxnType   string
	Reference string
	Remark    string
	OrderID   *uint
}

// RedeemTransaction 是礼品卡与钱包共享事务内的最小能力集合。
type RedeemTransaction interface {
	GetByCodeForUpdate(code string) (*giftcarddomain.GiftCard, error)
	UpdateCard(card *giftcarddomain.GiftCard) error
	CreditWallet(input WalletCreditInput) (*walletdomain.Account, *walletdomain.Transaction, error)
}

// RedeemTransactionRunner 保证礼品卡状态与钱包入账原子提交或回滚。
type RedeemTransactionRunner interface {
	WithinRedeemTransaction(fn func(tx RedeemTransaction) error) error
}

// UserDirectory 是兑换用户解析端口。
type UserDirectory interface {
	ListByIDs(ids []uint) ([]userdomain.User, error)
}

// CurrencyProvider 是站点币种读取端口。
type CurrencyProvider interface {
	SiteCurrency() string
}
