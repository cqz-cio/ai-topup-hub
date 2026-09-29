package application

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/constants"
)

// UpdateAffiliateProfileStatus 管理端更新返利用户状态
func (s *Service) UpdateAffiliateProfileStatus(profileID uint, rawStatus string) (*affiliatedomain.Profile, error) {
	if profileID == 0 || s.repo == nil {
		return nil, ErrNotFound
	}
	nextStatus := strings.TrimSpace(rawStatus)
	if nextStatus != constants.AffiliateProfileStatusActive && nextStatus != constants.AffiliateProfileStatusDisabled {
		return nil, ErrProfileStatusInvalid
	}

	profile, err := s.repo.GetProfileByID(profileID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(profile.Status) == nextStatus {
		return profile, nil
	}
	if err := s.repo.UpdateProfileStatus(profileID, nextStatus, time.Now()); err != nil {
		return nil, err
	}
	return s.repo.GetProfileByID(profileID)
}

// BatchUpdateAffiliateProfileStatus 管理端批量更新返利用户状态
func (s *Service) BatchUpdateAffiliateProfileStatus(profileIDs []uint, rawStatus string) (int64, error) {
	if s.repo == nil {
		return 0, ErrNotFound
	}
	nextStatus := strings.TrimSpace(rawStatus)
	if nextStatus != constants.AffiliateProfileStatusActive && nextStatus != constants.AffiliateProfileStatusDisabled {
		return 0, ErrProfileStatusInvalid
	}
	normalizedIDs := normalizeAffiliateProfileIDs(profileIDs)
	if len(normalizedIDs) == 0 {
		return 0, nil
	}
	return s.repo.BatchUpdateProfileStatus(normalizedIDs, nextStatus, time.Now())
}

// OpenAffiliate 为用户开通推广返利
func (s *Service) OpenAffiliate(userID uint) (*affiliatedomain.Profile, error) {
	if userID == 0 {
		return nil, ErrUserDisabled
	}
	if s.repo == nil || s.userRepo == nil {
		return nil, ErrNotFound
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return nil, err
	}
	if !setting.Enabled {
		return nil, ErrDisabled
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(user.Status) == constants.UserStatusDisabled {
		return nil, ErrUserDisabled
	}

	existing, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	const maxRetry = 8
	for i := 0; i < maxRetry; i++ {
		code, genErr := generateAffiliateCode()
		if genErr != nil {
			return nil, genErr
		}
		profile := &affiliatedomain.Profile{
			UserID:        userID,
			AffiliateCode: code,
			Status:        constants.AffiliateProfileStatusActive,
		}
		if err := s.repo.CreateProfile(profile); err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return nil, err
		}
		created, err := s.repo.GetProfileByID(profile.ID)
		if err != nil {
			return nil, err
		}
		if created != nil {
			return created, nil
		}
		return profile, nil
	}
	return nil, ErrCodeInvalid
}

func normalizeAffiliateProfileIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return []uint{}
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func generateAffiliateCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var builder strings.Builder
	builder.Grow(affiliateCodeLength)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < affiliateCodeLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}
	return builder.String(), nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
