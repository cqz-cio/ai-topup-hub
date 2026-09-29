package userhttp

import (
	"strings"
	"time"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// UserOrderService 是用户中心分销订单只读端点所需的最小用例接口。
type UserOrderService interface {
	ListUserOrders(userID uint, input resellermodule.OrderListInput) ([]resellermodule.OrderListItem, int64, error)
	GetUserOrderDetail(userID uint, orderNo string) (*resellermodule.OrderDetail, error)
	StatsUserOrders(userID uint, input resellermodule.OrderListInput) (resellermodule.OrderStats, error)
}

// UserOrderHandler 处理用户中心分销销售订单只读请求。
type UserOrderHandler struct {
	orders UserOrderService
}

func NewUserOrderHandler(orders UserOrderService) *UserOrderHandler {
	if orders == nil {
		panic("reseller user order handler: orders is nil")
	}
	return &UserOrderHandler{orders: orders}
}

// ListOrders 查询当前分销商视角的销售订单。
func (h *UserOrderHandler) ListOrders(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	input, err := orderListInputFromQuery(c, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	rows, total, err := h.orders.ListUserOrders(uid, input)
	if err != nil {
		respondUserOrderError(c, err, "error.order_fetch_failed")
		return
	}
	response.SuccessWithPage(c, dto.NewResellerOrderRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetOrderDetail 获取当前分销商视角的销售订单详情。
func (h *UserOrderHandler) GetOrderDetail(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	detail, err := h.orders.GetUserOrderDetail(uid, orderNo)
	if err != nil {
		respondUserOrderError(c, err, "error.order_fetch_failed")
		return
	}
	response.Success(c, dto.NewResellerOrderDetailResp(detail))
}

// GetOrderStats 获取当前分销商视角的销售订单统计。
func (h *UserOrderHandler) GetOrderStats(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	input, err := orderListInputFromQuery(c, 1, 0)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	stats, err := h.orders.StatsUserOrders(uid, input)
	if err != nil {
		respondUserOrderError(c, err, "error.order_fetch_failed")
		return
	}
	response.Success(c, dto.NewResellerOrderStatsResp(stats))
}

func orderListInputFromQuery(c *gin.Context, page, pageSize int) (resellermodule.OrderListInput, error) {
	createdFrom, err := parseOrderTimeQuery(c.Query("created_from"), false)
	if err != nil {
		return resellermodule.OrderListInput{}, err
	}
	createdTo, err := parseOrderTimeQuery(c.Query("created_to"), true)
	if err != nil {
		return resellermodule.OrderListInput{}, err
	}
	paidFrom, err := parseOrderTimeQuery(c.Query("paid_from"), false)
	if err != nil {
		return resellermodule.OrderListInput{}, err
	}
	paidTo, err := parseOrderTimeQuery(c.Query("paid_to"), true)
	if err != nil {
		return resellermodule.OrderListInput{}, err
	}
	return resellermodule.OrderListInput{
		Page:        page,
		PageSize:    pageSize,
		Status:      strings.TrimSpace(c.Query("status")),
		OrderNo:     strings.TrimSpace(c.Query("order_no")),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		PaidFrom:    paidFrom,
		PaidTo:      paidTo,
	}, nil
}

func parseOrderTimeQuery(raw string, endOfDay bool) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil, err
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return &parsed, nil
}
