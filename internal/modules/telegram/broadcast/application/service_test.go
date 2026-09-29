package broadcastapp

import (
	"context"
	"errors"
	"testing"

	"github.com/dujiao-next/internal/constants"
	broadcastcontract "github.com/dujiao-next/internal/modules/telegram/broadcast/contract"
	broadcastdomain "github.com/dujiao-next/internal/modules/telegram/broadcast/domain"
	notifycontract "github.com/dujiao-next/internal/modules/telegram/notify/contract"
	"github.com/dujiao-next/internal/shared/jsonslice"
)

type broadcastStoreStub struct {
	items  map[uint]*broadcastdomain.Broadcast
	nextID uint
}

func (r *broadcastStoreStub) Create(broadcast *broadcastdomain.Broadcast) error {
	if broadcast == nil {
		return nil
	}
	if r.items == nil {
		r.items = map[uint]*broadcastdomain.Broadcast{}
	}
	r.nextID++
	broadcast.ID = r.nextID
	copyValue := *broadcast
	r.items[broadcast.ID] = &copyValue
	return nil
}

func (r *broadcastStoreStub) GetByID(id uint) (*broadcastdomain.Broadcast, error) {
	if item, ok := r.items[id]; ok {
		copyValue := *item
		return &copyValue, nil
	}
	return nil, nil
}

func (r *broadcastStoreStub) List(broadcastcontract.ListFilter) ([]broadcastdomain.Broadcast, int64, error) {
	items := make([]broadcastdomain.Broadcast, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, *item)
	}
	return items, int64(len(items)), nil
}

func (r *broadcastStoreStub) Update(broadcast *broadcastdomain.Broadcast) error {
	if broadcast == nil {
		return nil
	}
	copyValue := *broadcast
	r.items[broadcast.ID] = &copyValue
	return nil
}

type userDirectoryStub struct {
	items []UserItem
}

func (directory *userDirectoryStub) ListTelegramUsers(query UserQuery) ([]UserItem, int64, error) {
	allowed := make(map[uint]struct{}, len(query.UserIDs))
	for _, id := range query.UserIDs {
		allowed[id] = struct{}{}
	}
	result := make([]UserItem, 0, len(directory.items))
	for _, item := range directory.items {
		if len(allowed) > 0 {
			if _, ok := allowed[item.UserID]; !ok {
				continue
			}
		}
		result = append(result, item)
	}
	return result, int64(len(result)), nil
}

type tokenResolverStub struct {
	token string
	err   error
}

func (resolver tokenResolverStub) ResolveActiveBotToken() (string, error) {
	if resolver.err != nil {
		return "", resolver.err
	}
	return resolver.token, nil
}

type senderStub struct {
	failures map[string]error
	calls    []notifycontract.SendOptions
}

func (s *senderStub) SendWithBotToken(_ context.Context, _ string, options notifycontract.SendOptions) error {
	s.calls = append(s.calls, options)
	if err, ok := s.failures[options.ChatID]; ok {
		return err
	}
	return nil
}

func TestServiceCreateBroadcastValidation(t *testing.T) {
	service := NewService(
		&broadcastStoreStub{},
		&userDirectoryStub{},
		tokenResolverStub{err: ErrTokenUnavailable},
		nil,
		&senderStub{},
	)

	_, err := service.CreateBroadcast(context.Background(), CreateInput{
		Title:         "",
		RecipientType: constants.TelegramBroadcastRecipientTypeAll,
		MessageHTML:   "<b>hello</b>",
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid broadcast error, got %v", err)
	}

	_, err = service.CreateBroadcast(context.Background(), CreateInput{
		Title:         "Demo",
		RecipientType: constants.TelegramBroadcastRecipientTypeAll,
		MessageHTML:   "<b>hello</b>",
	})
	if !errors.Is(err, ErrTokenUnavailable) {
		t.Fatalf("expected token unavailable error, got %v", err)
	}
}

func TestServiceCreateBroadcastSnapshotsSpecificRecipients(t *testing.T) {
	store := &broadcastStoreStub{}
	service := NewService(
		store,
		&userDirectoryStub{items: []UserItem{
			{UserID: 1, TelegramUserID: "1001"},
			{UserID: 2, TelegramUserID: "1002"},
		}},
		tokenResolverStub{token: "bot-token"},
		queuedDispatcherStub{},
		&senderStub{},
	)

	created, err := service.CreateBroadcast(context.Background(), CreateInput{
		Title:         "Demo",
		RecipientType: constants.TelegramBroadcastRecipientTypeSpecific,
		UserIDs:       []uint{2, 2},
		MessageHTML:   "<b>hello</b>",
	})
	if err != nil {
		t.Fatalf("create broadcast failed: %v", err)
	}
	if created.RecipientCount != 1 || len(created.RecipientChatIDs) != 1 || created.RecipientChatIDs[0] != "1002" {
		t.Fatalf("unexpected recipient snapshot: %#v", created.RecipientChatIDs)
	}
	selected, ok := created.FiltersJSON["selected_user_ids"].([]uint)
	if !ok || len(selected) != 1 || selected[0] != 2 {
		t.Fatalf("unexpected selected user snapshot: %#v", created.FiltersJSON["selected_user_ids"])
	}
}

type queuedDispatcherStub struct{}

func (queuedDispatcherStub) DispatchBroadcast(context.Context, uint) (bool, error) {
	return true, nil
}

func TestServiceProcessBroadcastUpdatesStats(t *testing.T) {
	store := &broadcastStoreStub{
		items: map[uint]*broadcastdomain.Broadcast{
			1: {
				ID:               1,
				Title:            "Demo",
				RecipientType:    constants.TelegramBroadcastRecipientTypeSpecific,
				RecipientChatIDs: jsonslice.Strings{"1001", "1002"},
				RecipientCount:   2,
				Status:           constants.TelegramBroadcastStatusPending,
				MessageHTML:      "<b>Hello</b>",
			},
		},
		nextID: 1,
	}
	sender := &senderStub{
		failures: map[string]error{
			"1002": errors.New("send failed"),
		},
	}
	service := NewService(
		store,
		&userDirectoryStub{},
		tokenResolverStub{token: "bot-token"},
		nil,
		sender,
	)

	if err := service.ProcessBroadcast(context.Background(), 1); err != nil {
		t.Fatalf("process broadcast failed: %v", err)
	}

	updated, err := store.GetByID(1)
	if err != nil {
		t.Fatalf("get updated broadcast failed: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated broadcast")
	}
	if updated.SuccessCount != 1 || updated.FailedCount != 1 {
		t.Fatalf("unexpected stats: success=%d failed=%d", updated.SuccessCount, updated.FailedCount)
	}
	if updated.Status != constants.TelegramBroadcastStatusCompleted {
		t.Fatalf("expected completed status, got %s", updated.Status)
	}
	if len(sender.calls) != 2 {
		t.Fatalf("expected 2 send calls, got %d", len(sender.calls))
	}
}
