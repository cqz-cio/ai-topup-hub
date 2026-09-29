//go:build !fullstack

package web

import "io/fs"

// Enabled 报告 fullstack 资源是否被 embed 进当前二进制。
func Enabled() bool { return false }

// AdminFS 默认构建模式下返回 nil（无 embed 资源）。
func AdminFS() fs.FS { return nil }

// UserFS 默认构建模式下返回 nil。
func UserFS() fs.FS { return nil }
