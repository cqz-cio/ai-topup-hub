// Package supportagent is a loopback-only local support prototype. It has no
// access to store credentials, orders, payment services or supplier operations.
package supportagent

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
)

type Config struct {
	PublicOrigin      string `json:"-"`           // Set only by the operator's command-line option.
	AllowModel        bool   `json:"allow_model"` // Explicit opt-in; omitted means offline.
	APIKey            string `json:"api_key"`
	Model             string `json:"model"`
	MaxOutput         int    `json:"max_output_tokens"`
	DailyBudget       int64  `json:"daily_budget_nanousd"`
	DailyCalls        int    `json:"daily_calls"`
	SessionDailyCalls int    `json:"session_daily_calls"`
	SessionPerMinute  int    `json:"session_per_minute"`
	GlobalPerMinute   int    `json:"global_per_minute"`
}

func OfflineConfig() Config {
	return Config{SessionPerMinute: 6, GlobalPerMinute: 20}
}

func ValidatePublicOrigin(origin string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Port() == "" && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && u.Host == u.Hostname()
}

func ReadConfig(path string) (Config, error) {
	c := Config{}
	b, err := os.ReadFile(path)
	if err != nil {
		return c, errors.New("private configuration unavailable")
	}
	if json.Unmarshal(b, &c) != nil {
		return c, errors.New("invalid private configuration")
	}
	if len(c.APIKey) < 20 || c.Model != "deepseek-flash" || c.MaxOutput < 100 || c.MaxOutput > 1000 ||
		c.DailyBudget < 1 || c.DailyBudget > 1000000000 || c.DailyCalls < 1 || c.DailyCalls > 200 ||
		c.SessionDailyCalls < 1 || c.SessionDailyCalls > c.DailyCalls || c.SessionPerMinute < 1 ||
		c.SessionPerMinute > 10 || c.GlobalPerMinute < 1 || c.GlobalPerMinute > 30 {
		return c, errors.New("invalid local support limits or model")
	}
	return c, nil
}
