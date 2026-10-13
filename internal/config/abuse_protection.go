package config

// AbuseProtectionConfig limits work reaching the application. Values <= 0
// select safe defaults, rather than silently disabling protection.
type AbuseProtectionConfig struct {
	PublicPerMinute   int `mapstructure:"public_per_minute"`
	AuthPerMinute     int `mapstructure:"auth_per_minute"`
	CaptchaPerMinute  int `mapstructure:"captcha_per_minute"`
	SendCodePerMinute int `mapstructure:"send_code_per_minute"`
	MutationPerMinute int `mapstructure:"mutation_per_minute"`
	AuthMaxConcurrent int `mapstructure:"auth_max_concurrent"`
	RecipientPerHour  int `mapstructure:"recipient_per_hour"`
	RecipientPerDay   int `mapstructure:"recipient_per_day"`
	MailPerHour       int `mapstructure:"mail_per_hour"`
	MailPerDay        int `mapstructure:"mail_per_day"`
}

func (c AbuseProtectionConfig) WithDefaults() AbuseProtectionConfig {
	defaults := []struct {
		value    *int
		fallback int
	}{
		{&c.PublicPerMinute, 240}, {&c.AuthPerMinute, 30},
		{&c.CaptchaPerMinute, 20}, {&c.SendCodePerMinute, 5},
		{&c.MutationPerMinute, 30}, {&c.AuthMaxConcurrent, 16},
		{&c.RecipientPerHour, 10}, {&c.RecipientPerDay, 30},
		{&c.MailPerHour, 300}, {&c.MailPerDay, 2000},
	}
	for _, item := range defaults {
		if *item.value <= 0 {
			*item.value = item.fallback
		}
	}
	return c
}
