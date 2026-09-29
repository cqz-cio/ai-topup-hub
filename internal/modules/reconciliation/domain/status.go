package domain

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
)

// IsStatusConsistent 判断本地采购状态与上游状态是否处于允许的同一业务窗口。
func IsStatusConsistent(localStatus, upstreamStatus string) bool {
	localStatus = strings.ToLower(strings.TrimSpace(localStatus))
	upstreamStatus = strings.ToLower(strings.TrimSpace(upstreamStatus))
	switch localStatus {
	case constants.ProcurementStatusCompleted, constants.ProcurementStatusFulfilled:
		return upstreamStatus == "completed" || upstreamStatus == "delivered" || upstreamStatus == "fulfilled" ||
			upstreamStatus == "refunded" || upstreamStatus == "partially_refunded"
	case constants.ProcurementStatusCanceled:
		return upstreamStatus == "canceled" || upstreamStatus == "cancelled" || upstreamStatus == "refunded" || upstreamStatus == "partially_refunded"
	case constants.ProcurementStatusPending:
		return upstreamStatus == "pending" || upstreamStatus == "paid"
	case constants.ProcurementStatusSubmitted, constants.ProcurementStatusAccepted:
		return upstreamStatus == "paid" || upstreamStatus == "processing" || upstreamStatus == "accepted"
	case constants.ProcurementStatusFailed, constants.ProcurementStatusRejected:
		return upstreamStatus == "failed" || upstreamStatus == "rejected"
	case "fulfilling":
		return upstreamStatus == "fulfilling" || upstreamStatus == "processing" || upstreamStatus == "paid"
	default:
		return localStatus == upstreamStatus
	}
}
