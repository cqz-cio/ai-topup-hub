package channeluserwiring

import userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"

type SimpleProvisioner struct {
	auth *userauthapp.Service
}

func NewSimpleProvisioner(auth *userauthapp.Service) SimpleProvisioner {
	return SimpleProvisioner{auth: auth}
}

func (p SimpleProvisioner) ProvisionUserID(channelUserID string) (uint, error) {
	user, _, _, err := p.auth.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: channelUserID,
	})
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, userauthapp.ErrNotFound
	}
	return user.ID, nil
}
