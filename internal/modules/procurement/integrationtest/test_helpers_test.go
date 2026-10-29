package procurement_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"fmt"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/upstream"
	"github.com/hibiken/asynq"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"github.com/dujiao-next/internal/config"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	procurementmapping "github.com/dujiao-next/internal/modules/procurement/infrastructure/mappingreader"
	procurementorder "github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	siteconnectiongormstore "github.com/dujiao-next/internal/modules/siteconnection/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ListFilter = procurementcontract.ListFilter

type redemptionMailQueue struct {
	tasks []queue.OrderStatusEmailPayload
}

func (q *redemptionMailQueue) EnqueueOrderStatusEmail(p queue.OrderStatusEmailPayload, _ ...asynq.Option) error {
	q.tasks = append(q.tasks, p)
	return nil
}

func startRedemptionFixture(t *testing.T, origin string, sources []string) (string, string) {
	t.Helper()
	node := os.Getenv("PAIPLUS_TEST_NODE")
	if node == "" {
		t.Skip("set PAIPLUS_TEST_NODE to run the real Node/Go shipping-to-redemption integration")
	}
	dir := t.TempDir()
	stats := filepath.Join(dir, "stats.json")
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "redemption-center", "tests", "shop-fixture.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(sources)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	cmd := exec.CommandContext(ctx, node, script)
	cmd.Env = append(os.Environ(), "PAIPLUS_FIXTURE_DB="+filepath.Join(dir, "test.db"), "PAIPLUS_FIXTURE_STATS="+stats, "PAIPLUS_FIXTURE_ORIGIN="+origin, "PAIPLUS_FIXTURE_SOURCES="+string(encoded))
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
		} else {
			ready <- ""
		}
	}()
	select {
	case url := <-ready:
		if !strings.HasPrefix(url, "http://127.0.0.1:") {
			t.Fatal("local redemption fixture did not start")
		}
		return url, stats
	case <-time.After(10 * time.Second):
		t.Fatal("local redemption fixture startup timed out")
	}
	return "", ""
}

func redemptionStats(t *testing.T, path string) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]int
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// A local SMTP receiver is the only email mock. Rendering, worker reads and
// attachment encoding run through the real application and SMTP implementation.
func redemptionSMTP(t *testing.T) (config.EmailConfig, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	messages := make(chan string, 10)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
				reader := bufio.NewReader(conn)
				_, _ = io.WriteString(conn, "220 localhost fixture\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
						_, _ = io.WriteString(conn, "250 localhost\r\n")
					case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
						_, _ = io.WriteString(conn, "250 OK\r\n")
					case strings.HasPrefix(line, "DATA"):
						_, _ = io.WriteString(conn, "354 body\r\n")
						var body strings.Builder
						for {
							part, err := reader.ReadString('\n')
							if err != nil {
								return
							}
							if part == ".\r\n" {
								break
							}
							body.WriteString(part)
						}
						messages <- body.String()
						_, _ = io.WriteString(conn, "250 accepted\r\n")
					case strings.HasPrefix(line, "QUIT"):
						_, _ = io.WriteString(conn, "221 bye\r\n")
						return
					default:
						_, _ = io.WriteString(conn, "500 unsupported\r\n")
					}
				}
			}()
		}
	}()
	return config.EmailConfig{Enabled: true, Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, From: "sender@example.invalid"}, messages
}

