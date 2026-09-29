package integrationtest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/dujiao-next/internal/constants"
	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	giftcardgormstore "github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	giftcardredeemgormuow "github.com/dujiao-next/internal/workflows/giftcardredeem/infrastructure/gormuow"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type giftCardTestCurrencyProvider struct {
	settings *settingsapp.Service
}

func (provider giftCardTestCurrencyProvider) SiteCurrency() string {
	currency, err := provider.settings.GetSiteCurrency(constants.SiteCurrencyDefault)
	if err != nil {
		return constants.SiteCurrencyDefault
	}
	return currency
}

func setupGiftCardServiceTest(t *testing.T) (*giftcardapp.Service, *walletapp.Service, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:gift_card_service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&settingsstore.SettingRecord{},
		&giftcarddomain.GiftCardBatch{},
		&giftcarddomain.GiftCard{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db

	userRepo := userstore.New(db)
	settingRepo := settingsstore.New(db)
	settingSvc := settingsapp.NewService(settingRepo)
	walletStore := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletStore,
		Transactions: walletStore,
	})
	cardStore := giftcardgormstore.New(db)
	giftSvc := giftcardapp.NewService(giftcardapp.Options{
		Repo:     cardStore,
		Users:    userRepo,
		Currency: giftCardTestCurrencyProvider{settings: settingSvc},
		Redeemer: giftcardredeemgormuow.New(cardStore, walletSvc),
	})
	return giftSvc, walletSvc, db
}

func seedGiftCardUser(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	user := userdomain.User{
		ID:           id,
		Email:        fmt.Sprintf("gift_card_user_%d@example.com", id),
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
}

func TestGiftCardServiceGenerateGiftCards(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	adminID := uint(999)
	batch, created, err := svc.Generate(giftcardapp.GenerateInput{
		Name:      "测试礼品卡",
		Quantity:  3,
		Amount:    money.FromDecimal(decimal.RequireFromString("25.00")),
		CreatedBy: &adminID,
	})
	if err != nil {
		t.Fatalf("generate gift cards failed: %v", err)
	}
	if batch == nil || batch.ID == 0 {
		t.Fatalf("invalid batch result: %+v", batch)
	}
	if created != 3 {
		t.Fatalf("expected created=3, got: %d", created)
	}

	var count int64
	if err := db.Model(&giftcarddomain.GiftCard{}).Where("batch_id = ?", batch.ID).Count(&count).Error; err != nil {
		t.Fatalf("count gift cards failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 gift cards in batch, got: %d", count)
	}
}

func TestGiftCardServiceGenerateGiftCardsUsesSiteCurrency(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	settingRepo := settingsstore.New(db)
	settingSvc := settingsapp.NewService(settingRepo)
	_, err := settingSvc.Update(constants.SettingKeySiteConfig, map[string]interface{}{
		constants.SettingFieldSiteCurrency: "USD",
	})
	if err != nil {
		t.Fatalf("set site currency failed: %v", err)
	}

	batch, created, err := svc.Generate(giftcardapp.GenerateInput{
		Name:     "站点币种礼品卡",
		Quantity: 2,
		Amount:   money.FromDecimal(decimal.RequireFromString("9.90")),
	})
	if err != nil {
		t.Fatalf("generate gift cards failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected created=2, got: %d", created)
	}
	if batch == nil {
		t.Fatal("batch should not be nil")
	}
	if batch.Currency != "USD" {
		t.Fatalf("expected batch currency USD, got: %s", batch.Currency)
	}

	var cards []giftcarddomain.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).Find(&cards).Error; err != nil {
		t.Fatalf("query gift cards failed: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 gift cards, got: %d", len(cards))
	}
	for _, card := range cards {
		if card.Currency != "USD" {
			t.Fatalf("expected card currency USD, got: %s", card.Currency)
		}
	}
}

