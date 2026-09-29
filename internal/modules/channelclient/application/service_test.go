package channelclientapp

import (
	"testing"
	"time"

	channelclientcontract "github.com/dujiao-next/internal/modules/channelclient/contract"
	channelclientdomain "github.com/dujiao-next/internal/modules/channelclient/domain"
	"github.com/dujiao-next/internal/upstream"
)

var _ channelclientcontract.Store = (*storeStub)(nil)

type storeStub struct {
	items map[uint]*channelclientdomain.Client
	next  uint
}

func (store *storeStub) Create(client *channelclientdomain.Client) error {
	if store.items == nil {
		store.items = make(map[uint]*channelclientdomain.Client)
	}
	store.next++
	client.ID = store.next
	copyValue := *client
	store.items[client.ID] = &copyValue
	return nil
}

func (store *storeStub) FindByID(id uint) (*channelclientdomain.Client, error) {
	return store.copyMatching(func(client *channelclientdomain.Client) bool { return client.ID == id }), nil
}

func (store *storeStub) FindByChannelKey(key string) (*channelclientdomain.Client, error) {
	return store.copyMatching(func(client *channelclientdomain.Client) bool { return client.ChannelKey == key }), nil
}

func (store *storeStub) FindActiveByChannelType(channelType string) (*channelclientdomain.Client, error) {
	return store.copyMatching(func(client *channelclientdomain.Client) bool {
		return client.ChannelType == channelType && client.Status == 1
	}), nil
}

func (store *storeStub) FindAll() ([]channelclientdomain.Client, error) {
	result := make([]channelclientdomain.Client, 0, len(store.items))
	for _, client := range store.items {
		result = append(result, *client)
	}
	return result, nil
}

func (store *storeStub) Update(client *channelclientdomain.Client) error {
	copyValue := *client
	store.items[client.ID] = &copyValue
	return nil
}

func (store *storeStub) UpdateLastUsed(id uint, usedAt time.Time) error {
	store.items[id].LastUsedAt = &usedAt
	return nil
}

func (store *storeStub) Delete(id uint, deletedAt time.Time) error {
	store.items[id].DeletedAt = &deletedAt
	return nil
}

func (store *storeStub) copyMatching(matches func(*channelclientdomain.Client) bool) *channelclientdomain.Client {
	for _, client := range store.items {
		if client.DeletedAt == nil && matches(client) {
			copyValue := *client
			return &copyValue
		}
	}
	return nil
}

func TestServiceEncryptsCredentialsAndResolvesIntegrationViews(t *testing.T) {
	store := &storeStub{}
	service := NewService(store, "test-app-secret")

	detail, err := service.CreateChannelClient(
		"Telegram Bot",
		"telegram_bot",
		"integration",
		"123456:plain-bot-token",
		"https://bot.example.test",
	)
	if err != nil {
		t.Fatalf("create channel client failed: %v", err)
	}
	persisted := store.items[detail.ID]
	if persisted.ChannelSecret == "" || persisted.ChannelSecret == detail.ChannelSecret {
		t.Fatal("channel secret must be persisted as ciphertext")
	}
	if persisted.BotToken == "" || persisted.BotToken == "123456:plain-bot-token" {
		t.Fatal("bot token must be persisted as ciphertext")
	}
	if !detail.BotTokenSet || detail.BotToken == "123456:plain-bot-token" {
		t.Fatalf("admin detail must expose only a masked token: %#v", detail)
	}

	token, err := service.ResolveBotTokenByType("telegram_bot")
	if err != nil {
		t.Fatalf("resolve bot token failed: %v", err)
	}
	if token != "123456:plain-bot-token" {
		t.Fatalf("unexpected resolved token: %q", token)
	}

	endpoint, err := service.GetActiveEndpoint("telegram_bot")
	if err != nil {
		t.Fatalf("resolve active endpoint failed: %v", err)
	}
	if endpoint.ChannelKey != detail.ChannelKey || endpoint.ChannelSecret != detail.ChannelSecret {
		t.Fatalf("unexpected active endpoint: %#v", endpoint)
	}
}

func TestServiceVerifiesSignatureAndMarksUsageThroughStore(t *testing.T) {
	store := &storeStub{}
	service := NewService(store, "test-app-secret")
	detail, err := service.CreateChannelClient("Channel", "telegram_bot", "", "", "")
	if err != nil {
		t.Fatalf("create channel client failed: %v", err)
	}

	timestamp := time.Now().Unix()
	body := []byte(`{"order_id":1}`)
	signature := upstream.Sign(detail.ChannelSecret, "POST", "/api/v1/channel/orders", timestamp, body)
	client, err := service.VerifyChannelSignature(
		detail.ChannelKey,
		signature,
		timestamp,
		"POST",
		"/api/v1/channel/orders",
		body,
	)
	if err != nil {
		t.Fatalf("verify channel signature failed: %v", err)
	}
	if client.ID != detail.ID {
		t.Fatalf("unexpected verified client id: %d", client.ID)
	}

	usedAt := time.Now().Add(time.Second)
	if err := service.MarkUsed(client.ID, usedAt); err != nil {
		t.Fatalf("mark channel used failed: %v", err)
	}
	if store.items[client.ID].LastUsedAt == nil || !store.items[client.ID].LastUsedAt.Equal(usedAt) {
		t.Fatalf("last-used timestamp was not persisted: %#v", store.items[client.ID].LastUsedAt)
	}
}
