package domain

import "time"

// Query 描述运营报表共享的时间范围输入。
type Query struct {
	Range        string
	From         *time.Time
	To           *time.Time
	Timezone     string
	ForceRefresh bool
}

// Window 是归一化后的左闭右开时间区间 [StartAt, EndAt)。
type Window struct {
	Range    string
	StartAt  time.Time
	EndAt    time.Time
	Timezone string
}
