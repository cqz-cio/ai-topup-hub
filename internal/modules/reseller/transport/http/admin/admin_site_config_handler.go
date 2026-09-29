package adminhttp

import (
	"context"
	"strings"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

type AdminSiteConfigService interface {
	UpdateAdminSiteConfig(ctx context.Context, resellerID uint, input resellermodule.ResellerSiteConfigInput) (*resellerdomain.SiteConfig, error)
	ResetAdminSiteConfig(ctx context.Context, resellerID uint) error
}

type SiteConfigDirectory interface {
	ListSiteConfigs(filter resellercontract.SiteConfigListFilter) ([]resellerdomain.SiteConfig, int64, error)
	GetSiteConfigByResellerID(resellerID uint) (*resellerdomain.SiteConfig, error)
	GetProfileByID(id uint) (*resellerdomain.Profile, error)
}

type AdminSiteConfigHandler struct {
	siteConfig AdminSiteConfigService
	directory  SiteConfigDirectory
	audit      AuditRecorder
}

func NewAdminSiteConfigHandler(siteConfig AdminSiteConfigService, directory SiteConfigDirectory, audit AuditRecorder) *AdminSiteConfigHandler {
	if siteConfig == nil || directory == nil {
		panic("reseller admin site config handler: required dependency is nil")
	}
	return &AdminSiteConfigHandler{siteConfig: siteConfig, directory: directory, audit: audit}
}

type adminSiteConfigRequest struct {
	SiteName     string                                   `json:"site_name"`
	Logo         string                                   `json:"logo"`
	Favicon      string                                   `json:"favicon"`
	Announcement resellermodule.ResellerAnnouncementInput `json:"announcement"`
	Support      resellermodule.ResellerSupportInput      `json:"support"`
	SEO          resellermodule.ResellerSEOInput          `json:"seo"`
	FooterLinks  []resellermodule.ResellerFooterLinkInput `json:"footer_links"`
	NavConfig    resellermodule.ResellerNavConfigInput    `json:"nav_config"`
}

func (req adminSiteConfigRequest) toInput() resellermodule.ResellerSiteConfigInput {
	return resellermodule.ResellerSiteConfigInput{
		SiteName:     req.SiteName,
		Logo:         req.Logo,
		Favicon:      req.Favicon,
		Announcement: req.Announcement,
		Support:      req.Support,
		SEO:          req.SEO,
		FooterLinks:  req.FooterLinks,
		NavConfig:    req.NavConfig,
	}
}

// ListSiteConfigs 管理端分销站点配置列表。
func (h *AdminSiteConfigHandler) ListSiteConfigs(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	resellerID, _ := ginutil.ParseQueryUint(c.Query("reseller_id"), false)
	rows, total, err := h.directory.ListSiteConfigs(resellercontract.SiteConfigListFilter{
		Page:        page,
		PageSize:    pageSize,
		ResellerID:  resellerID,
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		CreatedFrom: parseTimePointer(c.Query("created_from")),
		CreatedTo:   parseTimePointer(c.Query("created_to")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, dto.NewAdminResellerSiteConfigRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetSiteConfig 管理端获取单个分销站点配置。
func (h *AdminSiteConfigHandler) GetSiteConfig(c *gin.Context) {
	resellerID, err := ginutil.ParseParamUint(c, "reseller_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	row, err := h.directory.GetSiteConfigByResellerID(resellerID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if row == nil {
		profile, profileErr := h.directory.GetProfileByID(resellerID)
		if profileErr != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", profileErr)
			return
		}
		if profile == nil {
			ginutil.RespondError(c, response.CodeNotFound, "error.bad_request", nil)
			return
		}
		row = &resellerdomain.SiteConfig{ResellerID: resellerID, Profile: profile}
	}
	response.Success(c, dto.NewAdminResellerSiteConfigResp(row))
}

// UpdateSiteConfig 管理端更新分销站点配置。
func (h *AdminSiteConfigHandler) UpdateSiteConfig(c *gin.Context) {
	resellerID, err := ginutil.ParseParamUint(c, "reseller_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req adminSiteConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	row, err := h.siteConfig.UpdateAdminSiteConfig(c.Request.Context(), resellerID, req.toInput())
	if err != nil {
		respondAdminManagementError(c, err)
		return
	}
	if reloaded, reloadErr := h.directory.GetSiteConfigByResellerID(resellerID); reloadErr == nil && reloaded != nil {
		row = reloaded
	}
	h.recordAudit(c, "reseller_site_config_update", "/admin/resellers/site-configs/:reseller_id", "PUT", jsonmap.JSON{
		"reseller_id":    resellerID,
		"config_id":      row.ID,
		"site_name":      row.SiteName,
		"changed_fields": []string{"site_name", "logo", "favicon", "announcement", "support", "seo", "footer_links", "nav_config"},
		"source":         "admin",
	})
	response.Success(c, dto.NewAdminResellerSiteConfigResp(row))
}

// ResetSiteConfig 管理端重置分销站点配置。
func (h *AdminSiteConfigHandler) ResetSiteConfig(c *gin.Context) {
	resellerID, err := ginutil.ParseParamUint(c, "reseller_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.siteConfig.ResetAdminSiteConfig(c.Request.Context(), resellerID); err != nil {
		respondAdminManagementError(c, err)
		return
	}
	h.recordAudit(c, "reseller_site_config_reset", "/admin/resellers/site-configs/:reseller_id/reset", "POST", jsonmap.JSON{
		"reseller_id": resellerID,
		"source":      "admin",
	})
	response.Success(c, gin.H{"ok": true})
}

func (h *AdminSiteConfigHandler) recordAudit(c *gin.Context, action, object, method string, detail jsonmap.JSON) {
	if h == nil || h.audit == nil {
		return
	}
	_ = h.audit.Record(auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: c.GetString("username"),
		Action:           action,
		Object:           object,
		Method:           method,
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail:           detail,
	})
}
