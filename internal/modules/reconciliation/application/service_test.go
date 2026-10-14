package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

type jobRepositoryStub struct {
	job     *reconciliationdomain.Job
	updates []string
}

func (s *jobRepositoryStub) Create(job *reconciliationdomain.Job) error {
	job.ID = 99
	s.job = job
	return nil
}
func (s *jobRepositoryStub) GetByID(uint) (*reconciliationdomain.Job, error) { return s.job, nil }
func (s *jobRepositoryStub) Update(job *reconciliationdomain.Job) error {
	s.updates = append(s.updates, job.Status)
	return nil
}
func (s *jobRepositoryStub) List(reconciliationcontract.JobListFilter) ([]reconciliationdomain.Job, int64, error) {
	return nil, 0, nil
}

type itemRepositoryStub struct {
	created []reconciliationdomain.Item
}

func (s *itemRepositoryStub) BatchCreate(items []reconciliationdomain.Item) error {
	s.created = append(s.created, items...)
	return nil
}
func (s *itemRepositoryStub) GetByID(uint) (*reconciliationdomain.Item, error) { return nil, nil }
func (s *itemRepositoryStub) Update(*reconciliationdomain.Item) error          { return nil }
func (s *itemRepositoryStub) ListByJobID(uint, int, int) ([]reconciliationdomain.Item, int64, error) {
	return nil, 0, nil
}

type procurementReaderStub struct {
	orders []reconciliationcontract.ProcurementOrder
}

func (s procurementReaderStub) ListByConnectionAndTimeRange(uint, time.Time, time.Time) ([]reconciliationcontract.ProcurementOrder, error) {
	return s.orders, nil
}

type upstreamProviderStub struct {
	openCalls int
	reader    reconciliationcontract.UpstreamOrderReader
}

func (s *upstreamProviderStub) Open(uint) (reconciliationcontract.UpstreamOrderReader, error) {
	s.openCalls++
	return s.reader, nil
}

type upstreamReaderStub struct {
	calls      int
	err        error
	references []string
	details    map[uint]reconciliationcontract.UpstreamOrder
}

func (s *upstreamReaderStub) Get(_ context.Context, orderID uint, orderNo string) (*reconciliationcontract.UpstreamOrder, error) {
	s.calls++
	s.references = append(s.references, orderNo)
	detail := s.details[orderID]
	return &detail, s.err
}

type enqueuerStub struct {
	jobIDs []uint
	err    error
}

func (s *enqueuerStub) Enqueue(jobID uint) error {
	s.jobIDs = append(s.jobIDs, jobID)
	return s.err
}

type notifierStub struct{ calls int }

func (s *notifierStub) NotifyMismatch(*reconciliationdomain.Job) error {
	s.calls++
	return nil
}

func TestCreateAndEnqueueUsesCreatedJobIdentity(t *testing.T) {
	jobs := &jobRepositoryStub{}
	queue := &enqueuerStub{}
	svc := NewService(Options{
		Jobs: jobs, Items: &itemRepositoryStub{}, Procurements: procurementReaderStub{},
		Upstream: &upstreamProviderStub{reader: &upstreamReaderStub{}}, Queue: queue,
	})

	job, err := svc.CreateAndEnqueue(reconciliationcontract.RunInput{ConnectionID: 7, Type: constants.ReconciliationTypeFull})
	if err != nil {
		t.Fatalf("create and enqueue: %v", err)
	}
	if job.ID != 99 || len(queue.jobIDs) != 1 || queue.jobIDs[0] != 99 {
		t.Fatalf("expected persisted job id to be queued, job=%+v queued=%v", job, queue.jobIDs)
	}
}

func TestCreateAndEnqueueReportsUnavailableQueue(t *testing.T) {
	for _, absent := range []bool{true, false} {
		t.Run(fmt.Sprintf("absent_%v", absent), func(t *testing.T) {
			jobs := &jobRepositoryStub{}
			var queue reconciliationcontract.Enqueuer
			if !absent {
				queue = &enqueuerStub{err: errors.New("unavailable")}
			}
			svc := NewService(Options{Jobs: jobs, Items: &itemRepositoryStub{}, Procurements: procurementReaderStub{}, Upstream: &upstreamProviderStub{}, Queue: queue})
			job, err := svc.CreateAndEnqueue(reconciliationcontract.RunInput{ConnectionID: 1})
			if !errors.Is(err, reconciliationcontract.ErrQueueUnavailable) || job != nil {
				t.Fatalf("job=%+v err=%v", job, err)
			}
			if absent && jobs.job != nil {
				t.Fatal("missing queue created an unserviceable job")
			}
			if !absent && (jobs.job.Status != constants.ReconciliationJobStatusFailed || jobs.job.FinishedAt == nil) {
				t.Fatalf("enqueue failure left a pending job: %+v", jobs.job)
			}
		})
	}
}

