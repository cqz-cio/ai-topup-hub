package application

import (
	"time"

	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"
)

func (s *Service) GetJob(id uint) (*reconciliationdomain.Job, error) {
	return s.jobs.GetByID(id)
}

func (s *Service) ListJobs(filter reconciliationcontract.JobListFilter) ([]reconciliationdomain.Job, int64, error) {
	return s.jobs.List(filter)
}

func (s *Service) GetJobItems(jobID uint, page, pageSize int) ([]reconciliationdomain.Item, int64, error) {
	return s.items.ListByJobID(jobID, page, pageSize)
}

func (s *Service) ResolveItem(itemID, adminID uint, remark string) error {
	item, err := s.items.GetByID(itemID)
	if err != nil || item == nil {
		return reconciliationcontract.ErrItemNotFound
	}
	now := time.Now()
	item.Resolved, item.ResolvedBy, item.ResolvedAt, item.Remark = true, &adminID, &now, remark
	return s.items.Update(item)
}
