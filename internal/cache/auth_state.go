package cache

import (
	"context"
	"fmt"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
)

const authStateCacheTTL = 10 * time.Minute

// UserAuthState 用户鉴权快照
// token_invalid_before 为 Unix 秒时间戳，0 表示未设置
// 该结构仅用于服务端 Redis 缓存
// 字段保持简洁，避免重复查询数据库
type UserAuthState struct {
	UserID             uint   `json:"user_id"`
	Status             string `json:"status"`
	TokenVersion       uint64 `json:"token_version"`
	TokenInvalidBefore int64  `json:"token_invalid_before"`
	UpdatedAt          int64  `json:"updated_at"`
}

// AdminAuthState 管理员鉴权快照
type AdminAuthState struct {
	AdminID            uint   `json:"admin_id"`
	Username           string `json:"username"`
	TokenVersion       uint64 `json:"token_version"`
	TokenInvalidBefore int64  `json:"token_invalid_before"`
	IsSuper            bool   `json:"is_super"`
	UpdatedAt          int64  `json:"updated_at"`
}

func userAuthStateKey(userID uint) string {
	return fmt.Sprintf("auth:user:%d", userID)
}

func adminAuthStateKey(adminID uint) string {
	return fmt.Sprintf("auth:admin:%d", adminID)
}

// BuildUserAuthState 从用户模型构建鉴权快照
func BuildUserAuthState(user *userdomain.User) *UserAuthState {
	if user == nil {
		return nil
	}
	state := &UserAuthState{
		UserID:       user.ID,
		Status:       user.Status,
		TokenVersion: user.TokenVersion,
		UpdatedAt:    time.Now().Unix(),
	}
	if user.TokenInvalidBefore != nil {
		state.TokenInvalidBefore = user.TokenInvalidBefore.Unix()
	}
	return state
}

// BuildAdminAuthState 从管理员模型构建鉴权快照
func BuildAdminAuthState(admin *admindomain.Admin) *AdminAuthState {
	if admin == nil {
		return nil
	}
	state := &AdminAuthState{
		AdminID:      admin.ID,
		Username:     admin.Username,
		TokenVersion: admin.TokenVersion,
		IsSuper:      admin.IsSuper,
		UpdatedAt:    time.Now().Unix(),
	}
	if admin.TokenInvalidBefore != nil {
		state.TokenInvalidBefore = admin.TokenInvalidBefore.Unix()
	}
	return state
}

// GetUserAuthState 获取用户鉴权快照
func GetUserAuthState(ctx context.Context, userID uint) (*UserAuthState, bool, error) {
	if userID == 0 {
		return nil, false, nil
	}
	var state UserAuthState
	hit, err := GetJSON(ctx, userAuthStateKey(userID), &state)
	if err != nil || !hit {
		return nil, hit, err
	}
	return &state, true, nil
}

// SetUserAuthState 写入用户鉴权快照
func SetUserAuthState(ctx context.Context, state *UserAuthState) error {
	if state == nil || state.UserID == 0 {
		return nil
	}
	return SetJSON(ctx, userAuthStateKey(state.UserID), state, authStateCacheTTL)
}

// DelUserAuthState 删除用户鉴权快照
func DelUserAuthState(ctx context.Context, userID uint) error {
	if userID == 0 {
		return nil
	}
	return Del(ctx, userAuthStateKey(userID))
}

// GetAdminAuthState 获取管理员鉴权快照
func GetAdminAuthState(ctx context.Context, adminID uint) (*AdminAuthState, bool, error) {
	if adminID == 0 {
		return nil, false, nil
	}
	var state AdminAuthState
	hit, err := GetJSON(ctx, adminAuthStateKey(adminID), &state)
	if err != nil || !hit {
		return nil, hit, err
	}
	return &state, true, nil
}

// SetAdminAuthState 写入管理员鉴权快照
func SetAdminAuthState(ctx context.Context, state *AdminAuthState) error {
	if state == nil || state.AdminID == 0 {
		return nil
	}
	return SetJSON(ctx, adminAuthStateKey(state.AdminID), state, authStateCacheTTL)
}

// DelAdminAuthState 删除管理员鉴权快照
func DelAdminAuthState(ctx context.Context, adminID uint) error {
	if adminID == 0 {
		return nil
	}
	return Del(ctx, adminAuthStateKey(adminID))
}