func TestGiftCardServiceRedeemGiftCard(t *testing.T) {
	svc, walletSvc, db := setupGiftCardServiceTest(t)
	userID := uint(2001)
	seedGiftCardUser(t, db, userID)

	batch, _, err := svc.Generate(giftcardapp.GenerateInput{
		Name:     "兑换测试卡",
		Quantity: 1,
		Amount:   money.FromDecimal(decimal.RequireFromString("59.90")),
	})
	if err != nil {
		t.Fatalf("generate gift card failed: %v", err)
	}

	var card giftcarddomain.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&card).Error; err != nil {
		t.Fatalf("query generated card failed: %v", err)
	}

	redeemedCard, account, txn, err := svc.RedeemGiftCard(giftcardapp.RedeemInput{
		UserID: userID,
		Code:   card.Code,
	})
	if err != nil {
		t.Fatalf("redeem gift card failed: %v", err)
	}
	if redeemedCard == nil || redeemedCard.Status != giftcarddomain.GiftCardStatusRedeemed {
		t.Fatalf("unexpected redeemed card: %+v", redeemedCard)
	}
	if account == nil || !account.Balance.Decimal.Equal(decimal.RequireFromString("59.90")) {
		t.Fatalf("unexpected wallet account: %+v", account)
	}
	if txn == nil || txn.Type != constants.WalletTxnTypeGiftCard {
		t.Fatalf("unexpected wallet transaction: %+v", txn)
	}

	_, _, _, err = svc.RedeemGiftCard(giftcardapp.RedeemInput{
		UserID: userID,
		Code:   card.Code,
	})
	if !errors.Is(err, giftcardcontract.ErrRedeemed) {
		t.Fatalf("expected giftcardcontract.ErrRedeemed, got: %v", err)
	}

	accountAfter, err := walletSvc.GetAccount(userID)
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	if !accountAfter.Balance.Decimal.Equal(decimal.RequireFromString("59.90")) {
		t.Fatalf("unexpected account balance after duplicate redeem: %s", accountAfter.Balance.String())
	}
}

func TestGiftCardServiceRedeemExpiredGiftCard(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2002)
	seedGiftCardUser(t, db, userID)
	expiredAt := time.Now().Add(-1 * time.Hour)

	card := giftcarddomain.GiftCard{
		Name:      "过期礼品卡",
		Code:      "GC-EXPIRED-001",
		Amount:    money.FromDecimal(decimal.RequireFromString("10.00")),
		Currency:  "CNY",
		Status:    giftcarddomain.GiftCardStatusActive,
		ExpiresAt: &expiredAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create expired gift card failed: %v", err)
	}

	_, _, _, err := svc.RedeemGiftCard(giftcardapp.RedeemInput{
		UserID: userID,
		Code:   card.Code,
	})
	if !errors.Is(err, giftcardcontract.ErrExpired) {
		t.Fatalf("expected giftcardcontract.ErrExpired, got: %v", err)
	}
}

func TestGiftCardServiceBatchUpdateStatusSkipsRedeemed(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	now := time.Now()
	userID := uint(2003)
	seedGiftCardUser(t, db, userID)

	activeCard := giftcarddomain.GiftCard{
		Name:      "可变更礼品卡",
		Code:      "GC-BATCH-ACTIVE-001",
		Amount:    money.FromDecimal(decimal.RequireFromString("20.00")),
		Currency:  "CNY",
		Status:    giftcarddomain.GiftCardStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&activeCard).Error; err != nil {
		t.Fatalf("create active card failed: %v", err)
	}

	redeemedAt := now.Add(-10 * time.Minute)
	redeemedCard := giftcarddomain.GiftCard{
		Name:           "已兑换礼品卡",
		Code:           "GC-BATCH-REDEEMED-001",
		Amount:         money.FromDecimal(decimal.RequireFromString("30.00")),
		Currency:       "CNY",
		Status:         giftcarddomain.GiftCardStatusRedeemed,
		RedeemedAt:     &redeemedAt,
		RedeemedUserID: &userID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(&redeemedCard).Error; err != nil {
		t.Fatalf("create redeemed card failed: %v", err)
	}

	affected, err := svc.BatchUpdateStatus([]uint{activeCard.ID, redeemedCard.ID}, giftcarddomain.GiftCardStatusDisabled)
	if err != nil {
		t.Fatalf("batch update status failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected affected=1, got: %d", affected)
	}

	var checkActive giftcarddomain.GiftCard
	if err := db.First(&checkActive, activeCard.ID).Error; err != nil {
		t.Fatalf("query active card failed: %v", err)
	}
	if checkActive.Status != giftcarddomain.GiftCardStatusDisabled {
		t.Fatalf("expected active card status disabled, got: %s", checkActive.Status)
	}

	var checkRedeemed giftcarddomain.GiftCard
	if err := db.First(&checkRedeemed, redeemedCard.ID).Error; err != nil {
		t.Fatalf("query redeemed card failed: %v", err)
	}
	if checkRedeemed.Status != giftcarddomain.GiftCardStatusRedeemed {
		t.Fatalf("expected redeemed card status unchanged, got: %s", checkRedeemed.Status)
	}
}
