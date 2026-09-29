package application

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/dujiao-next/internal/constants"
	apicredentialcontract "github.com/dujiao-next/internal/modules/apicredential/contract"
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"
)

// Service API 凭证服务
type Service struct {
	credRepo apicredentialcontract.Repository
}

// NewService 创建凭证服务
func NewService(credRepo apicredentialcontract.Repository) *Service {
	return &Service{credRepo: credRepo}
}

// Apply 用户申请 API 对接权限
func (s *Service) Apply(userID uint) (*apicredentialdomain.ApiCredential, error) {
	existing, err := s.credRepo.GetAnyByUserID(userID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		if existing.DeletedAt != nil {
			if err := resetApiCredentialForReapply(existing); err != nil {
				return nil, err
			}
			if err := s.credRepo.UpdateAny(existing); err != nil {
				return nil, err
			}
			return existing, nil
		}

		switch existing.Status {
		case constants.ApiCredentialStatusPendingReview:
			return nil, apicredentialcontract.ErrPendingExist
		case constants.ApiCredentialStatusApproved:
			return nil, apicredentialcontract.ErrExists
		case constants.ApiCredentialStatusRejected:
			// 允许重新申请，并重置旧审批与凭证痕迹。
			if err := resetApiCredentialForReapply(existing); err != nil {
				return nil, err
			}
			if err := s.credRepo.Update(existing); err != nil {
				return nil, err
			}
			return existing, nil
		case constants.ApiCredentialStatusDisabled:
			return nil, apicredentialcontract.ErrExists
		}
	}

	apiKey, err := generateRandomHex(32)
	if err != nil {
		return nil, err
	}
	cred := &apicredentialdomain.ApiCredential{
		UserID: userID,
		ApiKey: apiKey,
		Status: constants.ApiCredentialStatusPendingReview,
	}
	if err := s.credRepo.Create(cred); err != nil {
		return nil, err
	}
	return cred, nil
}

func resetApiCredentialForReapply(cred *apicredentialdomain.ApiCredential) error {
	apiKey, err := generateRandomHex(32)
	if err != nil {
		return err
	}
	cred.ApiKey = apiKey
	cred.ApiSecret = ""
	cred.Status = constants.ApiCredentialStatusPendingReview
	cred.RejectReason = ""
	cred.ApprovedAt = nil
	cred.LastUsedAt = nil
	cred.IsActive = false
	cred.DeletedAt = nil
	return nil
}

// Approve admin 审核通过
func (s *Service) Approve(id uint) (*apicredentialdomain.ApiCredential, string, error) {
	cred, err := s.credRepo.GetByID(id)
	if err != nil {
		return nil, "", err
	}
	if cred == nil {
		return nil, "", apicredentialcontract.ErrNotFound
	}

	apiKey, err := generateRandomHex(32)
	if err != nil {
		return nil, "", err
	}
	apiSecret, err := generateRandomHex(64)
	if err != nil {
		return nil, "", err
	}

	now := time.Now()
	cred.ApiKey = apiKey
	cred.ApiSecret = apiSecret
	cred.Status = constants.ApiCredentialStatusApproved
	cred.ApprovedAt = &now
	cred.IsActive = true
	cred.RejectReason = ""

	if err := s.credRepo.Update(cred); err != nil {
		return nil, "", err
	}

	return cred, apiSecret, nil
}

// Reject admin 审核拒绝
func (s *Service) Reject(id uint, reason string) error {
	cred, err := s.credRepo.GetByID(id)
	if err != nil {
		return err
	}
	if cred == nil {
		return apicredentialcontract.ErrNotFound
	}

	cred.Status = constants.ApiCredentialStatusRejected
	cred.RejectReason = reason
	return s.credRepo.Update(cred)
}

// SetActive 启用/禁用
func (s *Service) SetActive(id uint, active bool) error {
	cred, err := s.credRepo.GetByID(id)
	if err != nil {
		return err
	}
	if cred == nil {
		return apicredentialcontract.ErrNotFound
	}
	if cred.Status != constants.ApiCredentialStatusApproved {
		return apicredentialcontract.ErrNotApproved
	}

	cred.IsActive = active
	return s.credRepo.Update(cred)
}

// SetActiveByUserID 用户自行启用/禁用
func (s *Service) SetActiveByUserID(userID uint, active bool) error {
	cred, err := s.credRepo.GetByUserID(userID)
	if err != nil {
		return err
	}
	if cred == nil {
		return apicredentialcontract.ErrNotFound
	}
	if cred.Status != constants.ApiCredentialStatusApproved {
		return apicredentialcontract.ErrNotApproved
	}

	cred.IsActive = active
	return s.credRepo.Update(cred)
}

// Regenerate 重新生成 Secret
func (s *Service) Regenerate(id uint) (string, error) {
	cred, err := s.credRepo.GetByID(id)
	if err != nil {
		return "", err
	}
	if cred == nil {
		return "", apicredentialcontract.ErrNotFound
	}
	if cred.Status != constants.ApiCredentialStatusApproved {
		return "", apicredentialcontract.ErrNotApproved
	}

	newSecret, err := generateRandomHex(64)
	if err != nil {
		return "", err
	}

	cred.ApiSecret = newSecret
	if err := s.credRepo.Update(cred); err != nil {
		return "", err
	}
	return newSecret, nil
}

// RegenerateByUserID 用户重新生成 Secret
func (s *Service) RegenerateByUserID(userID uint) (string, error) {
	cred, err := s.credRepo.GetByUserID(userID)
	if err != nil {
		return "", err
	}
	if cred == nil {
		return "", apicredentialcontract.ErrNotFound
	}
	return s.Regenerate(cred.ID)
}

// GetByUserID 获取用户的凭证
func (s *Service) GetByUserID(userID uint) (*apicredentialdomain.ApiCredential, error) {
	return s.credRepo.GetByUserID(userID)
}

// GetByID 根据 ID 获取凭证
func (s *Service) GetByID(id uint) (*apicredentialdomain.ApiCredential, error) {
	return s.credRepo.GetByID(id)
}

// List 列表查询
func (s *Service) List(filter apicredentialcontract.ListFilter) ([]apicredentialdomain.ApiCredential, int64, error) {
	return s.credRepo.List(filter)
}

// Delete 删除凭证
func (s *Service) Delete(id uint) error {
	return s.credRepo.Delete(id)
}

func generateRandomHex(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
