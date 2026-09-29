package telegrambroadcast

import (
	"context"
	"errors"

	channelclientapp "github.com/dujiao-next/internal/modules/channelclient/application"
	externalidentitycontract "github.com/dujiao-next/internal/modules/identity/externalidentity/contract"
	broadcastapp "github.com/dujiao-next/internal/modules/telegram/broadcast/application"
	"github.com/dujiao-next/internal/queue"
)

type UserDirectory struct {
	store externalidentitycontract.Store
}

func NewUserDirectory(store externalidentitycontract.Store) UserDirectory {
	return UserDirectory{store: store}
}

func (directory UserDirectory) ListTelegramUsers(query broadcastapp.UserQuery) ([]broadcastapp.UserItem, int64, error) {
	items, total, err := directory.store.ListTelegramUsers(externalidentitycontract.TelegramUserFilter{
		Page: query.Page, PageSize: query.PageSize, UserIDs: query.UserIDs,
		Keyword: query.Keyword, DisplayName: query.DisplayName,
		TelegramUsername: query.TelegramUsername, TelegramUserID: query.TelegramUserID,
		CreatedFrom: query.CreatedFrom, CreatedTo: query.CreatedTo,
	})
	if err != nil {
		return nil, 0, err
	}
	result := make([]broadcastapp.UserItem, 0, len(items))
	for _, item := range items {
		result = append(result, broadcastapp.UserItem{
			UserID: item.UserID, DisplayName: item.DisplayName, UserEmail: item.UserEmail,
			TelegramUsername: item.TelegramUsername, TelegramUserID: item.TelegramUserID,
			BoundAt: item.BoundAt, UserCreatedAt: item.UserCreatedAt,
		})
	}
	return result, total, nil
}

type BotTokenResolver struct {
	service *channelclientapp.Service
}

func NewBotTokenResolver(service *channelclientapp.Service) BotTokenResolver {
	return BotTokenResolver{service: service}
}

func (resolver BotTokenResolver) ResolveActiveBotToken() (string, error) {
	if resolver.service == nil {
		return "", broadcastapp.ErrTokenUnavailable
	}
	token, err := resolver.service.ResolveBotTokenByType("telegram_bot")
	if err != nil {
		if errors.Is(err, channelclientapp.ErrNotFound) {
			return "", broadcastapp.ErrTokenUnavailable
		}
		return "", err
	}
	return token, nil
}

type Dispatcher struct {
	queue *queue.Client
}

func NewDispatcher(queueClient *queue.Client) Dispatcher {
	return Dispatcher{queue: queueClient}
}

func (dispatcher Dispatcher) DispatchBroadcast(_ context.Context, broadcastID uint) (bool, error) {
	if dispatcher.queue == nil || !dispatcher.queue.Enabled() {
		return false, nil
	}
	if err := dispatcher.queue.EnqueueTelegramBroadcast(queue.TelegramBroadcastPayload{BroadcastID: broadcastID}); err != nil {
		return false, err
	}
	return true, nil
}
