package broadcastapp

import (
	"context"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	broadcastcontract "github.com/dujiao-next/internal/modules/telegram/broadcast/contract"
	broadcastdomain "github.com/dujiao-next/internal/modules/telegram/broadcast/domain"
	notifycontract "github.com/dujiao-next/internal/modules/telegram/notify/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
)

// ListInput describes broadcast-list pagination.
type ListInput struct {
	Page     int
	PageSize int
}

// UserQuery describes the selectable Telegram user filter.
type UserQuery struct {
	Page             int
	PageSize         int
	UserIDs          []uint
	Keyword          string
	DisplayName      string
	TelegramUsername string
	TelegramUserID   string
	CreatedFrom      *time.Time
	CreatedTo        *time.Time
}

// UserItem is the broadcast module's user-directory projection.
type UserItem struct {
	UserID           uint      `json:"user_id"`
	DisplayName      string    `json:"display_name"`
	UserEmail        string    `json:"user_email"`
	TelegramUsername string    `json:"telegram_username"`
	TelegramUserID   string    `json:"telegram_user_id"`
	BoundAt          time.Time `json:"bound_at"`
	UserCreatedAt    time.Time `json:"user_created_at"`
}

// CreateInput contains the immutable input used to create a broadcast job.
type CreateInput struct {
	Title          string
	RecipientType  string
	UserIDs        []uint
	Filters        jsonmap.JSON
	AttachmentURL  string
	AttachmentName string
	MessageHTML    string
}

// UserDirectory provides the audience projection without exposing another
// module's persistence contract to the broadcast application layer.
type UserDirectory interface {
	ListTelegramUsers(UserQuery) ([]UserItem, int64, error)
}

// BotTokenResolver resolves the active Telegram bot credential.
type BotTokenResolver interface {
	ResolveActiveBotToken() (string, error)
}

// Dispatcher sends a persisted broadcast to asynchronous execution. It
// returns false when asynchronous delivery is unavailable and the service
// should execute in-process.
type Dispatcher interface {
	DispatchBroadcast(context.Context, uint) (bool, error)
}

// Sender delivers one Telegram message.
type Sender interface {
	SendWithBotToken(context.Context, string, notifycontract.SendOptions) error
}

// Service owns the Telegram broadcast use cases.
type Service struct {
	store      broadcastcontract.Store
	users      UserDirectory
	tokens     BotTokenResolver
	dispatcher Dispatcher
	sender     Sender
}

// NewService constructs the broadcast application service from module ports.
func NewService(
	store broadcastcontract.Store,
	users UserDirectory,
	tokens BotTokenResolver,
	dispatcher Dispatcher,
	sender Sender,
) *Service {
	return &Service{
		store:      store,
		users:      users,
		tokens:     tokens,
		dispatcher: dispatcher,
		sender:     sender,
	}
}

// GetBroadcast returns one broadcast job.
func (s *Service) GetBroadcast(id uint) (*broadcastdomain.Broadcast, error) {
	if s == nil || s.store == nil {
		return nil, ErrInvalid
	}
	return s.store.GetByID(id)
}

// ListBroadcasts returns persisted broadcast jobs.
func (s *Service) ListBroadcasts(input ListInput) ([]broadcastdomain.Broadcast, int64, error) {
	if s == nil || s.store == nil {
		return nil, 0, ErrInvalid
	}
	return s.store.List(broadcastcontract.ListFilter{
		Page:     input.Page,
		PageSize: input.PageSize,
	})
}

// ListTelegramUsers returns selectable Telegram recipients.
func (s *Service) ListTelegramUsers(input UserQuery) ([]UserItem, int64, error) {
	if s == nil || s.users == nil {
		return nil, 0, ErrInvalid
	}
	return s.users.ListTelegramUsers(input)
}

// CreateBroadcast validates, snapshots and dispatches a broadcast job.
func (s *Service) CreateBroadcast(ctx context.Context, input CreateInput) (*broadcastdomain.Broadcast, error) {
	if s == nil || s.store == nil || s.users == nil || s.tokens == nil {
		return nil, ErrInvalid
	}

	title := strings.TrimSpace(input.Title)
	messageHTML := strings.TrimSpace(input.MessageHTML)
	recipientType := strings.ToLower(strings.TrimSpace(input.RecipientType))
	if title == "" || messageHTML == "" {
		return nil, ErrInvalid
	}
	if recipientType != constants.TelegramBroadcastRecipientTypeAll && recipientType != constants.TelegramBroadcastRecipientTypeSpecific {
		return nil, ErrInvalid
	}

	if _, err := s.tokens.ResolveActiveBotToken(); err != nil {
		return nil, err
	}

	recipientChatIDs, filtersSnapshot, err := s.resolveRecipients(input)
	if err != nil {
		return nil, err
	}
	if len(recipientChatIDs) == 0 {
		return nil, ErrNoRecipients
	}

	broadcast := &broadcastdomain.Broadcast{
		Title:            title,
		RecipientType:    recipientType,
		FiltersJSON:      filtersSnapshot,
		RecipientChatIDs: jsonslice.Strings(recipientChatIDs),
		RecipientCount:   len(recipientChatIDs),
		Status:           constants.TelegramBroadcastStatusPending,
		MessageHTML:      messageHTML,
		AttachmentURL:    strings.TrimSpace(input.AttachmentURL),
		AttachmentName:   strings.TrimSpace(input.AttachmentName),
	}
	if err := s.store.Create(broadcast); err != nil {
		return nil, err
	}
	if err := s.dispatchBroadcast(ctx, broadcast.ID); err != nil {
		return nil, err
	}
	return broadcast, nil
}

