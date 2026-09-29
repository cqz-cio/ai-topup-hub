package channelclientapp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/crypto"
	channelclientcontract "github.com/dujiao-next/internal/modules/channelclient/contract"
	channelclientdomain "github.com/dujiao-next/internal/modules/channelclient/domain"
	"github.com/dujiao-next/internal/upstream"
)

// Service 渠道客户端业务服务。
type Service struct {
	store  channelclientcontract.Store
	encKey []byte // AES-256 密钥
}

// NewService 创建渠道客户端服务。
func NewService(store channelclientcontract.Store, appSecretKey string) *Service {
	return &Service{
		store:  store,
		encKey: crypto.DeriveKey(appSecretKey),
	}
}

func maskBotToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

// CreateChannelClient 创建渠道客户端
func (s *Service) CreateChannelClient(name, channelType, description, botToken, callbackURL string) (*ClientDetail, error) {
	// 生成随机 key (32 bytes = 64 hex chars)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("generate channel key: %w", err)
	}
	channelKey := hex.EncodeToString(keyBytes)

	// 生成随机 secret (32 bytes = 64 hex chars)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("generate channel secret: %w", err)
	}
	plainSecret := hex.EncodeToString(secretBytes)

	// 加密 secret 存储
	encryptedSecret, err := crypto.Encrypt(s.encKey, plainSecret)
	if err != nil {
		return nil, fmt.Errorf("encrypt channel secret: %w", err)
	}

	client := &channelclientdomain.Client{
		Name:          name,
		ChannelType:   channelType,
		ChannelKey:    channelKey,
		ChannelSecret: encryptedSecret,
		CallbackURL:   callbackURL,
		Status:        1,
		Description:   description,
	}

	// 加密 bot_token（如果提供）
	if botToken != "" {
		encryptedToken, err := crypto.Encrypt(s.encKey, botToken)
		if err != nil {
			return nil, fmt.Errorf("encrypt bot token: %w", err)
		}
		client.BotToken = encryptedToken
	}

	if err := s.store.Create(client); err != nil {
		return nil, err
	}

	return &ClientDetail{
		ID:            client.ID,
		Name:          client.Name,
		ChannelType:   client.ChannelType,
		ChannelKey:    client.ChannelKey,
		ChannelSecret: plainSecret,
		BotToken:      maskBotToken(botToken),
		BotTokenSet:   botToken != "",
		CallbackURL:   client.CallbackURL,
		Description:   client.Description,
		Status:        client.Status,
	}, nil
}

// GetChannelClient 获取渠道客户端
func (s *Service) GetChannelClient(id uint) (*channelclientdomain.Client, error) {
	client, err := s.store.FindByID(id)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrNotFound
	}
	return client, nil
}

// ListChannelClients 列出所有渠道客户端
func (s *Service) ListChannelClients() ([]channelclientdomain.Client, error) {
	return s.store.FindAll()
}

// GetChannelClientDetail 获取渠道客户端详情（含解密 secret）
func (s *Service) GetChannelClientDetail(id uint) (*ClientDetail, error) {
	client, err := s.store.FindByID(id)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrNotFound
	}

	plainSecret, err := crypto.Decrypt(s.encKey, client.ChannelSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt channel secret: %w", err)
	}

	resp := &ClientDetail{
		ID:            client.ID,
		Name:          client.Name,
		ChannelType:   client.ChannelType,
		ChannelKey:    client.ChannelKey,
		ChannelSecret: plainSecret,
		BotTokenSet:   client.BotToken != "",
		CallbackURL:   client.CallbackURL,
		Description:   client.Description,
		Status:        client.Status,
	}

	if client.BotToken != "" {
		plainToken, err := crypto.Decrypt(s.encKey, client.BotToken)
		if err == nil {
			resp.BotToken = maskBotToken(plainToken)
		}
	}

	return resp, nil
}

