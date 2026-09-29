package reconciliationhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"

	"github.com/gin-gonic/gin"
)

type reconciliationServiceStub struct {
	resolveErr error
	itemID     uint
	adminID    uint
	remark     string
}

func (s *reconciliationServiceStub) CreateAndEnqueue(reconciliationcontract.RunInput) (*reconciliationdomain.Job, error) {
	return nil, nil
}

func (s *reconciliationServiceStub) ListJobs(reconciliationcontract.JobListFilter) ([]reconciliationdomain.Job, int64, error) {
	return nil, 0, nil
}

func (s *reconciliationServiceStub) GetJob(uint) (*reconciliationdomain.Job, error) {
	return nil, nil
}

func (s *reconciliationServiceStub) GetJobItems(uint, int, int) ([]reconciliationdomain.Item, int64, error) {
	return nil, 0, nil
}

func (s *reconciliationServiceStub) ResolveItem(itemID, adminID uint, remark string) error {
	s.itemID, s.adminID, s.remark = itemID, adminID, remark
	return s.resolveErr
}

func TestResolveItemMapsNotFoundAndPassesAdminIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name       string
		resolveErr error
		wantCode   int
	}{
		{"success", nil, 0},
		{"not found", reconciliationcontract.ErrItemNotFound, 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &reconciliationServiceStub{resolveErr: test.resolveErr}
			handler := NewAdminHandler(service)
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPut, "/reconciliation/items/12/resolve", strings.NewReader(`{"remark":"checked"}`))
			context.Request.Header.Set("Content-Type", "application/json")
			context.Params = gin.Params{{Key: "id", Value: "12"}}
			context.Set("admin_id", uint(9))

			handler.ResolveItem(context)

			if recorder.Code != http.StatusOK {
				t.Fatalf("HTTP status=%d want 200 body=%s", recorder.Code, recorder.Body.String())
			}
			var payload struct {
				StatusCode int `json:"status_code"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload.StatusCode != test.wantCode {
				t.Fatalf("business status=%d want %d body=%s", payload.StatusCode, test.wantCode, recorder.Body.String())
			}
			if service.itemID != 12 || service.adminID != 9 || service.remark != "checked" {
				t.Fatalf("unexpected resolve input: item=%d admin=%d remark=%q", service.itemID, service.adminID, service.remark)
			}
		})
	}
}
