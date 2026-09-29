package adgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/modules/adproxy/contract"
	adproxydomain "github.com/dujiao-next/internal/modules/adproxy/domain"
)

// Client 通过 HTTP 访问广告网关。
type Client struct {
	client  *http.Client
	baseURL string
}

var _ contract.Gateway = (*Client)(nil)

// New 创建使用当前构建环境默认地址的广告网关客户端。
func New() *Client {
	return NewClient(&http.Client{Timeout: 5 * time.Second}, ServerURL)
}

// NewClient 创建可注入 HTTP 客户端和网关地址的广告网关客户端。
func NewClient(client *http.Client, baseURL string) *Client {
	if client == nil {
		panic("adgateway client: http client is nil")
	}
	return &Client{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

// RenderSlot 请求 ad-system 渲染指定广告位
func (s *Client) RenderSlot(ctx context.Context, slotCode string, params map[string]string) (*adproxydomain.RenderResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v1/public/ad-slots/%s/render", s.baseURL, url.PathEscape(slotCode)))
	if err != nil {
		return nil, fmt.Errorf("ad_proxy: invalid url: %w", err)
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("ad_proxy: create request failed: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		logger.Warnw("ad_proxy_render_slot_failed", "slot_code", slotCode, "error", err)
		return nil, fmt.Errorf("ad_proxy: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ad_proxy: read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Warnw("ad_proxy_render_slot_non_ok", "slot_code", slotCode, "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("ad_proxy: upstream returned %d", resp.StatusCode)
	}

	var apiResp struct {
		StatusCode int                           `json:"status_code"`
		Msg        string                        `json:"msg"`
		Data       *adproxydomain.RenderResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("ad_proxy: decode response failed: %w", err)
	}
	if apiResp.StatusCode != 0 || apiResp.Data == nil {
		return nil, fmt.Errorf("ad_proxy: upstream error: %s", apiResp.Msg)
	}

	return apiResp.Data, nil
}

// ReportImpression 上报广告曝光
func (s *Client) ReportImpression(ctx context.Context, payload json.RawMessage) error {
	u := fmt.Sprintf("%s/api/v1/public/ad-events/impression", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ad_proxy: create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		logger.Warnw("ad_proxy_report_impression_failed", "error", err)
		return fmt.Errorf("ad_proxy: request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ad_proxy: upstream returned %d", resp.StatusCode)
	}
	return nil
}
