package application

import giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"

// Service 礼品卡管理用例（不含兑换写路径）。
type Service struct {
	repo     giftcardcontract.Repository
	users    giftcardcontract.UserDirectory
	currency giftcardcontract.CurrencyProvider
	redeemer giftcardcontract.RedeemTransactionRunner
}

// Options 组装管理用例依赖。
type Options struct {
	Repo     giftcardcontract.Repository
	Users    giftcardcontract.UserDirectory
	Currency giftcardcontract.CurrencyProvider
	Redeemer giftcardcontract.RedeemTransactionRunner
}

func NewService(opts Options) *Service {
	if opts.Repo == nil {
		panic("giftcard service: repo is nil")
	}
	return &Service{
		repo:     opts.Repo,
		users:    opts.Users,
		currency: opts.Currency,
		redeemer: opts.Redeemer,
	}
}
