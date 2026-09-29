package callbackclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
	"github.com/dujiao-next/internal/upstream"
)

const signaturePath = "/api/v1/upstream/callback"

// Client 执行符合上游协议的签名 HTTP 回调。
type Client struct {
	httpClient *http.Client
}

var _ downstreamcontract.Deliverer = (*Client)(nil)

func New() *Client {
	return NewWithHTTPClient(newSafeHTTPClient())
}

func NewWithHTTPClient(client *http.Client) *Client {
	if client == nil {
		panic("downstream callback client: http client is nil")
	}
	return &Client{httpClient: client}
}

func (c *Client) Send(ctx context.Context, request downstreamcontract.DeliveryRequest) error {
	body, err := json.Marshal(request.Payload)
	if err != nil {
		return err
	}
	signature := upstream.Sign(request.APISecret, http.MethodPost, signaturePath, request.Payload.Timestamp, body)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, request.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set(upstream.HeaderApiKey, request.APIKey)
	httpRequest.Header.Set(upstream.HeaderTimestamp, fmt.Sprintf("%d", request.Payload.Timestamp))
	httpRequest.Header.Set(upstream.HeaderSignature, signature)

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
	if readErr != nil {
		return readErr
	}
	if response.StatusCode == http.StatusOK {
		var result struct {
			OK bool `json:"ok"`
		}
		if json.Unmarshal(body, &result) == nil && result.OK {
			return nil
		}
	}
	return fmt.Errorf("callback returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
}
