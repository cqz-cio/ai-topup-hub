package application

import (
	"strings"
	"time"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// ApplyWithdraw 用户提交提现申请
func (s *Service) ApplyWithdraw(userID uint, input WithdrawApplyInput) (*affiliatedomain.WithdrawRequest, error) {
	if userID == 0 || s.repo == nil {
		return nil, ErrNotOpened
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return nil, err
	}
	if !setting.Enabled {
		return nil, ErrDisabled
	}

	amount := input.Amount.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrWithdrawAmountInvalid
	}
	minAmount := decimal.NewFromFloat(setting.MinWithdrawAmount).Round(2)
	if amount.LessThan(minAmount) {
		return nil, ErrWithdrawAmountInvalid
	}
	channel := strings.TrimSpace(input.Channel)
	account := strings.TrimSpace(input.Account)
	if channel == "" || account == "" {
		return nil, ErrWithdrawChannelInvalid
	}
	if len(setting.WithdrawChannels) > 0 && !containsWithdrawChannel(setting.WithdrawChannels, channel) {
		return nil, ErrWithdrawChannelInvalid
	}
	if err := s.ConfirmDueCommissions(time.Now()); err != nil {
		return nil, err
	}

	var createdID uint
	err = s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		profile, err := repoTx.GetProfileByUserID(userID)
		if err != nil {
			return err
		}
		if profile == nil {
			return ErrNotOpened
		}
		if strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
			return ErrNotOpened
		}

		commissions, err := repoTx.ListAvailableCommissionsForUpdate(profile.ID)
		if err != nil {
			return err
		}

		remaining := amount
		selectedIDs := make([]uint, 0)
		now := time.Now()
		for _, commission := range commissions {
			if remaining.LessThanOrEqual(decimal.Zero) {
				break
			}
			rowAmount := commission.CommissionAmount.Decimal.Round(2)
			if rowAmount.LessThanOrEqual(decimal.Zero) {
				continue
			}
			if rowAmount.LessThanOrEqual(remaining) {
				selectedIDs = append(selectedIDs, commission.ID)
				remaining = remaining.Sub(rowAmount).Round(2)
				continue
			}

			// 最后一条记录金额大于申请剩余金额时，拆分记录避免超额冻结。
			boundAmount := remaining.Round(2)
			remainAmount := rowAmount.Sub(boundAmount).Round(2)
			commission.CommissionAmount = money.FromDecimal(boundAmount)
			commission.UpdatedAt = now
			if err := repoTx.UpdateCommission(&commission); err != nil {
				return err
			}

			remainCommission := commission
			remainCommission.ID = 0
			remainCommission.CommissionType = buildSplitCommissionType(commission.ID)
			remainCommission.CommissionAmount = money.FromDecimal(remainAmount)
			remainCommission.WithdrawRequestID = nil
			remainCommission.Status = constants.AffiliateCommissionStatusAvailable
			remainCommission.InvalidReason = ""
			remainCommission.CreatedAt = now
			remainCommission.UpdatedAt = now
			if err := repoTx.CreateCommission(&remainCommission); err != nil {
				return err
			}

			selectedIDs = append(selectedIDs, commission.ID)
			remaining = decimal.Zero
			break
		}
		if remaining.GreaterThan(decimal.Zero) {
			return ErrWithdrawInsufficient
		}

		req := &affiliatedomain.WithdrawRequest{
			AffiliateProfileID: profile.ID,
			Amount:             money.FromDecimal(amount),
			Channel:            channel,
			Account:            account,
			Status:             constants.AffiliateWithdrawStatusPendingReview,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := repoTx.CreateWithdraw(req); err != nil {
			return err
		}
		if err := repoTx.BatchUpdateCommissions(selectedIDs, map[string]interface{}{
			"withdraw_request_id": req.ID,
			"updated_at":          now,
		}); err != nil {
			return err
		}
		createdID = req.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithdrawByID(createdID)
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
