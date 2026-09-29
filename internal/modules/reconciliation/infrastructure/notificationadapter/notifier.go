package notificationadapter

import (
	"fmt"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/contract"
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"
)

type Notifier struct {
	enqueuer contract.NotificationEnqueuer
}

var _ reconciliationcontract.MismatchNotifier = (*Notifier)(nil)

func New(enqueuer contract.NotificationEnqueuer) reconciliationcontract.MismatchNotifier {
	if enqueuer == nil {
		return nil
	}
	return &Notifier{enqueuer: enqueuer}
}

func (n *Notifier) NotifyMismatch(job *reconciliationdomain.Job) error {
	return n.enqueuer.Enqueue(contract.EnqueueInput{
		EventType: constants.NotificationEventExceptionAlert,
		BizType:   constants.NotificationBizTypeReconciliation,
		BizID:     job.ID,
		Data: map[string]any{
			"message": fmt.Sprintf("对账任务 #%d 完成，发现 %d 项差异", job.ID, job.MismatchedCount),
			"job_id":  job.ID, "connection_id": job.ConnectionID,
			"total_count": job.TotalCount, "mismatched_count": job.MismatchedCount,
		},
	})
}
