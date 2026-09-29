package application

// AcknowledgeCommand 是确认合规声明的应用层命令。
type AcknowledgeCommand struct {
	Segment1  string
	Segment2  string
	Segment3  string
	AdminID   uint
	Username  string
	ClientIP  string
	UserAgent string
}
