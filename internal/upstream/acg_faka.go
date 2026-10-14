package upstream

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
	"golang.org/x/time/rate"
)

// ACGFakaAdapter implements the signed Shared API used by ELLAIXSHOP.
// This adapter deliberately offers automatic, single-specification card goods
// only. Unsupported variants/forms stay unavailable instead of choosing a SKU.
type ACGFakaAdapter struct {
	baseURL, merchantID, secret, uploadsDir string
	client                                  *http.Client
	amountLimiter                           *rate.Limiter
}

var _ Adapter = (*ACGFakaAdapter)(nil)
var _ ReferenceOrderAmountAdapter = (*ACGFakaAdapter)(nil)

// The supplier's state endpoint allows 120 requests per IP per 600 seconds.
// Share a conservative budget across adapters/jobs in this process.
var acgAmountLimiter = rate.NewLimiter(rate.Every(6*time.Second), 5)

func NewACGFakaAdapter(conn *siteconnectiondomain.Connection, uploadsDir string) (*ACGFakaAdapter, error) {
	base, err := url.Parse(strings.TrimRight(conn.BaseURL, "/"))
	if err != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("acg-faka requires an HTTPS site URL without credentials, query or fragment")
	}
	if id, err := strconv.ParseUint(conn.ApiKey, 10, 64); err != nil || id == 0 || strings.TrimSpace(conn.ApiSecret) == "" {
		return nil, errors.New("acg-faka requires a numeric merchant ID and merchant secret")
	}
	return &ACGFakaAdapter{
		baseURL: base.String(), merchantID: conn.ApiKey, secret: conn.ApiSecret, uploadsDir: uploadsDir,
		client:        &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		amountLimiter: acgAmountLimiter,
	}, nil
}

func (*ACGFakaAdapter) RequiresSingleSubmission() bool { return true }

// PHP: md5(urldecode(http_build_query(ksort(nonEmpty(data))) + "&key=" + secret)).
func signACGFaka(values url.Values, secret string) string {
	filtered := url.Values{}
	for key, items := range values {
		if key != "sign" && len(items) == 1 && items[0] != "" {
			filtered.Set(key, items[0])
		}
	}
	text, _ := url.QueryUnescape(filtered.Encode() + "&key=" + secret)
	sum := md5.Sum([]byte(text)) // Required by the supplier's wire protocol.
	return hex.EncodeToString(sum[:])
}

type acgAPIError struct{ message string }

func (e *acgAPIError) Error() string { return e.message }

func (a *ACGFakaAdapter) post(ctx context.Context, endpoint string, values url.Values, target any) error {
	if values == nil {
		values = url.Values{}
	}
	values.Set("app_id", a.merchantID)
	// The merchant secret stays local; the supplier authenticates app_id and sign.
	values.Del("app_key")
	values.Set("sign", signACGFaka(values, a.secret))
	return a.postForm(ctx, endpoint, values, target)
}

func (a *ACGFakaAdapter) postForm(ctx context.Context, endpoint string, values url.Values, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return errors.New("invalid acg-faka request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("acg-faka request failed: %s", a.redact(err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("acg-faka returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil || len(body) > 8<<20 {
		return errors.New("acg-faka response could not be read")
	}
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return errors.New("acg-faka returned an invalid JSON response")
	}
	if envelope.Code != 200 {
		return &acgAPIError{a.redact(envelope.Message)}
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return errors.New("acg-faka returned invalid response data")
	}
	return nil
}

func (a *ACGFakaAdapter) redact(message string) string {
	message = strings.ReplaceAll(message, a.secret, "[redacted]")
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

// The supplier sends amounts and inventory as both JSON numbers and strings.
type acgString string

func (value *acgString) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*value = ""
		return nil
	}
	var text string
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
	} else {
		var number json.Number
		if err := json.Unmarshal(raw, &number); err != nil {
			return err
		}
		text = number.String()
	}
	*value = acgString(text)
	return nil
}

