package application

import "time"

// SystemClock 使用系统本地时间，保持旧实现的 time.Now 语义。
type SystemClock struct{}

// Now 返回当前系统时间。
func (SystemClock) Now() time.Time {
	return time.Now()
}

// WarningLoggerFunc 将现有结构化日志函数适配为 WarningLogger。
type WarningLoggerFunc func(message string, keysAndValues ...interface{})

// Warnw 记录结构化告警。
func (f WarningLoggerFunc) Warnw(message string, keysAndValues ...interface{}) {
	f(message, keysAndValues...)
}