// ProcessBroadcast delivers a persisted broadcast job.
func (s *Service) ProcessBroadcast(ctx context.Context, broadcastID uint) error {
	if s == nil || s.store == nil || s.tokens == nil || s.sender == nil {
		return ErrInvalid
	}
	broadcast, err := s.store.GetByID(broadcastID)
	if err != nil {
		return err
	}
	if broadcast == nil {
		return ErrNotFound
	}
	if broadcast.Status == constants.TelegramBroadcastStatusCompleted {
		return nil
	}

	now := time.Now()
	broadcast.Status = constants.TelegramBroadcastStatusRunning
	if broadcast.StartedAt == nil {
		broadcast.StartedAt = &now
	}
	broadcast.CompletedAt = nil
	broadcast.LastError = ""
	if err := s.store.Update(broadcast); err != nil {
		return err
	}

	token, err := s.tokens.ResolveActiveBotToken()
	if err != nil {
		return s.markBroadcastFailed(broadcast, err.Error())
	}

	chatIDs := dedupeStrings([]string(broadcast.RecipientChatIDs))
	if len(chatIDs) == 0 {
		return s.markBroadcastFailed(broadcast, ErrNoRecipients.Error())
	}

	successCount := 0
	failedCount := 0
	lastError := ""
	for _, chatID := range chatIDs {
		err := s.sender.SendWithBotToken(ctx, token, notifycontract.SendOptions{
			ChatID:                chatID,
			Message:               broadcast.MessageHTML,
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
			AttachmentURL:         broadcast.AttachmentURL,
			AttachmentDisplayName: broadcast.AttachmentName,
		})
		if err != nil {
			failedCount++
			lastError = err.Error()
			continue
		}
		successCount++
	}

	completedAt := time.Now()
	broadcast.SuccessCount = successCount
	broadcast.FailedCount = failedCount
	broadcast.CompletedAt = &completedAt
	broadcast.LastError = lastError
	if successCount == 0 && failedCount > 0 {
		broadcast.Status = constants.TelegramBroadcastStatusFailed
	} else {
		broadcast.Status = constants.TelegramBroadcastStatusCompleted
	}
	return s.store.Update(broadcast)
}

func (s *Service) resolveRecipients(input CreateInput) ([]string, jsonmap.JSON, error) {
	filtersSnapshot := cloneJSONMap(input.Filters)
	if filtersSnapshot == nil {
		filtersSnapshot = jsonmap.JSON{}
	}

	var (
		items []UserItem
		err   error
	)
	switch strings.ToLower(strings.TrimSpace(input.RecipientType)) {
	case constants.TelegramBroadcastRecipientTypeAll:
		items, _, err = s.users.ListTelegramUsers(UserQuery{Page: 1, PageSize: 0})
	case constants.TelegramBroadcastRecipientTypeSpecific:
		uniqueUserIDs := uniqueUintIDs(input.UserIDs)
		if len(uniqueUserIDs) == 0 {
			return nil, nil, ErrNoRecipients
		}
		filtersSnapshot["selected_user_ids"] = uniqueUserIDs
		items, _, err = s.users.ListTelegramUsers(UserQuery{
			Page:     1,
			PageSize: 0,
			UserIDs:  uniqueUserIDs,
		})
	default:
		return nil, nil, ErrInvalid
	}
	if err != nil {
		return nil, nil, err
	}

	chatIDs := make([]string, 0, len(items))
	for _, item := range items {
		chatID := strings.TrimSpace(item.TelegramUserID)
		if chatID == "" {
			continue
		}
		chatIDs = append(chatIDs, chatID)
	}
	chatIDs = dedupeStrings(chatIDs)
	if len(chatIDs) == 0 {
		return nil, nil, ErrNoRecipients
	}
	return chatIDs, filtersSnapshot, nil
}

func (s *Service) dispatchBroadcast(ctx context.Context, broadcastID uint) error {
	if s.dispatcher != nil {
		queued, err := s.dispatcher.DispatchBroadcast(ctx, broadcastID)
		if err != nil {
			return err
		}
		if queued {
			return nil
		}
	}

	go func() {
		_ = s.ProcessBroadcast(context.Background(), broadcastID)
	}()
	return nil
}

func (s *Service) markBroadcastFailed(broadcast *broadcastdomain.Broadcast, reason string) error {
	if broadcast == nil {
		return ErrNotFound
	}
	completedAt := time.Now()
	broadcast.Status = constants.TelegramBroadcastStatusFailed
	broadcast.CompletedAt = &completedAt
	broadcast.LastError = strings.TrimSpace(reason)
	return s.store.Update(broadcast)
}

func cloneJSONMap(source jsonmap.JSON) jsonmap.JSON {
	if source == nil {
		return nil
	}
	result := make(jsonmap.JSON, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func uniqueUintIDs(source []uint) []uint {
	if len(source) == 0 {
		return []uint{}
	}
	result := make([]uint, 0, len(source))
	seen := make(map[uint]struct{}, len(source))
	for _, item := range source {
		if item == 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func dedupeStrings(source []string) []string {
	if len(source) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(source))
	seen := make(map[string]struct{}, len(source))
	for _, item := range source {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
