package memberlevelhttp

import (
	"fmt"
	"net/http"

	"github.com/dujiao-next/internal/i18n"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	memberlevelpresenter "github.com/dujiao-next/internal/modules/memberlevel/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// ActiveLevelReader 是前台/渠道会员等级只读端口。
type ActiveLevelReader interface {
	ListActiveLevels() ([]memberleveldomain.MemberLevel, error)
}

// PublicHandler 处理前台公开会员等级请求。
type PublicHandler struct {
	levels ActiveLevelReader
}

func NewPublicHandler(levels ActiveLevelReader) *PublicHandler {
	if levels == nil {
		panic("memberlevel public handler: levels is nil")
	}
	return &PublicHandler{levels: levels}
}

// List 获取公共会员等级列表。
func (h *PublicHandler) List(c *gin.Context) {
	levels, err := h.levels.ListActiveLevels()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.member_level_fetch_failed", err)
		return
	}

	views := make([]memberlevelpresenter.MemberLevel, 0, len(levels))
	for _, l := range levels {
		views = append(views, memberlevelpresenter.MemberLevel{
			ID:                l.ID,
			Name:              l.NameJSON,
			Slug:              l.Slug,
			Icon:              l.Icon,
			DiscountRate:      l.DiscountRate.Decimal.InexactFloat64(),
			RechargeThreshold: l.RechargeThreshold.Decimal.InexactFloat64(),
			SpendThreshold:    l.SpendThreshold.Decimal.InexactFloat64(),
			IsDefault:         l.IsDefault,
			SortOrder:         l.SortOrder,
		})
	}
	response.Success(c, views)
}

// ChannelHandler 处理渠道会员等级请求。
type ChannelHandler struct {
	levels ActiveLevelReader
}

func NewChannelHandler(levels ActiveLevelReader) *ChannelHandler {
	if levels == nil {
		panic("memberlevel channel handler: levels is nil")
	}
	return &ChannelHandler{levels: levels}
}

type channelLevelItem struct {
	ID                uint    `json:"id"`
	Name              string  `json:"name"`
	Slug              string  `json:"slug"`
	Icon              string  `json:"icon"`
	DiscountRate      float64 `json:"discount_rate"`
	RechargeThreshold float64 `json:"recharge_threshold"`
	SpendThreshold    float64 `json:"spend_threshold"`
	IsDefault         bool    `json:"is_default"`
	SortOrder         int     `json:"sort_order"`
}

// List GET /api/v1/channel/member-levels?locale=zh-CN
func (h *ChannelHandler) List(c *gin.Context) {
	locale := c.DefaultQuery("locale", "zh-CN")
	defaultLocale := "zh-CN"

	levels, err := h.levels.ListActiveLevels()
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_member_levels_list", "error", err)
		localeKey := i18n.ResolveLocale(c)
		msg := i18n.T(localeKey, "error.internal_error")
		response.ChannelError(c, http.StatusInternalServerError, response.CodeInternal, msg, "internal_error")
		return
	}

	items := make([]channelLevelItem, 0, len(levels))
	for _, l := range levels {
		items = append(items, channelLevelItem{
			ID:                l.ID,
			Name:              resolveLocalizedJSON(l.NameJSON, locale, defaultLocale),
			Slug:              l.Slug,
			Icon:              l.Icon,
			DiscountRate:      l.DiscountRate.Decimal.InexactFloat64(),
			RechargeThreshold: l.RechargeThreshold.Decimal.InexactFloat64(),
			SpendThreshold:    l.SpendThreshold.Decimal.InexactFloat64(),
			IsDefault:         l.IsDefault,
			SortOrder:         l.SortOrder,
		})
	}

	response.ChannelSuccess(c, gin.H{"items": items})
}

func resolveLocalizedJSON(m jsonmap.JSON, locale, defaultLocale string) string {
	if len(m) == 0 {
		return ""
	}
	if v, ok := m[locale]; ok {
		if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
			return s
		}
	}
	if v, ok := m[defaultLocale]; ok {
		if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
			return s
		}
	}
	for _, v := range m {
		if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}
