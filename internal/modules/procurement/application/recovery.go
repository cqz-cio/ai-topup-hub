package application

import (
	"errors"
	"fmt"
	"time"

	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
)

// RecoverPaidOrders repairs work after a failed callback enqueue or a restart.
// It never sends a purchase itself and never retries an uncertain submission.
func (s *Service) RecoverPaidOrders() error {
	reader, ok := s.procRepo.(procurementcontract.RecoveryReader)
	if !ok || s.queue == nil {
		return nil
	}
	ids, err := reader.FindPaidOrdersMissingProcurement(100)
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err := s.CreateForOrder(id); err != nil && !errors.Is(err, procurementcontract.ErrExists) {
			failures = append(failures, fmt.Errorf("recover paid order %d: %w", id, err))
		}
	}
	orders, err := reader.ListRecoverable(time.Now().Add(-2*time.Minute), 100)
	if err != nil {
		return err
	}
	for i := range orders {
		order := &orders[i]
		switch order.Status {
		case "pending", "failed":
			err = s.queue.EnqueueSubmit(order.ID, 0)
		case "accepted":
			err = s.queue.EnqueuePoll(order.ID, 0)
		case "submitting":
			err = s.markSubmissionUnknown(order, "worker stopped during a supplier purchase; reconcile before retrying")
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
