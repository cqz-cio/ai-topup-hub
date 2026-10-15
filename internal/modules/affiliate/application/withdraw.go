package application

import (
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"strings"
	"time"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/constants"
)

// ApplyWithdraw 用户提交提现申请
func (s *Service) ApplyWithdraw(userID uint, input WithdrawApplyInput) (*affiliatedomain.WithdrawRequest, error) {
	return nil, walletcontract.ErrUnifiedWallet
}

// ReviewWithdraw 管理端审核提现申请
func (s *Service) ReviewWithdraw(adminID, withdrawID uint, action, rejectReason string) (*affiliatedomain.WithdrawRequest, error) {
	if withdrawID == 0 || s.repo == nil {
		return nil, ErrNotFound
	}
	act := strings.ToLower(strings.TrimSpace(action))
	if act != constants.AffiliateWithdrawActionReject && act != constants.AffiliateWithdrawActionPay {
		return nil, ErrWithdrawStatusInvalid
	}
	rejectReason = strings.TrimSpace(rejectReason)

	err := s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		req, err := repoTx.GetWithdrawByIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		if req == nil {
			return ErrNotFound
		}
		if req.Status != constants.AffiliateWithdrawStatusPendingReview {
			return ErrWithdrawStatusInvalid
		}

		commissions, err := repoTx.ListCommissionsByWithdrawIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		ids := make([]uint, 0, len(commissions))
		for _, commission := range commissions {
			ids = append(ids, commission.ID)
		}

		now := time.Now()
		req.ProcessedBy = &adminID
		req.ProcessedAt = &now
		req.UpdatedAt = now
		if act == constants.AffiliateWithdrawActionReject {
			req.Status = constants.AffiliateWithdrawStatusRejected
			req.RejectReason = rejectReason
			if err := repoTx.BatchUpdateCommissions(ids, map[string]interface{}{
				"withdraw_request_id": nil,
				"updated_at":          now,
			}); err != nil {
				return err
			}
		} else {
			req.Status = constants.AffiliateWithdrawStatusPaid
			req.RejectReason = ""
			if err := repoTx.BatchUpdateCommissions(ids, map[string]interface{}{
				"status":     constants.AffiliateCommissionStatusWithdrawn,
				"updated_at": now,
			}); err != nil {
				return err
			}
		}
		return repoTx.UpdateWithdraw(req)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithdrawByID(withdrawID)
}

func containsWithdrawChannel(channels []string, channel string) bool {
	target := strings.ToLower(strings.TrimSpace(channel))
	if target == "" {
		return false
	}
	for _, item := range channels {
		if strings.ToLower(strings.TrimSpace(item)) == target {
			return true
		}
	}
	return false
}