// ListChannelClientDetails 列出所有渠道客户端（含解密 secret）
func (s *Service) ListChannelClientDetails() ([]ClientDetail, error) {
	clients, err := s.store.FindAll()
	if err != nil {
		return nil, err
	}
	result := make([]ClientDetail, 0, len(clients))
	for _, c := range clients {
		plainSecret, decErr := crypto.Decrypt(s.encKey, c.ChannelSecret)
		if decErr != nil {
			plainSecret = ""
		}
		resp := ClientDetail{
			ID:            c.ID,
			Name:          c.Name,
			ChannelType:   c.ChannelType,
			ChannelKey:    c.ChannelKey,
			ChannelSecret: plainSecret,
			BotTokenSet:   c.BotToken != "",
			CallbackURL:   c.CallbackURL,
			Description:   c.Description,
			Status:        c.Status,
		}
		if c.BotToken != "" {
			plainToken, decErr := crypto.Decrypt(s.encKey, c.BotToken)
			if decErr == nil {
				resp.BotToken = maskBotToken(plainToken)
			}
		}
		result = append(result, resp)
	}
	return result, nil
}

// ResetChannelClientSecret 重置渠道客户端 Secret
func (s *Service) ResetChannelClientSecret(id uint) (*ClientDetail, error) {
	client, err := s.store.FindByID(id)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrNotFound
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("generate channel secret: %w", err)
	}
	plainSecret := hex.EncodeToString(secretBytes)

	encryptedSecret, err := crypto.Encrypt(s.encKey, plainSecret)
	if err != nil {
		return nil, fmt.Errorf("encrypt channel secret: %w", err)
	}

	client.ChannelSecret = encryptedSecret
	if err := s.store.Update(client); err != nil {
		return nil, err
	}

	resp := &ClientDetail{
		ID:            client.ID,
		Name:          client.Name,
		ChannelType:   client.ChannelType,
		ChannelKey:    client.ChannelKey,
		ChannelSecret: plainSecret,
		BotTokenSet:   client.BotToken != "",
		CallbackURL:   client.CallbackURL,
		Description:   client.Description,
		Status:        client.Status,
	}
	if client.BotToken != "" {
		plainToken, decErr := crypto.Decrypt(s.encKey, client.BotToken)
		if decErr == nil {
			resp.BotToken = maskBotToken(plainToken)
		}
	}
	return resp, nil
}

// DeleteChannelClient 删除渠道客户端（软删除）
func (s *Service) DeleteChannelClient(id uint) error {
	client, err := s.store.FindByID(id)
	if err != nil {
		return err
	}
	if client == nil {
		return ErrNotFound
	}
	return s.store.Delete(client.ID, time.Now())
}

// UpdateChannelClientStatus 更新渠道客户端状态
func (s *Service) UpdateChannelClientStatus(id uint, status int) error {
	client, err := s.store.FindByID(id)
	if err != nil {
		return err
	}
	if client == nil {
		return ErrNotFound
	}
	client.Status = status
	return s.store.Update(client)
}

// UpdateChannelClient 更新渠道客户端信息（名称、描述、bot_token）
func (s *Service) UpdateChannelClient(id uint, name, description string, botToken *string, callbackURL *string) (*ClientDetail, error) {
	client, err := s.store.FindByID(id)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrNotFound
	}

	if name != "" {
		client.Name = name
	}
	client.Description = description
	if callbackURL != nil {
		client.CallbackURL = *callbackURL
	}

	// botToken 为 nil 表示不修改；非 nil 则更新（空字符串表示清空）
	if botToken != nil {
		if *botToken != "" {
			encryptedToken, err := crypto.Encrypt(s.encKey, *botToken)
			if err != nil {
				return nil, fmt.Errorf("encrypt bot token: %w", err)
			}
			client.BotToken = encryptedToken
		} else {
			client.BotToken = ""
		}
	}

	if err := s.store.Update(client); err != nil {
		return nil, err
	}

	plainSecret, err := crypto.Decrypt(s.encKey, client.ChannelSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt channel secret: %w", err)
	}

	resp := &ClientDetail{
		ID:            client.ID,
		Name:          client.Name,
		ChannelType:   client.ChannelType,
		ChannelKey:    client.ChannelKey,
		ChannelSecret: plainSecret,
		BotTokenSet:   client.BotToken != "",
		CallbackURL:   client.CallbackURL,
		Description:   client.Description,
		Status:        client.Status,
	}
	if client.BotToken != "" {
		plainToken, decErr := crypto.Decrypt(s.encKey, client.BotToken)
		if decErr == nil {
			resp.BotToken = maskBotToken(plainToken)
		}
	}
	return resp, nil
}

