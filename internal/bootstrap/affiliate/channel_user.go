package affiliatebootstrap

import (
	affiliatetransport "github.com/dujiao-next/internal/modules/affiliate/transport/http"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
)

// affiliateChannelUserAdapter 连接身份模块与 Affiliate 渠道 transport。
type affiliateChannelUserAdapter struct {
	auth *userauthapp.Service
}

func (a affiliateChannelUserAdapter) ProvisionUserID(identity affiliatetransport.ChannelIdentity) (uint, error) {
	user, _, _, err := a.auth.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: identity.ChannelUserID,
		Username:      identity.Username,
		FirstName:     identity.FirstName,
		LastName:      identity.LastName,
		AvatarURL:     identity.AvatarURL,
	})
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, userauthapp.ErrNotFound
	}
	return user.ID, nil
}
