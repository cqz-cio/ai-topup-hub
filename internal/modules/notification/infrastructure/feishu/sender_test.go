package feishu

import (
	"context"
	"errors"
	"testing"

	"github.com/dujiao-next/internal/modules/notification/contract"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

type fakeMessageClient struct {
	calls         int
	receiveIDType string
	receiveID     string
	message       string
	err           error
}

func (c *fakeMessageClient) SendText(_ context.Context, receiveIDType, receiveID, message string) error {
	c.calls++
	c.receiveIDType = receiveIDType
	c.receiveID = receiveID
	c.message = message
	return c.err
}

func TestNewSDKConfigPreservesRequiredClientSettings(t *testing.T) {
	config := newSDKConfig("cli_demo", "secret")

	if config.BaseUrl != feishuBaseURL {
		t.Fatalf("unexpected base URL: %q", config.BaseUrl)
	}
	if config.AppId != "cli_demo" || config.AppSecret != "secret" {
		t.Fatalf("unexpected credentials: app_id=%q app_secret=%q", config.AppId, config.AppSecret)
	}
	if config.ReqTimeout != feishuRequestTimeout {
		t.Fatalf("unexpected request timeout: %v", config.ReqTimeout)
	}
	if config.LogLevel != larkcore.LogLevelError {
		t.Fatalf("unexpected log level: %v", config.LogLevel)
	}
	if config.AppType != larkcore.AppTypeSelfBuilt || !config.EnableTokenCache {
		t.Fatalf("unexpected app settings: app_type=%v token_cache=%v", config.AppType, config.EnableTokenCache)
	}
	if config.Logger == nil || config.Serializable == nil || config.HttpClient == nil {
		t.Fatal("SDK core services must be initialized")
	}
}

func TestSenderSendsTrimmedMessageAndReusesClientForCredentials(t *testing.T) {
	var clients []*fakeMessageClient
	var credentials [][2]string
	sender := newSender(func(appID, appSecret string) messageClient {
		client := &fakeMessageClient{}
		clients = append(clients, client)
		credentials = append(credentials, [2]string{appID, appSecret})
		return client
	})

	if err := sender.SendMessage(context.Background(), " cli_demo ", " secret ", " CHAT_ID ", " oc_first ", " hello "); err != nil {
		t.Fatalf("send first message: %v", err)
	}
	if err := sender.SendMessage(context.Background(), "cli_demo", "secret", "chat_id", "oc_second", "world"); err != nil {
		t.Fatalf("send second message: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("same credentials should reuse one client, got %d", len(clients))
	}
	if credentials[0] != [2]string{"cli_demo", "secret"} {
		t.Fatalf("unexpected factory credentials: %#v", credentials[0])
	}
	if clients[0].calls != 2 || clients[0].receiveID != "oc_second" || clients[0].message != "world" {
		t.Fatalf("unexpected send calls: %#v", clients[0])
	}

	if err := sender.SendMessage(context.Background(), "cli_other", "secret-2", "open_id", "ou_user", "next"); err != nil {
		t.Fatalf("send with changed credentials: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("changed credentials should create another client, got %d", len(clients))
	}
	if clients[1].receiveIDType != "open_id" || clients[1].receiveID != "ou_user" {
		t.Fatalf("unexpected changed-client request: %#v", clients[1])
	}
}

func TestSenderRejectsInvalidConfiguration(t *testing.T) {
	sender := newSender(func(_, _ string) messageClient { return &fakeMessageClient{} })
	tests := []struct {
		name          string
		appID         string
		appSecret     string
		receiveIDType string
		receiveID     string
		message       string
	}{
		{"missing app id", "", "secret", "chat_id", "oc_demo", "message"},
		{"missing app secret", "cli_demo", "", "chat_id", "oc_demo", "message"},
		{"unsupported receive id type", "cli_demo", "secret", "department_id", "od_demo", "message"},
		{"missing receive id", "cli_demo", "secret", "chat_id", "", "message"},
		{"missing message", "cli_demo", "secret", "chat_id", "oc_demo", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := sender.SendMessage(context.Background(), test.appID, test.appSecret, test.receiveIDType, test.receiveID, test.message)
			if !errors.Is(err, contract.ErrConfigInvalid) {
				t.Fatalf("expected invalid config, got %v", err)
			}
		})
	}
}

func TestSenderReturnsClientError(t *testing.T) {
	wantErr := errors.New("feishu unavailable")
	sender := newSender(func(_, _ string) messageClient { return &fakeMessageClient{err: wantErr} })
	err := sender.SendMessage(context.Background(), "cli_demo", "secret", "chat_id", "oc_demo", "message")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected client error, got %v", err)
	}
}
