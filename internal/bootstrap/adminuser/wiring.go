package adminuserwiring

import (
	"github.com/dujiao-next/internal/app/container"
	adminusertransport "github.com/dujiao-next/internal/modules/identity/user/transport/http/admin"
)

func NewHandler(c *container.Container) *adminusertransport.AdminHandler {
	return adminusertransport.NewAdminHandler(
		adminUserDirectoryAdapter{users: c.UserStore},
		adminUserEmailAdapter{},
		adminUserWalletAdapter{wallets: c.WalletService},
		adminUserOAuthAdapter{identities: c.ExternalIdentityStore},
		adminUserOAuthUnbindAdapter{auth: c.UserAuthService},
		adminUserCouponUsageAdapter{usages: c.CouponUsageRepo},
		adminUserCouponAdapter{coupons: c.CouponRepo},
		adminUserProductAdapter{products: c.ProductRepo},
		adminUserAuthStateAdapter{},
	)
}
