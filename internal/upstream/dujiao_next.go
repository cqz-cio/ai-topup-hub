package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"github.com/dujiao-next/internal/logger"

	"github.com/google/uuid"
)

// upstreamHTTPError 上游返回非 200 时的结构化错误
type upstreamHTTPError struct {
	Status  int
	Code    string
	Message string
	Body    string
}

func (e *upstreamHTTPError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("upstream responded with status %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("upstream responded with status %d: %s", e.Status, e.Body)
}

// extractUpstreamErrorCode 从错误链中提取 upstreamHTTPError.Code
func extractUpstreamErrorCode(err error) string {
	var ue *upstreamHTTPError
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}

// DujiaoNextAdapter Dujiao-Next 协议适配器
type DujiaoNextAdapter struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	uploadsDir string
	client     *http.Client
}

// NewDujiaoNextAdapter 创建 Dujiao-Next 适配器
func NewDujiaoNextAdapter(conn *siteconnectiondomain.Connection, uploadsDir string) *DujiaoNextAdapter {
	return &DujiaoNextAdapter{
		baseURL:    strings.TrimRight(conn.BaseURL, "/"),
		apiKey:     conn.ApiKey,
		apiSecret:  conn.ApiSecret,
		uploadsDir: uploadsDir,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping 连接测试
func (a *DujiaoNextAdapter) Ping(ctx context.Context) (*PingResult, error) {
	var result struct {
		OK bool `json:"ok"`
		PingResult
	}
	if err := a.doRequest(ctx, http.MethodPost, "/api/v1/upstream/ping", nil, &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("ping failed")
	}
	return &result.PingResult, nil
}

// ListCategories 拉取上游分类列表
func (a *DujiaoNextAdapter) ListCategories(ctx context.Context) (*CategoryListResult, error) {
	var result struct {
		OK         bool               `json:"ok"`
		Categories []UpstreamCategory `json:"categories"`
	}
	if err := a.doRequest(ctx, http.MethodGet, "/api/v1/upstream/categories", nil, &result); err != nil {
		// 旧版上游不支持分类 API，返回空列表
		var ue *upstreamHTTPError
		if errors.As(err, &ue) && ue.Status == http.StatusNotFound {
			return &CategoryListResult{Supported: false, Categories: []UpstreamCategory{}}, nil
		}
		return nil, err
	}
	return &CategoryListResult{Supported: true, Categories: result.Categories}, nil
}

// ListProducts 拉取上游商品列表
func (a *DujiaoNextAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	path := fmt.Sprintf("/api/v1/upstream/products?page=%d&page_size=%d", opts.Page, opts.PageSize)
	if opts.UpdatedAfter != nil {
		path += "&updated_after=" + opts.UpdatedAfter.Format(time.RFC3339)
	}
	if opts.IncludeInactive {
		path += "&include_inactive=true"
	}
	var result ProductListResult
	if err := a.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProduct 获取单个商品详情
// 上游已删除（软删）→ 返回 ErrUpstreamProductDeleted
// 旧版上游对下架商品也返回 404 product_unavailable → 返回 ErrUpstreamProductUnavailable
// 新版上游下架商品改为 200 + is_active=false，调用方应根据 IsActive 字段判断
func (a *DujiaoNextAdapter) GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error) {
	path := fmt.Sprintf("/api/v1/upstream/products/%d", productID)
	var result struct {
		OK      bool            `json:"ok"`
		Product UpstreamProduct `json:"product"`
	}
	if err := a.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		// 解析上游返回的 error_code 归一化为哨兵错误
		switch extractUpstreamErrorCode(err) {
		case "product_deleted", "product_not_found":
			return nil, ErrUpstreamProductDeleted
		case "product_unavailable":
			return nil, ErrUpstreamProductUnavailable
		}
		return nil, err
	}
	return &result.Product, nil
}

// CreateOrder 发起采购单
func (a *DujiaoNextAdapter) CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	var result CreateUpstreamOrderResp
	if err := a.doRequest(ctx, http.MethodPost, "/api/v1/upstream/orders", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOrder 查询上游订单状态
func (a *DujiaoNextAdapter) GetOrder(ctx context.Context, orderID uint) (*UpstreamOrderDetail, error) {
	path := fmt.Sprintf("/api/v1/upstream/orders/%d", orderID)
	var result UpstreamOrderDetail
	if err := a.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelOrder 取消采购单
func (a *DujiaoNextAdapter) CancelOrder(ctx context.Context, orderID uint) error {
	path := fmt.Sprintf("/api/v1/upstream/orders/%d/cancel", orderID)
	var result struct {
		OK bool `json:"ok"`
	}
	if err := a.doRequest(ctx, http.MethodPost, path, nil, &result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("cancel order failed")
	}
	return nil
}

// DownloadImage 下载图片到本地
func (a *DujiaoNextAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	// 相对路径转绝对 URL
	fullURL := imageURL
	if strings.HasPrefix(imageURL, "/") {
		fullURL = a.baseURL + imageURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}

	// 确定文件扩展名
	ext := filepath.Ext(imageURL)
	if ext == "" || len(ext) > 6 {
		ext = ".jpg"
	}
	// 去除 query string
	if idx := strings.Index(ext, "?"); idx > 0 {
		ext = ext[:idx]
	}

	filename := uuid.New().String() + ext
	dir := filepath.Join(a.uploadsDir, "upstream")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create uploads dir: %w", err)
	}

	filePath := filepath.Join(dir, filename)
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// 返回相对路径
	return "/uploads/upstream/" + filename, nil
}

// doRequest 发送签名请求
func (a *DujiaoNextAdapter) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
	}

	// 签名用的 path 不含 query string
	signPath := path
	if idx := strings.Index(path, "?"); idx > 0 {
		signPath = path[:idx]
	}

	timestamp := time.Now().Unix()
	signature := Sign(a.apiSecret, method, signPath, timestamp, bodyBytes)

	url := a.baseURL + path
	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set(HeaderApiKey, a.apiKey)
	req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", timestamp))
	req.Header.Set(HeaderSignature, signature)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Warnw("upstream_request_error",
			"method", method, "path", path,
			"status", resp.StatusCode, "body", string(respBody))
		// 尝试解析结构化错误响应
		var errPayload struct {
			ErrorCode    string `json:"error_code"`
			ErrorMessage string `json:"error_message"`
		}
		_ = json.Unmarshal(respBody, &errPayload)
		return &upstreamHTTPError{
			Status:  resp.StatusCode,
			Code:    errPayload.ErrorCode,
			Message: errPayload.ErrorMessage,
			Body:    string(respBody),
		}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}
