package gormstore

import (
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"

	"gorm.io/gorm"
)

type JobStore struct {
	db *gorm.DB
}

var _ reconciliationcontract.JobRepository = (*JobStore)(nil)

func NewJobStore(db *gorm.DB) *JobStore { return &JobStore{db: db} }

func (s *JobStore) Create(job *reconciliationdomain.Job) error {
	return s.db.Create(job).Error
}

func (s *JobStore) GetByID(id uint) (*reconciliationdomain.Job, error) {
	var job reconciliationdomain.Job
	if err := s.db.Preload("Connection", "deleted_at IS NULL").First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *JobStore) Update(job *reconciliationdomain.Job) error {
	return s.db.Save(job).Error
}

func (s *JobStore) List(filter reconciliationcontract.JobListFilter) ([]reconciliationdomain.Job, int64, error) {
	var jobs []reconciliationdomain.Job
	var total int64
	query := s.db.Model(&reconciliationdomain.Job{})
	if filter.ConnectionID > 0 {
		query = query.Where("connection_id = ?", filter.ConnectionID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if err := query.Preload("Connection", "deleted_at IS NULL").Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}
