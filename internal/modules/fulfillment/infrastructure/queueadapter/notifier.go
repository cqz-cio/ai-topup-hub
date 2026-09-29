package queueadapter

import (
	"strings"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"

	"github.com/dujiao-next/internal/queue"
)

// BotNotifier 把平台队列适配成交付通知端口。
type BotNotifier struct {
	client *queue.Client
}

var _ fulfillmentapp.BotNotifier = (*BotNotifier)(nil)

func NewBotNotifier(client *queue.Client) *BotNotifier {
	return &BotNotifier{client: client}
}

func (n *BotNotifier) EnqueueOrderFulfilled(telegramUserID string, orderID uint) error {
	if n == nil || n.client == nil {
		return nil
	}
	return n.client.EnqueueBotNotify(queue.BotNotifyPayload{
		EventType:      queue.BotNotifyEventOrderFulfilled,
		OrderID:        orderID,
		TelegramUserID: strings.TrimSpace(telegramUserID),
	})
}
