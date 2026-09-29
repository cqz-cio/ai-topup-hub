package domain

// Status 表示合规声明的确认状态。
type Status struct {
	Acknowledged           bool
	AcknowledgedAt         string
	AcknowledgedByAdminID  uint
	AcknowledgedByUsername string
	Version                string
}
