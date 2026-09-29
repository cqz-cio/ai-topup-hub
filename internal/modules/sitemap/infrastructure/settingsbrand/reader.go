package settingsbrand

import settingsapp "github.com/dujiao-next/internal/modules/settings/application"

// Reader 从设置应用服务读取站点 URL。
type Reader struct {
	settings *settingsapp.Service
}

// New 创建站点品牌读取适配器。
func New(settings *settingsapp.Service) Reader {
	return Reader{settings: settings}
}

// GetSiteURL 返回配置的站点根地址。
func (r Reader) GetSiteURL() (string, error) {
	if r.settings == nil {
		return "", nil
	}
	brand, err := r.settings.GetSiteBrand()
	if err != nil {
		return "", err
	}
	return brand.SiteURL, nil
}