func TestExecuteOpensUpstreamOnceAndPersistsMismatch(t *testing.T) {
	jobs := &jobRepositoryStub{job: &reconciliationdomain.Job{
		ID: 1, ConnectionID: 7, Type: constants.ReconciliationTypeFull,
		Status: constants.ReconciliationJobStatusPending,
	}}
	items := &itemRepositoryStub{}
	upstreamOrders := &upstreamReaderStub{details: map[uint]reconciliationcontract.UpstreamOrder{
		11: {Status: "completed", Amount: "10.00"},
		12: {Status: "failed", Amount: "12.00"},
	}}
	upstreamProvider := &upstreamProviderStub{reader: upstreamOrders}
	notifier := &notifierStub{}
	svc := NewService(Options{
		Jobs:  jobs,
		Items: items,
		Procurements: procurementReaderStub{orders: []reconciliationcontract.ProcurementOrder{
			{ID: 21, UpstreamOrderID: 11, Status: constants.ProcurementStatusCompleted, UpstreamAmount: money.FromDecimal(decimal.NewFromInt(10))},
			{ID: 22, UpstreamOrderID: 12, Status: constants.ProcurementStatusCompleted, UpstreamAmount: money.FromDecimal(decimal.NewFromInt(10))},
		}},
		Upstream: upstreamProvider, Notifications: notifier,
	})

	if err := svc.Execute(context.Background(), 1); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if upstreamProvider.openCalls != 1 || upstreamOrders.calls != 2 {
		t.Fatalf("expected one upstream session and two reads, open=%d get=%d", upstreamProvider.openCalls, upstreamOrders.calls)
	}
	if len(items.created) != 1 || items.created[0].ProcurementOrderID != 22 || items.created[0].MismatchType != constants.MismatchTypeBoth {
		t.Fatalf("unexpected mismatches: %+v", items.created)
	}
	if jobs.job.Status != constants.ReconciliationJobStatusCompleted || jobs.job.TotalCount != 2 || jobs.job.MatchedCount != 1 || jobs.job.MismatchedCount != 1 {
		t.Fatalf("unexpected completed job: %+v", jobs.job)
	}
	if len(jobs.updates) != 2 || jobs.updates[0] != constants.ReconciliationJobStatusRunning || jobs.updates[1] != constants.ReconciliationJobStatusCompleted {
		t.Fatalf("unexpected status transitions: %v", jobs.updates)
	}
	if notifier.calls != 1 {
		t.Fatalf("expected one mismatch notification, got %d", notifier.calls)
	}
}

func TestExecuteStringReferencesAndIncompleteResults(t *testing.T) {
	for _, tc := range []struct {
		name, kind, amount string
		queryErr           error
		incomplete         bool
	}{
		{"string reference", constants.ReconciliationTypeFull, "10.00", nil, false},
		{"status only without amount", constants.ReconciliationTypeStatus, "", nil, false},
		{"missing amount", constants.ReconciliationTypeFull, "", nil, true},
		{"invalid amount", constants.ReconciliationTypeAmount, "unknown", nil, true},
		{"query failed", constants.ReconciliationTypeFull, "", errors.New("offline"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs := &jobRepositoryStub{job: &reconciliationdomain.Job{ID: 1, ConnectionID: 7, Type: tc.kind, Status: constants.ReconciliationJobStatusPending}}
			reader := &upstreamReaderStub{err: tc.queryErr, details: map[uint]reconciliationcontract.UpstreamOrder{0: {Status: "delivered", Amount: tc.amount}}}
			svc := NewService(Options{Jobs: jobs, Items: &itemRepositoryStub{}, Upstream: &upstreamProviderStub{reader: reader}, Procurements: procurementReaderStub{orders: []reconciliationcontract.ProcurementOrder{{ID: 1, UpstreamOrderNo: "supplier-string-123", Status: constants.ProcurementStatusCompleted, UpstreamAmount: money.FromDecimal(decimal.NewFromInt(10))}}}})
			err := svc.Execute(context.Background(), 1)
			if errors.Is(err, reconciliationcontract.ErrIncompleteResults) != tc.incomplete {
				t.Fatalf("unexpected result: %v", err)
			}
			if len(reader.references) != 1 || reader.references[0] != "supplier-string-123" {
				t.Fatalf("string supplier order skipped: %+v", reader.references)
			}
			if tc.incomplete {
				if jobs.job.Status != constants.ReconciliationJobStatusFailed || jobs.job.MatchedCount != 0 {
					t.Fatalf("unchecked order marked matched: %+v", jobs.job)
				}
			} else if jobs.job.Status != constants.ReconciliationJobStatusCompleted || jobs.job.MatchedCount != 1 {
				t.Fatalf("valid order did not reconcile: %+v", jobs.job)
			}
		})
	}
}

