package domain

import (
	"time"
)

// Media 素材库
type Media struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	Name      string     `gorm:"type:varchar(255);not null;index" json:"name"`       // 自定义素材名称（默认=原始文件名，管理员可编辑）
	Filename  string     `gorm:"type:varchar(255);not null" json:"filename"`         // 原始文件名（不可变）
	Path      string     `gorm:"type:varchar(500);not null;uniqueIndex" json:"path"` // /uploads/scene/year/month/uuid.ext
	MimeType  string     `gorm:"type:varchar(100);not null" json:"mime_type"`
	Size      int64      `gorm:"not null" json:"size"`
	Scene     string     `gorm:"type:varchar(60);not null;index" json:"scene"`
	Width     int        `gorm:"not null;default:0" json:"width"`
	Height    int        `gorm:"not null;default:0" json:"height"`
	CreatedAt time.Time  `gorm:"index" json:"created_at"`
	DeletedAt *time.Time `gorm:"index" json:"-"`
}

func (Media) TableName() string {
	return "media"
}
