package aicdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const BaseURL = "https://www.aicdkshop.com"

type Client struct {
	baseURL, key string
	http         *http.Client
}

func New(key string) *Client {
	return &Client{baseURL: BaseURL, key: strings.TrimSpace(key), http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) Ready() bool { return c.key != "" }

func (c *Client) InspectAccount(ctx context.Context, session string) (*contract.Account, error) {
	var out struct {
		OK           bool   `json:"ok"`
		Pending      bool   `json:"pending"`
		AccountID    string `json:"account_id"`
		Subscription *struct {
			Plan string `json:"plan_type"`
		} `json:"subscription"`
	}
	if err := c.do(ctx, "POST", "/api/partner/v1/subscription", "", map[string]string{"session": session}, &out); err != nil {
		return nil, err
	}
	if !out.OK || out.Pending || out.AccountID == "" || out.Subscription == nil || out.Subscription.Plan == "" {
		return nil, contract.ErrUnavailable
	}
	return &contract.Account{ID: out.AccountID, Plan: out.Subscription.Plan}, nil
}

// No request/response bodies or transport error strings (which may contain URLs)
// escape this adapter. Callers must reconcile ambiguous write failures.
func (c *Client) do(ctx context.Context, method, path, key string, body, out interface{}) error {
	if !c.Ready() {
		return contract.ErrUnavailable
	}
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return contract.ErrInvalid
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &buf)
	if err != nil {
		return errors.New("partner_request_invalid")
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("partner_transport_unknown")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errors.New("partner_response_unknown")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("partner_http_%d", resp.StatusCode)
	}
	if err = json.Unmarshal(data, out); err != nil {
		return errors.New("partner_response_invalid")
	}
	return nil
}
func (c *Client) Check(ctx context.Context, b contract.Binding) error {
	var quota struct {
		OK        bool           `json:"ok"`
		Available int            `json:"available"`
		Costs     map[string]int `json:"plan_costs"`
	}
	if err := c.do(ctx, "GET", "/api/partner/v1/quota", "", nil, &quota); err != nil {
		return err
	}
	cost, ok := quota.Costs[b.Plan]
	if !quota.OK || !ok || cost <= 0 || quota.Available < cost {
		return errors.New("insufficient_quota")
	}
	var cfg struct {
		OK      bool `json:"ok"`
		Regions []struct {
			Country string `json:"country"`
			Version int    `json:"version"`
			Plans   []struct {
				Plan     string   `json:"plan"`
				Channels []string `json:"protocol_channels"`
			} `json:"plans"`
		} `json:"regions"`
	}
	if err := c.do(ctx, "GET", "/api/config", "", nil, &cfg); err != nil {
		return err
	}
	if cfg.OK {
		for _, r := range cfg.Regions {
			if r.Country == b.Region && r.Version == b.RegionVersion {
				for _, p := range r.Plans {
					if p.Plan == b.Plan {
						for _, ch := range p.Channels {
							if ch == b.Channel {
								return nil
							}
						}
					}
				}
			}
		}
	}
	return errors.New("region_configuration_changed")
}
func (c *Client) Pay(ctx context.Context, key string, r contract.PayRequest) (*contract.Result, error) {
	var out contract.Result
	err := c.do(ctx, "POST", "/api/partner/v1/pay", key, r, &out)
	return &out, err
}
func (c *Client) Poll(ctx context.Context, id string) (*contract.Result, error) {
	var out contract.Result
	err := c.do(ctx, "GET", "/api/partner/v1/operations/"+url.PathEscape(id), "", nil, &out)
	return &out, err
}
func (c *Client) Finalize(ctx context.Context, r contract.FinalizeRequest) (*contract.Result, error) {
	var out contract.Result
	err := c.do(ctx, "POST", "/api/partner/v1/pay/finalize", "", r, &out)
	return &out, err
}
func (c *Client) SubscriptionActive(ctx context.Context, session, plan string) (bool, error) {
	var out struct {
		OK           bool `json:"ok"`
		Subscription *struct {
			Plan        string `json:"plan_type"`
			ActiveUntil string `json:"active_until"`
		} `json:"subscription"`
	}
	err := c.do(ctx, "POST", "/api/partner/v1/subscription", "", map[string]string{"session": session}, &out)
	if err != nil {
		return false, err
	}
	if !out.OK || out.Subscription == nil {
		return false, nil
	}
	want := map[string]string{"chatgptplusplan": "plus", "chatgptprolite": "prolite", "chatgptpro": "pro"}[plan]
	until, err := time.Parse(time.RFC3339, out.Subscription.ActiveUntil)
	return err == nil && until.After(time.Now()) && out.Subscription.Plan == want, nil
}
