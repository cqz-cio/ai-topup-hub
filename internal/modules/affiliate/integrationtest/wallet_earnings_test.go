package integrationtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	affiliatestore "github.com/dujiao-next/internal/modules/affiliate/infrastructure/gormstore"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/testkit/memorysettings"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func affiliateWalletFixture(t *testing.T) (*gorm.DB, *affiliateapp.Service, *affiliatestore.Store, *orderdomain.Order) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:affiliate_wallet_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := db.DB()
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { raw.Close() })
	if err := db.AutoMigrate(&userdomain.User{}, &orderdomain.Order{}, &orderdomain.OrderItem{}, &orderdomain.OrderRefundRecord{}, &affiliatedomain.Profile{}, &affiliatedomain.Commission{}, &walletdomain.Account{}, &walletdomain.Transaction{}, &walletdomain.EarningSettlement{}); err != nil {
		t.Fatal(err)
	}
	profile := createAffiliateTestProfile(t, db, 1, "WALLET01", constants.AffiliateProfileStatusActive)
	settings := settingsapp.NewService(memorysettings.New())
	if _, err := settings.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{Enabled: true, CommissionRate: 2, ConfirmDays: 30, MinWithdrawAmount: 20, WithdrawChannels: []string{"alipay", "wechat"}}); err != nil {
		t.Fatal(err)
	}
	store := affiliatestore.New(db)
	svc := affiliateapp.NewService(store, nil, nil, nil, settings)
	paid := time.Now().Add(-24 * time.Hour)
	amount := money.FromDecimal(decimal.NewFromInt(100))
	order := &orderdomain.Order{OrderNo: fmt.Sprintf("LOCAL-AFF-%d", time.Now().UnixNano()), UserID: 2, Currency: "CNY", Status: constants.OrderStatusPaid, PaidAt: &paid, AffiliateProfileID: &profile.ID, OriginalAmount: amount, TotalAmount: amount, WalletPaidAmount: amount, Items: []orderdomain.OrderItem{{ProductID: 1, SKUID: 1, Quantity: 1, OriginalUnitPrice: amount, UnitPrice: amount, OriginalTotalPrice: amount, TotalPrice: amount, AffiliateOriginalPrice: true, AffiliateProductEnabled: true, FulfillmentType: constants.FulfillmentTypeManual}}}
	order.Items[0].TitleJSON = jsonmap.JSON{"zh-CN": "本地测试商品"}
	return db, svc, store, order
}

func TestOnlyOriginalPriceOrdersEarnAffiliateIncome(t *testing.T) {
	cases := []struct {
		name   string
		change func(*orderdomain.Order)
		want   int64
	}{
		{"original paid with wallet", func(o *orderdomain.Order) {}, 1},
		{"personal price snapshot", func(o *orderdomain.Order) { o.Items[0].AffiliateOriginalPrice = false }, 0},
		{"historical order without eligibility snapshot", func(o *orderdomain.Order) {
			o.Items[0].AffiliateOriginalPrice = false
			o.Items[0].AffiliateProductEnabled = false
		}, 0},
		{"member", func(o *orderdomain.Order) { o.Items[0].MemberDiscount = money.FromDecimal(decimal.NewFromInt(1)) }, 0},
		{"promotion", func(o *orderdomain.Order) { o.Items[0].PromotionDiscount = money.FromDecimal(decimal.NewFromInt(1)) }, 0},
		{"wholesale", func(o *orderdomain.Order) { o.Items[0].WholesaleDiscount = money.FromDecimal(decimal.NewFromInt(1)) }, 0},
		{"coupon", func(o *orderdomain.Order) { o.Items[0].CouponDiscount = money.FromDecimal(decimal.NewFromInt(1)) }, 0},
		{"self purchase", func(o *orderdomain.Order) { o.UserID = 1 }, 0},
		{"reseller", func(o *orderdomain.Order) { id := uint(3); o.ResellerID = &id }, 0},
		{"disabled product", func(o *orderdomain.Order) { o.Items[0].AffiliateProductEnabled = false }, 0},
		{"unpaid", func(o *orderdomain.Order) { o.PaidAt = nil }, 0},
		{"one discounted line excludes whole order", func(o *orderdomain.Order) {
			other := o.Items[0]
			other.ProductID = 2
			other.AffiliateOriginalPrice = false
			o.Items = append(o.Items, other)
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, svc, _, order := affiliateWalletFixture(t)
			tc.change(order)
			if err := db.Create(order).Error; err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := svc.HandleOrderPaid(order.ID); err != nil {
					t.Fatal(err)
				}
			}
			var count int64
			db.Model(&affiliatedomain.Commission{}).Count(&count)
			if count != tc.want {
				t.Fatalf("commissions=%d want %d", count, tc.want)
			}
		})
	}
}

func TestAffiliateMaturityCreditsWalletAndRefundReversesIt(t *testing.T) {
	db, svc, store, order := affiliateWalletFixture(t)
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleOrderPaid(order.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmDueCommissions(time.Now()); err != nil {
		t.Fatal(err)
	}
	var accounts int64
	db.Model(&walletdomain.Account{}).Count(&accounts)
	if accounts != 0 {
		t.Fatal("pending commission became spendable")
	}
	for i := 0; i < 2; i++ {
		if err := svc.ConfirmDueCommissions(time.Now().Add(31 * 24 * time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	var account walletdomain.Account
	if err := db.Where("user_id = ?", 1).First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Balance.String() != "2.00" || account.WithdrawableBalance.String() != "2.00" {
		t.Fatalf("incorrect settled wallet %+v", account)
	}
	if err := store.WithinTransaction(func(tx affiliatecontract.Store) error {
		return svc.HandleOrderRefunded(tx, order, decimal.NewFromInt(50), decimal.Zero, "test refund")
	}); err != nil {
		t.Fatal(err)
	}
	db.First(&account, account.ID)
	if account.Balance.String() != "1.00" || account.WithdrawableBalance.String() != "1.00" {
		t.Fatalf("refund did not reverse wallet %+v", account)
	}
	if err := svc.ConfirmDueCommissions(time.Now().Add(31 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	db.First(&account, account.ID)
	if account.Balance.String() != "1.00" {
		t.Fatal("refund income was credited again")
	}
}

func TestAffiliateRecoveryRepairsMissingPaymentHookOnce(t *testing.T) {
	db, svc, _, order := affiliateWalletFixture(t)
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := svc.ConfirmDueCommissions(time.Now().Add(31 * 24 * time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	db.Model(&affiliatedomain.Commission{}).Count(&count)
	if count != 1 {
		t.Fatalf("recovery commissions=%d", count)
	}
	var a walletdomain.Account
	if err := db.Where("user_id = ?", 1).First(&a).Error; err != nil {
		t.Fatal(err)
	}
	if a.Balance.String() != "2.00" {
		t.Fatalf("recovered wallet %+v", a)
	}
}