// DecryptBotToken 解密渠道客户端的 Bot Token（供 Channel API 使用）
func (s *Service) DecryptBotToken(client *channelclientdomain.Client) (string, error) {
	if client.BotToken == "" {
		return "", nil
	}
	return crypto.Decrypt(s.encKey, client.BotToken)
}

// DecryptChannelSecret 解密渠道客户端的 ChannelSecret
func (s *Service) DecryptChannelSecret(client *channelclientdomain.Client) (string, error) {
	if client.ChannelSecret == "" {
		return "", nil
	}
	return crypto.Decrypt(s.encKey, client.ChannelSecret)
}

// VerifyChannelSignature 验证渠道签名
// 复用 upstream/signer.go 的 HMAC-SHA256 签名算法
func (s *Service) VerifyChannelSignature(key, signature string, timestamp int64, method, path string, body []byte) (*channelclientdomain.Client, error) {
	// 验证时间戳
	if !upstream.IsTimestampValid(timestamp) {
		return nil, ErrTimestampExpired
	}

	// 查找客户端
	client, err := s.store.FindByChannelKey(key)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrNotFound
	}
	if client.Status != 1 {
		return nil, ErrDisabled
	}

	// 解密 secret
	plainSecret, err := crypto.Decrypt(s.encKey, client.ChannelSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt channel secret: %w", err)
	}

	// 验证签名（复用 upstream.Verify）
	if !upstream.Verify(plainSecret, method, path, signature, timestamp, body) {
		return nil, ErrSignatureInvalid
	}

	return client, nil
}

// MarkUsed records successful channel authentication without exposing the
// persistence store to the router.
func (s *Service) MarkUsed(id uint, usedAt time.Time) error {
	return s.store.UpdateLastUsed(id, usedAt)
}

// ResolveBotTokenByType returns the active channel's decrypted bot token.
func (s *Service) ResolveBotTokenByType(channelType string) (string, error) {
	client, err := s.store.FindActiveByChannelType(strings.TrimSpace(channelType))
	if err != nil {
		return "", err
	}
	if client == nil {
		return "", ErrNotFound
	}
	token, err := s.DecryptBotToken(client)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", ErrNotFound
	}
	return strings.TrimSpace(token), nil
}

// DecryptBotTokenByClientID returns one configured bot token to the channel
// transport without leaking the persisted client entity.
func (s *Service) DecryptBotTokenByClientID(clientID uint) (string, error) {
	client, err := s.GetChannelClient(clientID)
	if err != nil {
		return "", err
	}
	return s.DecryptBotToken(client)
}

// GetActiveEndpoint resolves the active callback endpoint and decrypts its
// signing secret for background integrations.
func (s *Service) GetActiveEndpoint(channelType string) (*ActiveEndpoint, error) {
	client, err := s.store.FindActiveByChannelType(strings.TrimSpace(channelType))
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrNotFound
	}
	secret, err := s.DecryptChannelSecret(client)
	if err != nil {
		return nil, err
	}
	return &ActiveEndpoint{
		ClientID:      client.ID,
		ChannelKey:    client.ChannelKey,
		CallbackURL:   client.CallbackURL,
		ChannelSecret: secret,
	}, nil
}