func TestCompareOrderTrimsUpstreamAmount(t *testing.T) {
	job := &reconciliationdomain.Job{Type: constants.ReconciliationTypeAmount}
	order := &reconciliationcontract.ProcurementOrder{UpstreamAmount: money.FromDecimal(decimal.NewFromInt(10))}
	item := compareOrder(job, order, &reconciliationcontract.UpstreamOrder{Amount: " 12.00 "})
	if item == nil || item.MismatchType != constants.MismatchTypeAmount {
		t.Fatal("upstream amount with whitespace was incorrectly treated as matched")
	}
}

type financialReaderStub struct {
	upstreamReaderStub
	amountCalls int
	amount      string
	currency    string
	amountErr   error
}

func (s *financialReaderStub) GetWithAmount(context.Context, uint, string) (*reconciliationcontract.UpstreamOrder, error) {
	s.amountCalls++
	return &reconciliationcontract.UpstreamOrder{Status: "delivered", Amount: s.amount, Currency: s.currency}, s.amountErr
}

func TestFinancialLookupCompletesOrRecordsExactAmountMismatch(t *testing.T) {
	for _, tc := range []struct {
		name, kind, amount, currency string
		queryErr                     error
		incomplete, mismatch         bool
	}{
		{"amount match", constants.ReconciliationTypeAmount, "107.50", "CNY", nil, false, false},
		{"full match", constants.ReconciliationTypeFull, "107.50", "CNY", nil, false, false},
		{"one cent discrepancy", constants.ReconciliationTypeAmount, "107.51", "CNY", nil, false, true},
		{"paid zero amount", constants.ReconciliationTypeAmount, "0.00", "CNY", nil, false, true},
		{"query failure", constants.ReconciliationTypeFull, "", "", errors.New("unavailable"), true, false},
		{"missing amount", constants.ReconciliationTypeFull, "", "CNY", nil, true, false},
		{"currency mismatch", constants.ReconciliationTypeAmount, "107.50", "USD", nil, true, false},
		{"missing currency", constants.ReconciliationTypeAmount, "107.50", "", nil, true, false},
		{"status independent", constants.ReconciliationTypeStatus, "", "", errors.New("amount offline"), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs := &jobRepositoryStub{job: &reconciliationdomain.Job{ID: 1, Type: tc.kind, Status: constants.ReconciliationJobStatusPending}}
			items := &itemRepositoryStub{}
			reader := &financialReaderStub{upstreamReaderStub: upstreamReaderStub{details: map[uint]reconciliationcontract.UpstreamOrder{0: {Status: "delivered"}}}, amount: tc.amount, currency: tc.currency, amountErr: tc.queryErr}
			order := reconciliationcontract.ProcurementOrder{ID: 7, UpstreamOrderNo: "00123", Status: constants.ProcurementStatusFulfilled, UpstreamAmount: money.FromDecimal(decimal.RequireFromString("107.50")), Currency: "CNY"}
			svc := NewService(Options{Jobs: jobs, Items: items, Upstream: &upstreamProviderStub{reader: reader}, Procurements: procurementReaderStub{orders: []reconciliationcontract.ProcurementOrder{order}}})
			err := svc.Execute(context.Background(), 1)
			if errors.Is(err, reconciliationcontract.ErrIncompleteResults) != tc.incomplete {
				t.Fatalf("unexpected result: %+v %v", jobs.job, err)
			}
			if tc.kind == constants.ReconciliationTypeStatus {
				if reader.amountCalls != 0 || reader.calls != 1 {
					t.Fatal("status query used financial endpoint")
				}
			} else if reader.amountCalls != 1 || reader.calls != 0 {
				t.Fatal("amount query did not use financial endpoint")
			}
			if tc.mismatch {
				if len(items.created) != 1 || items.created[0].MismatchType != constants.MismatchTypeAmount || !items.created[0].UpstreamAmount.Equal(decimal.RequireFromString(tc.amount)) || jobs.job.MatchedCount != 0 {
					t.Fatalf("exact discrepancy lost: %+v %+v", items.created, jobs.job)
				}
			} else if len(items.created) != 0 {
				t.Fatal("unchecked amount was recorded as discrepancy")
			}
			if tc.incomplete {
				if jobs.job.Status != constants.ReconciliationJobStatusFailed || jobs.job.MatchedCount != 0 {
					t.Fatalf("unchecked job reported success: %+v", jobs.job)
				}
			} else if jobs.job.Status != constants.ReconciliationJobStatusCompleted {
				t.Fatalf("job not completed: %+v", jobs.job)
			}
			if !tc.incomplete && !tc.mismatch && jobs.job.MatchedCount != 1 {
				t.Fatalf("valid match not counted: %+v", jobs.job)
			}
			// Completed jobs are idempotent and never change the recorded procurement cost.
			if !tc.incomplete {
				calls := reader.amountCalls
				if err := svc.Execute(context.Background(), 1); err != nil || reader.amountCalls != calls {
					t.Fatal("completed job queried twice")
				}
			}
			if !order.UpstreamAmount.Equal(decimal.RequireFromString("107.50")) {
				t.Fatal("reconciliation changed purchase cost")
			}
		})
	}
}
