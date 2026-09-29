package adminuserwiring

import (
	"context"
	"errors"
	"fmt"

	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/cache"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	externalidentitycontract "github.com/dujiao-next/internal/modules/identity/externalidentity/contract"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	adminusertransport "github.com/dujiao-next/internal/modules/identity/user/transport/http/admin"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
)

type adminUserDirectoryAdapter struct {
	users usercontract.Store
}

func (a adminUserDirectoryAdapter) List(filter adminusertransport.UserListFilter) ([]userdomain.User, int64, error) {
	return a.users.List(usercontract.ListFilter{
		Page:          filter.Page,
		PageSize:      filter.PageSize,
		UserID:        filter.UserID,
		Keyword:       filter.Keyword,
		Status:        filter.Status,
		CreatedFrom:   filter.CreatedFrom,
		CreatedTo:     filter.CreatedTo,
		LastLoginFrom: filter.LastLoginFrom,
		LastLoginTo:   filter.LastLoginTo,
		SortBy:        filter.SortBy,
		SortOrder:     filter.SortOrder,
	})
}

func (a adminUserDirectoryAdapter) GetByID(id uint) (*userdomain.User, error) {
	return a.users.GetByID(id)
}

func (a adminUserDirectoryAdapter) GetByEmail(email string) (*userdomain.User, error) {
	return a.users.GetByEmail(email)
}

func (a adminUserDirectoryAdapter) Update(user *userdomain.User) error {
	return a.users.Update(user)
}

func (a adminUserDirectoryAdapter) BatchUpdateStatus(ids []uint, status string) error {
	return a.users.BatchUpdateStatus(ids, status)
}

type adminUserEmailAdapter struct{}

func (adminUserEmailAdapter) NormalizeEmail(email string) (string, error) {
	normalized, err := userauthapp.NormalizeEmail(email)
	if err != nil {
		return "", mapAdminUserTransportError(err)
	}
	return normalized, nil
}

type adminUserWalletAdapter struct {
	wallets *walletapp.Service
}

func (a adminUserWalletAdapter) GetBalancesByUserIDs(userIDs []uint) (map[uint]money.Amount, error) {
	return a.wallets.GetBalancesByUserIDs(userIDs)
}

func (a adminUserWalletAdapter) GetAccount(userID uint) (*walletdomain.Account, error) {
	return a.wallets.GetAccount(userID)
}

type adminUserOAuthAdapter struct {
	identities externalidentitycontract.Store
}

func (a adminUserOAuthAdapter) ListByUserID(userID uint) ([]externalidentitydomain.Identity, error) {
	return a.identities.ListByUserID(userID)
}

type adminUserOAuthUnbindAdapter struct {
	auth *userauthapp.Service
}

func (a adminUserOAuthUnbindAdapter) UnbindTelegram(userID uint) error {
	return mapAdminUserTransportError(a.auth.UnbindTelegram(userID))
}

func (a adminUserOAuthUnbindAdapter) UnbindGoogle(userID uint) error {
	return mapAdminUserTransportError(a.auth.UnbindGoogle(userID))
}

type adminUserCouponUsageAdapter struct {
	usages couponcontract.UsageRepository
}

func (a adminUserCouponUsageAdapter) ListByUser(filter couponcontract.UsageListFilter) ([]coupondomain.CouponUsage, int64, error) {
	return a.usages.ListByUser(filter)
}

type adminUserCouponAdapter struct {
	coupons couponcontract.Repository
}

func (a adminUserCouponAdapter) ListByIDs(ids []uint) ([]coupondomain.Coupon, error) {
	return a.coupons.ListByIDs(ids)
}

type adminUserProductAdapter struct {
	products productcontract.Repository
}

func (a adminUserProductAdapter) ListByIDs(ids []uint) ([]productdomain.Product, error) {
	return a.products.ListByIDs(ids)
}

type adminUserAuthStateAdapter struct{}

func (adminUserAuthStateAdapter) SetUserAuthState(ctx context.Context, user *userdomain.User) error {
	return cache.SetUserAuthState(ctx, cache.BuildUserAuthState(user))
}

func (adminUserAuthStateAdapter) DelUserAuthState(ctx context.Context, userID uint) error {
	return cache.DelUserAuthState(ctx, userID)
}

func mapAdminUserTransportError(err error) error {
	if err == nil {
		return nil
	}
	for _, mapping := range []struct {
		source error
		target error
	}{
		{userauthapp.ErrNotFound, adminusertransport.ErrNotFound},
		{userauthapp.ErrUserDisabled, adminusertransport.ErrUserDisabled},
		{userauthapp.ErrUserOAuthNotBound, adminusertransport.ErrUserOAuthNotBound},
		{userauthapp.ErrTelegramUnbindRequiresEmail, adminusertransport.ErrTelegramUnbindRequiresEmail},
		{userauthapp.ErrGoogleUnbindLocked, adminusertransport.ErrGoogleUnbindLocked},
		{userauthapp.ErrInvalidEmail, adminusertransport.ErrInvalidEmail},
	} {
		if errors.Is(err, mapping.source) {
			return fmt.Errorf("%w: %v", mapping.target, err)
		}
	}
	return err
}
