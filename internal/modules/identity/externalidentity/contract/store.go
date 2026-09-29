package externalidentitycontract

import (
	"time"

	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
)

type TelegramUserFilter struct {
	Page             int
	PageSize         int
	Keyword          string
	DisplayName      string
	TelegramUsername string
	TelegramUserID   string
	CreatedFrom      *time.Time
	CreatedTo        *time.Time
	UserIDs          []uint
}

type TelegramUser struct {
	UserID           uint
	DisplayName      string
	UserEmail        string
	TelegramUsername string
	TelegramUserID   string
	BoundAt          time.Time
	UserCreatedAt    time.Time
}

type Store interface {
	GetByProviderUserID(provider, providerUserID string) (*externalidentitydomain.Identity, error)
	GetByUserProvider(userID uint, provider string) (*externalidentitydomain.Identity, error)
	ListByUserID(userID uint) ([]externalidentitydomain.Identity, error)
	ListTelegramUsers(filter TelegramUserFilter) ([]TelegramUser, int64, error)
	Create(*externalidentitydomain.Identity) error
	Update(*externalidentitydomain.Identity) error
	DeleteByID(id uint) error
}
