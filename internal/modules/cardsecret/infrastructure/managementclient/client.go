package managementclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/cardsecret/contract"
)

type Client struct {
	origin, token string
	http          *http.Client
}

func New(origin, token string) (*Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) || len(token) < 32 {
		return nil, errors.New("invalid PAIPLUS management origin or credential")
	}
	return &Client{origin: strings.TrimRight(origin, "/"), token: token, http: &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("management redirect rejected") }}}, nil
}

func FromEnvironment() (contract.Management, error) {
	token := os.Getenv("PAIPLUS_MANAGEMENT_TOKEN")
	if token == "" {
		return nil, nil
	}
	if token == os.Getenv("PAIPLUS_INTEGRATION_TOKEN") {
		return nil, errors.New("PAIPLUS management requires a separate credential")
	}
	return New(os.Getenv("PAIPLUS_ORIGIN"), token)
}

var messages = map[string]string{
	"CARD_INVALID":       "卡密无效，销毁失败：该卡密已使用或上游已失效。",
	"CARD_REVOKED":       "卡密无效，销毁失败：这张本站卡密已销毁。",
	"CARD_IN_USE":        "销毁失败：卡密正在充值或已有提交记录，请先核对充值结果，不要据此退款。",
	"CARD_BUSY":          "销毁失败：卡密正在核验，请稍后重试。",
	"CHECK_UNAVAILABLE":  "暂时无法确认上游状态，操作未确认成功。请刷新卡密状态，不要据此退款。",
	"CONFIRM_REQUIRED":   "请填写销毁原因，并确认永久停用这张本站卡密。",
	"OPERATION_CONFLICT": "操作状态已变化，请刷新卡密详情核对。",
	"NOT_FOUND":          "卡密不存在。",
	"FILTER_INVALID":     "请选择有效卡密状态。",
	"RATE_LIMITED":       "操作较频繁，请稍后重试。",
}

func (c *Client) Call(ctx context.Context, operation string, input contract.ManagementInput, adminID uint) (json.RawMessage, error) {
	if adminID == 0 || (operation != "cards" && operation != "detail" && operation != "check" && operation != "revoke") {
		return nil, errors.New("invalid card management operation")
	}
	payload := struct {
		contract.ManagementInput
		Actor string `json:"actor"`
	}{input, fmt.Sprintf("shop-admin:%d", adminID)}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("card management request failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.origin+"/api/management/"+operation, bytes.NewReader(encoded))
	if err != nil {
		return nil, errors.New("card management request failed")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, &contract.ManagementError{Code: "CHECK_UNAVAILABLE", Message: messages["CHECK_UNAVAILABLE"]}
	}
	defer res.Body.Close()
	var envelope struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&envelope) != nil {
		return nil, errors.New("invalid card management response")
	}
	if res.StatusCode != http.StatusOK || !envelope.OK {
		if message, ok := messages[envelope.Error.Code]; ok {
			return nil, &contract.ManagementError{Code: envelope.Error.Code, Message: message}
		}
		return nil, errors.New("card management service unavailable")
	}
	if len(envelope.Data) < 2 || envelope.Data[0] != '{' || !json.Valid(envelope.Data) {
		return nil, errors.New("incomplete card management response")
	}
	return envelope.Data, nil
}