func decodedRedemptionMail(t *testing.T, raw string) (string, bool) {
	t.Helper()
	message, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	contentType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	decode := func(reader io.Reader, encoding string) string {
		if strings.EqualFold(encoding, "base64") {
			reader = base64.NewDecoder(base64.StdEncoding, reader)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if !strings.HasPrefix(contentType, "multipart/") {
		return decode(message.Body, message.Header.Get("Content-Transfer-Encoding")), false
	}
	reader := multipart.NewReader(message.Body, params["boundary"])
	var text strings.Builder
	attachment := false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if part.FileName() != "" {
			attachment = true
		}
		text.WriteString(decode(part, part.Header.Get("Content-Transfer-Encoding")))
	}
	return text.String(), attachment
}

type ProcurementOrder = procurementdomain.Order

var (
	ErrExists = procurementcontract.ErrExists
)

func newTestSiteConnectionService(db *gorm.DB, secretKey, uploadsDir string) *siteconnectionapp.Service {
	return siteconnectionapp.NewService(siteconnectiongormstore.New(db), secretKey, uploadsDir)
}

func setupProcurementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:procurement_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&fulfillmentdomain.Fulfillment{},
		&procurementdomain.Order{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db
	return db
}

func createProcTestOrder(t *testing.T, db *gorm.DB, orderNo, status, fulfillmentType string) *orderdomain.Order {
	t.Helper()
	order := &orderdomain.Order{
		OrderNo:        orderNo,
		UserID:         1,
		Status:         status,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := &orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       1,
		SKUID:           1,
		Quantity:        1,
		FulfillmentType: fulfillmentType,
		TitleJSON:       jsonmap.JSON{"zh-CN": "Test Product"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(100)),
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	var loaded orderdomain.Order
	if err := db.Preload("Items").First(&loaded, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	return &loaded
}

func createTestProcurementOrder(t *testing.T, db *gorm.DB, connID, localOrderID uint, localOrderNo, status string) *ProcurementOrder {
	t.Helper()
	order := &ProcurementOrder{
		ConnectionID:    connID,
		LocalOrderID:    localOrderID,
		LocalOrderNo:    localOrderNo,
		Status:          status,
		LocalSellAmount: money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		TraceID:         "test-trace-id",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create procurement order failed: %v", err)
	}
	return order
}

func newTestProcurementService(db *gorm.DB, connections *siteconnectionapp.Service) *procurementapp.Service {
	options := procurementOptionsForTest(db)
	options.Connections = procurementupstream.New(connections)
	return procurementapp.NewService(options)
}

func procurementOptionsForTest(db *gorm.DB) procurementapp.Options {
	orders := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	return procurementapp.Options{
		Repository:      procurementgormstore.New(db),
		Orders:          procurementorder.New(orders),
		ProductMappings: procurementmapping.NewProducts(mappinggormstore.NewMappingStore(db)),
		SKUMappings:     procurementmapping.NewSKUs(mappinggormstore.NewSKUMappingStore(db)),
		OrderLifecycle:  procurementgormstore.NewLifecycle(db, nil, nil, config.EmailConfig{}),
	}
}

// The wire protocol is covered in internal/upstream/acg_faka_test.go. This
// fixture exercises the real DB claim, gateway, order lifecycle and card record.
type acgProcurementAdapter struct {
	upstream.Adapter
	calls     atomic.Int32
	result    *upstream.CreateUpstreamOrderResp
	err       error
	reference string
	request   upstream.CreateUpstreamOrderReq
}

func (*acgProcurementAdapter) RequiresSingleSubmission() bool { return true }
func (a *acgProcurementAdapter) CreateOrder(_ context.Context, request upstream.CreateUpstreamOrderReq) (*upstream.CreateUpstreamOrderResp, error) {
	a.calls.Add(1)
	a.request = request
	return a.result, a.err
}
func (a *acgProcurementAdapter) GetOrderByReference(_ context.Context, no string) (*upstream.UpstreamOrderDetail, error) {
	a.reference = no
	return &upstream.UpstreamOrderDetail{OrderNo: no, Status: "delivered", Fulfillment: &upstream.UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: "DELAYED-CARD"}}, nil
}

type acgProcurementProvider struct{ adapter *acgProcurementAdapter }

func (p *acgProcurementProvider) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	return &siteconnectiondomain.Connection{ID: id, Protocol: constants.ConnectionProtocolACGFaka, RetryMax: 2, RetryIntervals: "[30,60]"}, nil
}
func (p *acgProcurementProvider) GetAdapter(*siteconnectiondomain.Connection) (upstream.Adapter, error) {
	return p.adapter, nil
}

func acgProcurementFixture(t *testing.T, status string, adapter *acgProcurementAdapter) (*gorm.DB, *procurementapp.Service, *ProcurementOrder, *orderdomain.Order) {
	t.Helper()
	db := setupProcurementTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	order := createProcTestOrder(t, db, "ACG-ORDER-1", status, constants.FulfillmentTypeUpstream)
	if err := db.Model(&orderdomain.OrderItem{}).Where("order_id = ?", order.ID).Update("cost_price", "33.00").Error; err != nil {
		t.Fatal(err)
	}
	pm := &mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 4, IsActive: true}
	if err := db.Create(pm).Error; err != nil {
		t.Fatal(err)
	}
	sm := &mappingdomain.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 4, UpstreamIsActive: true}
	if err := db.Create(sm).Error; err != nil {
		t.Fatal(err)
	}
	// Construct the same production service, replacing only the supplier wire.
	options := procurementOptionsForTest(db)
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "pending")
	return db, svc, proc, order
}