func (a *ACGFakaAdapter) Ping(ctx context.Context) (*PingResult, error) {
	var data struct {
		ShopName string    `json:"shopName"`
		Balance  acgString `json:"balance"`
	}
	if err := a.post(ctx, "/shared/authentication/connect", nil, &data); err != nil {
		return nil, err
	}
	amount, err := decimal.NewFromString(string(data.Balance))
	if err != nil || amount.IsNegative() {
		return nil, errors.New("acg-faka returned invalid balance")
	}
	id, _ := strconv.ParseUint(a.merchantID, 10, 64)
	return &PingResult{SiteName: data.ShopName, ProtocolVersion: "acg-faka-shared", UserID: uint(id), Balance: amount.StringFixed(2), Currency: "CNY"}, nil
}

type acgCommodity struct {
	ID           uint            `json:"id"`
	CategoryID   uint            `json:"category_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Cover        string          `json:"cover"`
	Code         string          `json:"code"`
	Price        acgString       `json:"price"`
	UserPrice    acgString       `json:"user_price"`
	FactoryPrice acgString       `json:"factory_price"`
	Stock        acgString       `json:"stock"`
	Status       int             `json:"status"`
	DeliveryWay  int             `json:"delivery_way"`
	Config       json.RawMessage `json:"config"`
	Widget       json.RawMessage `json:"widget"`
	Minimum      int             `json:"minimum"`
	Maximum      int             `json:"maximum"`
}
type acgCategory struct {
	ID       uint           `json:"id"`
	ParentID uint           `json:"pid"`
	Name     string         `json:"name"`
	Sort     int            `json:"sort"`
	Children []acgCommodity `json:"children"`
}

func (a *ACGFakaAdapter) categories(ctx context.Context) ([]acgCategory, error) {
	var data []acgCategory
	err := a.post(ctx, "/shared/commodity/items", nil, &data)
	return data, err
}

func (a *ACGFakaAdapter) ListCategories(ctx context.Context) (*CategoryListResult, error) {
	categories, err := a.categories(ctx)
	if err != nil {
		return nil, err
	}
	result := &CategoryListResult{Supported: true, Categories: []UpstreamCategory{}}
	for _, category := range categories {
		result.Categories = append(result.Categories, UpstreamCategory{ID: category.ID, ParentID: category.ParentID, Slug: fmt.Sprintf("acg-%d", category.ID), Name: jsonmap.JSON{"zh-CN": category.Name}, SortOrder: category.Sort})
	}
	return result, nil
}

func acgEmptyConfig(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))
	return text == "" || text == "null" || text == `""` || text == "[]" || text == "{}"
}
func acgSupported(product acgCommodity) bool {
	return product.ID > 0 && product.Code != "" && product.Status == 1 && product.DeliveryWay == 0 && acgEmptyConfig(product.Config) && acgEmptyConfig(product.Widget)
}

func (a *ACGFakaAdapter) product(product acgCommodity) UpstreamProduct {
	price := product.FactoryPrice
	if price == "" {
		price = product.UserPrice
	}
	if price == "" {
		price = product.Price
	}
	amount, err := decimal.NewFromString(string(price))
	active := acgSupported(product) && err == nil && amount.GreaterThan(decimal.Zero)
	stock, err := strconv.Atoi(string(product.Stock))
	if err != nil || stock < 0 || !active {
		stock = 0
	}
	images := []string{}
	if product.Cover != "" {
		base, _ := url.Parse(a.baseURL + "/")
		cover, err := url.Parse(product.Cover)
		if err == nil {
			resolved := base.ResolveReference(cover)
			if resolved.Scheme == "https" && resolved.User == nil {
				images = append(images, resolved.String())
			}
		}
	}
	return UpstreamProduct{ID: product.ID, CategoryID: product.CategoryID, Title: jsonmap.JSON{"zh-CN": product.Name}, Description: jsonmap.JSON{"zh-CN": regexp.MustCompile(`<[^>]*>`).ReplaceAllString(product.Description, "")}, Content: jsonmap.JSON{"zh-CN": product.Description}, Images: images, PriceAmount: amount.StringFixed(2), Currency: "CNY", FulfillmentType: constants.FulfillmentTypeAuto, IsActive: active, SKUs: []UpstreamSKU{{ID: product.ID, SKUCode: fmt.Sprintf("acg-%d", product.ID), SpecValues: jsonmap.JSON{}, PriceAmount: amount.StringFixed(2), StockQuantity: stock, StockStatus: func() string {
		if stock > 0 {
			return "in_stock"
		}
		return "out_of_stock"
	}(), IsActive: active}}}
}

func (a *ACGFakaAdapter) detail(ctx context.Context, product acgCommodity) (acgCommodity, error) {
	var detail acgCommodity
	err := a.post(ctx, "/shared/commodity/item", url.Values{"code": {product.Code}}, &detail)
	if err != nil {
		return detail, err
	}
	if detail.ID != product.ID || detail.Code != product.Code {
		return detail, errors.New("acg-faka product reference changed")
	}
	return detail, nil
}

func (a *ACGFakaAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	categories, err := a.categories(ctx)
	if err != nil {
		return nil, err
	}
	flat := []acgCommodity{}
	for _, category := range categories {
		for _, product := range category.Children {
			product.CategoryID = category.ID
			flat = append(flat, product)
		}
	}
	page, size := opts.Page, opts.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	start := len(flat)
	if page-1 <= len(flat)/size {
		start = min((page-1)*size, len(flat))
	}
	end := min(start+size, len(flat))
	result := &ProductListResult{Total: len(flat), Items: []UpstreamProduct{}, IncludesInactive: false}
	for _, product := range flat[start:end] {
		detail, err := a.detail(ctx, product)
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, a.product(detail))
	}
	return result, nil
}

func (a *ACGFakaAdapter) loadProduct(ctx context.Context, id uint) (acgCommodity, error) {
	categories, err := a.categories(ctx)
	if err != nil {
		return acgCommodity{}, err
	}
	for _, category := range categories {
		for _, product := range category.Children {
			if product.ID == id {
				return a.detail(ctx, product)
			}
		}
	}
	return acgCommodity{}, ErrUpstreamProductUnavailable
}

func (a *ACGFakaAdapter) quote(ctx context.Context, code string, quantity int) (decimal.Decimal, error) {
	var quote struct {
		Price    acgString `json:"price"`
		Currency string    `json:"currency_code"`
	}
	if err := a.post(ctx, "/shared/commodity/valuation", url.Values{"code": {code}, "num": {strconv.Itoa(quantity)}, "race": {""}, "card_id": {"0"}}, &quote); err != nil {
		return decimal.Zero, err
	}
	amount, err := decimal.NewFromString(string(quote.Price))
	if err != nil || !amount.GreaterThan(decimal.Zero) || quote.Currency != "CNY" {
		return decimal.Zero, errors.New("acg-faka returned an invalid price or unsupported currency")
	}
	return amount, nil
}
func (a *ACGFakaAdapter) stock(ctx context.Context, code string) (int, error) {
	var data struct {
		Stock acgString `json:"stock"`
	}
	if err := a.post(ctx, "/shared/commodity/stock", url.Values{"code": {code}, "race": {""}}, &data); err != nil {
		return 0, err
	}
	stock, err := strconv.Atoi(string(data.Stock))
	if err != nil || stock < 0 {
		return 0, errors.New("acg-faka returned invalid stock")
	}
	return stock, nil
}

func (a *ACGFakaAdapter) GetProduct(ctx context.Context, id uint) (*UpstreamProduct, error) {
	product, err := a.loadProduct(ctx, id)
	if err != nil {
		return nil, err
	}
	if acgSupported(product) {
		price, err := a.quote(ctx, product.Code, 1)
		if err != nil {
			return nil, err
		}
		product.FactoryPrice = acgString(price.StringFixed(2))
		stock, err := a.stock(ctx, product.Code)
		if err != nil {
			return nil, err
		}
		product.Stock = acgString(strconv.Itoa(stock))
	}
	result := a.product(product)
	return &result, nil
}

func acgBusinessCode(message string) string {
	switch {
	case strings.Contains(message, "余额不足"):
		return "insufficient_balance"
	case strings.Contains(message, "库存不足") || strings.Contains(message, "缺货"):
		return "product_out_of_stock"
	case strings.Contains(message, "request ID already exists"):
		return "duplicate_order"
	case strings.Contains(message, "白名单"):
		return "forbidden"
	case strings.Contains(message, "密钥") || strings.Contains(message, "商户ID"):
		return "unauthorized"
	default:
		return "unknown_outcome"
	}
}

func (a *ACGFakaAdapter) CreateOrder(ctx context.Context, request CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	if !purchasesEnabled() {
		return purchaseDisabledResponse(), nil
	}
	rejected := func(code, message string) (*CreateUpstreamOrderResp, error) {
		return &CreateUpstreamOrderResp{ErrorCode: code, ErrorMessage: message}, nil
	}
	if request.Quantity < 1 || request.DownstreamOrderNo == "" || request.Currency != "CNY" {
		return rejected("invalid_request", "acg-faka requires a CNY order, quantity and downstream order number")
	}
	maximum, err := decimal.NewFromString(request.MaxAmount)
	if err != nil || !maximum.GreaterThan(decimal.Zero) {
		return rejected("invalid_request", "a positive purchase cost limit is required")
	}
	product, err := a.loadProduct(ctx, request.SKUID)
	if err != nil {
		return rejected("supplier_unavailable", err.Error()) // Read-only preflight; no charge was submitted.
	}
	if !acgSupported(product) {
		return rejected("product_unavailable", "only automatic single-specification card products without forms are supported")
	}
	if (product.Minimum > 0 && request.Quantity < product.Minimum) || (product.Maximum > 0 && request.Quantity > product.Maximum) {
		return rejected("invalid_request", "quantity is outside supplier limits")
	}
	amount, err := a.quote(ctx, product.Code, request.Quantity)
	if err != nil {
		return rejected("supplier_unavailable", err.Error())
	}
	if amount.GreaterThan(maximum) {
		return rejected("upstream_price_changed", "supplier cost exceeds this paid order's purchase cost limit; review the order before purchasing")
	}
	stock, err := a.stock(ctx, product.Code)
	if err != nil {
		return rejected("supplier_unavailable", err.Error())
	}
	if stock < request.Quantity {
		return rejected("product_out_of_stock", "supplier stock is insufficient")
	}
	account, err := a.Ping(ctx)
	if err != nil {
		return rejected("supplier_unavailable", err.Error())
	}
	balance, _ := decimal.NewFromString(account.Balance)
	if balance.LessThan(amount) {
		return rejected("insufficient_balance", "supplier merchant balance is insufficient")
	}
	identity := request.TraceID
	if identity == "" {
		identity = request.DownstreamOrderNo
	}
	digest := sha256.Sum256([]byte(a.merchantID + ":" + identity))
	requestNo := "zxa-" + hex.EncodeToString(digest[:24])
	values := url.Values{"shared_code": {product.Code}, "num": {strconv.Itoa(request.Quantity)}, "contact": {"orders@zhixiang.invalid"}, "card_id": {"0"}, "device": {"0"}, "password": {""}, "race": {""}, "request_no": {requestNo}}
	var data struct {
		OrderNo acgString `json:"tradeNo"`
		Amount  acgString `json:"amount"`
		Secret  acgString `json:"secret"`
	}
	if err := a.post(ctx, "/shared/commodity/trade", values, &data); err != nil {
		var business *acgAPIError
		if errors.As(err, &business) {
			code := acgBusinessCode(business.message)
			if code != "unknown_outcome" {
				return rejected(code, business.message)
			}
		}
		return nil, err // Unknown outcome after a purchase request: never resend.
	}
	paid, err := decimal.NewFromString(string(data.Amount))
	if data.OrderNo == "" || err != nil || !paid.GreaterThan(decimal.Zero) {
		return nil, errors.New("acg-faka purchase response is incomplete; reconcile it before any retry")
	}
	result := &CreateUpstreamOrderResp{OK: true, OrderNo: string(data.OrderNo), Status: "accepted", Amount: paid.StringFixed(2), Currency: "CNY"}
	if strings.TrimSpace(string(data.Secret)) != "" {
		result.Fulfillment = &UpstreamFulfillment{Type: constants.FulfillmentTypeAuto, Status: "delivered", Payload: string(data.Secret)}
		result.Status = "delivered"
	}
	return result, nil
}

type acgOrderQuery struct {
	Secret acgString `json:"secret"`
	Status *int      `json:"status"`
}

func (a *ACGFakaAdapter) queryOrder(ctx context.Context, orderNo string) (*UpstreamOrderDetail, *acgOrderQuery, error) {
	if strings.TrimSpace(orderNo) == "" {
		return nil, nil, errors.New("supplier order number is required")
	}
	var data acgOrderQuery
	if err := a.post(ctx, "/shared/commodity/query?tradeNo="+url.QueryEscape(orderNo), url.Values{"tradeNo": {orderNo}}, &data); err != nil {
		return nil, nil, err
	}
	if data.Status == nil || (*data.Status != 0 && *data.Status != 1) {
		return nil, nil, errors.New("acg-faka returned invalid order status")
	}
	result := &UpstreamOrderDetail{OrderNo: orderNo, Status: "accepted", Currency: "CNY"}
	if *data.Status == 1 && strings.TrimSpace(string(data.Secret)) != "" {
		result.Status = "delivered"
		result.Fulfillment = &UpstreamFulfillment{Type: constants.FulfillmentTypeAuto, Status: "delivered", Payload: string(data.Secret)}
	}
	return result, &data, nil
}

func (a *ACGFakaAdapter) GetOrderByReference(ctx context.Context, orderNo string) (*UpstreamOrderDetail, error) {
	result, _, err := a.queryOrder(ctx, orderNo)
	return result, err
}

var acgCNYAmount = regexp.MustCompile(`^[0-9]{1,16}(\.[0-9]{1,2})?$`)

func (a *ACGFakaAdapter) GetOrderWithAmountByReference(ctx context.Context, orderNo string) (*UpstreamOrderDetail, error) {
	// Authenticate ownership before querying the public payment-state endpoint.
	result, query, err := a.queryOrder(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if err := a.amountLimiter.Wait(ctx); err != nil {
		return nil, errors.New("acg-faka amount query canceled or deadline exceeded")
	}
	var state struct {
		OrderNo      acgString `json:"trade_no"`
		Amount       acgString `json:"amount"`
		Status       *int      `json:"status"`
		Currency     string    `json:"currency"`
		CurrencyCode string    `json:"currency_code"`
	}
	// This endpoint needs only the reference. Never send merchant credentials here.
	if err := a.postForm(ctx, "/user/api/order/state", url.Values{"tradeNo": {orderNo}}, &state); err != nil {
		return nil, err
	}
	if string(state.OrderNo) != orderNo || state.Status == nil || *state.Status != 1 || *query.Status != *state.Status {
		return nil, errors.New("acg-faka amount response does not identify the same paid order")
	}
	// This adapter only purchases CNY. Reject an explicit conflicting currency.
	for _, currency := range []string{state.Currency, state.CurrencyCode} {
		if currency != "" && currency != "CNY" {
			return nil, errors.New("acg-faka amount response has an unsupported currency")
		}
	}
	if !acgCNYAmount.MatchString(string(state.Amount)) {
		return nil, errors.New("acg-faka amount response is missing or invalid")
	}
	amount, err := decimal.NewFromString(string(state.Amount))
	if err != nil {
		return nil, errors.New("acg-faka amount response is invalid")
	}
	result.Amount = amount.StringFixed(2)
	return result, nil
}
func (a *ACGFakaAdapter) GetOrder(ctx context.Context, id uint) (*UpstreamOrderDetail, error) {
	return a.GetOrderByReference(ctx, strconv.FormatUint(uint64(id), 10))
}
func (*ACGFakaAdapter) CancelOrder(context.Context, uint) error {
	return errors.New("acg-faka balance purchases cannot be canceled through the Shared API; reconcile with the supplier")
}
func (a *ACGFakaAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	return (&DujiaoNextAdapter{baseURL: a.baseURL, uploadsDir: a.uploadsDir, client: a.client}).DownloadImage(ctx, imageURL)
}
