package application

import (
	"strings"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
)

// List 获取礼品卡列表。
func (s *Service) List(input ListInput) ([]giftcarddomain.GiftCard, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, giftcardcontract.ErrFetchFailed
	}
	cards, total, err := s.repo.List(giftcardcontract.ListFilter{
		Code:           strings.TrimSpace(strings.ToUpper(input.Code)),
		Status:         strings.TrimSpace(strings.ToLower(input.Status)),
		BatchNo:        strings.TrimSpace(strings.ToUpper(input.BatchNo)),
		RedeemedUserID: input.RedeemedUserID,
		CreatedFrom:    input.CreatedFrom,
		CreatedTo:      input.CreatedTo,
		RedeemedFrom:   input.RedeemedFrom,
		RedeemedTo:     input.RedeemedTo,
		ExpiresFrom:    input.ExpiresFrom,
		ExpiresTo:      input.ExpiresTo,
		Page:           input.Page,
		PageSize:       input.PageSize,
	})
	if err != nil {
		return nil, 0, giftcardcontract.ErrFetchFailed
	}
	return cards, total, nil
}

// Update 更新礼品卡。
func (s *Service) Update(id uint, input UpdateInput) (*giftcarddomain.GiftCard, error) {
	if s == nil || s.repo == nil || id == 0 {
		return nil, giftcardcontract.ErrInvalid
	}
	card, err := s.repo.GetByID(id)
	if err != nil {
		return nil, giftcardcontract.ErrFetchFailed
	}
	if card == nil {
		return nil, giftcardcontract.ErrNotFound
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, giftcardcontract.ErrInvalid
		}
		card.Name = name
	}
	if input.Status != nil {
		status := strings.TrimSpace(strings.ToLower(*input.Status))
		switch status {
		case giftcarddomain.GiftCardStatusActive, giftcarddomain.GiftCardStatusDisabled:
			if card.Status == giftcarddomain.GiftCardStatusRedeemed {
				return nil, giftcardcontract.ErrInvalid
			}
			card.Status = status
		default:
			return nil, giftcardcontract.ErrInvalid
		}
	}
	if input.ClearExpiresAt {
		card.ExpiresAt = nil
	} else if input.ExpiresAt != nil {
		normalized := normalizeExpireAt(input.ExpiresAt)
		if normalized != nil && normalized.Before(time.Now()) {
			return nil, giftcardcontract.ErrInvalid
		}
		card.ExpiresAt = normalized
	}
	card.UpdatedAt = time.Now()
	if err := s.repo.Update(card); err != nil {
		return nil, giftcardcontract.ErrUpdateFailed
	}
	return card, nil
}

// Delete 删除礼品卡。
func (s *Service) Delete(id uint) error {
	if s == nil || s.repo == nil || id == 0 {
		return giftcardcontract.ErrInvalid
	}
	card, err := s.repo.GetByID(id)
	if err != nil {
		return giftcardcontract.ErrFetchFailed
	}
	if card == nil {
		return giftcardcontract.ErrNotFound
	}
	if card.Status == giftcarddomain.GiftCardStatusRedeemed {
		return giftcardcontract.ErrInvalid
	}
	if err := s.repo.Delete(id); err != nil {
		return giftcardcontract.ErrDeleteFailed
	}
	return nil
}

// BatchUpdateStatus 批量更新礼品卡状态。
func (s *Service) BatchUpdateStatus(ids []uint, status string) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, giftcardcontract.ErrInvalid
	}
	normalizedIDs := normalizeIDs(ids)
	if len(normalizedIDs) == 0 {
		return 0, giftcardcontract.ErrInvalid
	}
	normalizedStatus := strings.TrimSpace(strings.ToLower(status))
	switch normalizedStatus {
	case giftcarddomain.GiftCardStatusActive, giftcarddomain.GiftCardStatusDisabled:
	default:
		return 0, giftcardcontract.ErrInvalid
	}
	rows, err := s.repo.BatchUpdateStatus(normalizedIDs, normalizedStatus, time.Now())
	if err != nil {
		return 0, giftcardcontract.ErrUpdateFailed
	}
	return rows, nil
}

// ResolveRedeemedUsers 批量解析礼品卡兑换用户。
func (s *Service) ResolveRedeemedUsers(cards []giftcarddomain.GiftCard) (map[uint]userdomain.User, error) {
	result := make(map[uint]userdomain.User)
	if s == nil || s.users == nil || len(cards) == 0 {
		return result, nil
	}
	userIDs := make([]uint, 0, len(cards))
	seen := make(map[uint]struct{})
	for _, card := range cards {
		if card.RedeemedUserID == nil || *card.RedeemedUserID == 0 {
			continue
		}
		if _, ok := seen[*card.RedeemedUserID]; ok {
			continue
		}
		seen[*card.RedeemedUserID] = struct{}{}
		userIDs = append(userIDs, *card.RedeemedUserID)
	}
	if len(userIDs) == 0 {
		return result, nil
	}
	users, err := s.users.ListByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		result[user.ID] = user
	}
	return result, nil
}
