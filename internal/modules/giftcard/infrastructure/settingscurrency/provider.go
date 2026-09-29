package settingscurrency

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
)

type Provider struct {
	settings *settingsapp.Service
}

func New(settings *settingsapp.Service) Provider {
	return Provider{settings: settings}
}

func (provider Provider) SiteCurrency() string {
	if provider.settings == nil {
		return constants.SiteCurrencyDefault
	}
	currency, err := provider.settings.GetSiteCurrency(constants.SiteCurrencyDefault)
	if err != nil || strings.TrimSpace(currency) == "" {
		return constants.SiteCurrencyDefault
	}
	return currency
}
