package application

import (
	"context"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"
)

type Options struct {
	Jobs          reconciliationcontract.JobRepository
	Items         reconciliationcontract.ItemRepository
	Procurements  reconciliationcontract.ProcurementReader
	Upstream      reconciliationcontract.UpstreamOrderProvider
	Queue         reconciliationcontract.Enqueuer
	Notifications reconciliationcontract.MismatchNotifier
}

type Service struct {
	jobs          reconciliationcontract.JobRepository
	items         reconciliationcontract.ItemRepository
	procurements  reconciliationcontract.ProcurementReader
	upstream      reconciliationcontract.UpstreamOrderProvider
	queue         reconciliationcontract.Enqueuer
	notifications reconciliationcontract.MismatchNotifier
}

var _ reconciliationcontract.UseCase = (*Service)(nil)

func NewService(options Options) *Service {
	if options.Jobs == nil || options.Items == nil || options.Procurements == nil || options.Upstream == nil {
		panic("reconciliation service: required dependency is nil")
	}
	return &Service{
		jobs: options.Jobs, items: options.Items, procurements: options.Procurements,
		upstream: options.Upstream, queue: options.Queue, notifications: options.Notifications,
	}
}

func (s *Service) CreateAndEnqueue(input reconciliationcontract.RunInput) (*reconciliationdomain.Job, error) {
	job := &reconciliationdomain.Job{
		ConnectionID:   input.ConnectionID,
		Type:           input.Type,
		Status:         constants.ReconciliationJobStatusPending,
		TimeRangeStart: input.TimeRangeStart,
		TimeRangeEnd:   input.TimeRangeEnd,
	}
	if err := s.jobs.Create(job); err != nil {
		return nil, fmt.Errorf("create reconciliation job: %w", err)
	}
	if s.queue != nil {
		if err := s.queue.Enqueue(job.ID); err != nil {
			logger.Warnw("reconciliation_enqueue_failed", "job_id", job.ID, "error", err)
		}
	}
	return job, nil
}

func (s *Service) Execute(ctx context.Context, jobID uint) error {
	job, err := s.jobs.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("get reconciliation job: %w", err)
	}
	if job == nil {
		return reconciliationcontract.ErrJobNotFound
	}
	if job.Status == constants.ReconciliationJobStatusRunning {
		return reconciliationcontract.ErrJobRunning
	}
	if job.Status == constants.ReconciliationJobStatusCompleted {
		return nil
	}

	now := time.Now()
	job.Status, job.StartedAt = constants.ReconciliationJobStatusRunning, &now
	if err := s.jobs.Update(job); err != nil {
		return fmt.Errorf("update job status to running: %w", err)
	}
	if err := s.execute(ctx, job); err != nil {
		finishedAt := time.Now()
		job.Status, job.FinishedAt = constants.ReconciliationJobStatusFailed, &finishedAt
		job.ResultJSON = marshalResult(map[string]string{"error": err.Error()})
		_ = s.jobs.Update(job)
		return fmt.Errorf("execute reconciliation: %w", err)
	}

	finishedAt := time.Now()
	job.Status, job.FinishedAt = constants.ReconciliationJobStatusCompleted, &finishedAt
	_ = s.jobs.Update(job)
	if job.MismatchedCount > 0 && s.notifications != nil {
		_ = s.notifications.NotifyMismatch(job)
	}
	return nil
}
