package systemhttp

import (
	"context"
	"errors"
	"time"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/platform/runtimecap"
	"github.com/dujiao-next/internal/selfupdate"
	"github.com/dujiao-next/internal/version"

	"github.com/gin-gonic/gin"
)

// ReleaseChecker 系统版本检测端口。
type ReleaseChecker interface {
	CheckLatestRelease(ctx context.Context) (*version.CheckResult, error)
}

type defaultReleaseChecker struct{}

func (defaultReleaseChecker) CheckLatestRelease(ctx context.Context) (*version.CheckResult, error) {
	return version.CheckLatestRelease(ctx)
}

// AdminHandler 处理后台系统信息请求。
type AdminHandler struct {
	releases ReleaseChecker
	// updates 一键升级任务管理器。进程内单例，串行化升级任务并保存可轮询的进度。
	updates  *selfupdate.Manager
	features func() runtimecap.Features
}

func NewAdminHandler(releases ReleaseChecker, features ...func() runtimecap.Features) *AdminHandler {
	if releases == nil {
		releases = defaultReleaseChecker{}
	}
	handler := &AdminHandler{
		releases: releases,
		updates:  selfupdate.NewManager(),
	}
	if len(features) > 0 {
		handler.features = features[0]
	}
	return handler
}

func (h *AdminHandler) GetFeatures(c *gin.Context) {
	features := runtimecap.Resolve(runtimecap.Options{})
	if h.features != nil {
		features = h.features()
	}
	response.Success(c, features)
}

// CheckSystemUpdate 通过 GitHub Releases API 检测是否有新版本发布
// GET /api/v1/admin/system/version/check
func (h *AdminHandler) CheckSystemUpdate(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	result, err := h.releases.CheckLatestRelease(ctx)
	if err != nil {
		if errors.Is(err, version.ErrRateLimited) {
			ginutil.RespondError(c, response.CodeTooManyRequests, "error.update_check_rate_limited", err)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.update_check_failed", err)
		return
	}

	response.Success(c, result)
}
