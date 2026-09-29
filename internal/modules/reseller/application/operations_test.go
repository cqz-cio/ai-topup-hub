package application

import (
	"context"
	"strings"
	"testing"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	"github.com/shopspring/decimal"
)

type operationsStoreStub struct {
	overview resellercontract.OperationsOverviewRow
	finance  resellercontract.OperationsFinanceRowSet
}

func (s operationsStoreStub) GetOverview(startAt, endAt time.Time) (resellercontract.OperationsOverviewRow, error) {
	return s.overview, nil
}

func (s operationsStoreStub) GetFinance(startAt, endAt time.Time) (resellercontract.OperationsFinanceRowSet, error) {
	return s.finance, nil
}

func TestResellerOperationsServiceOverviewBuildsAlertsAndFormatsAverage(t *testing.T) {
	svc := NewOperationsService(operationsStoreStub{
		overview: resellercontract.OperationsOverviewRow{
			Lifecycle: resellercontract.OperationsLifecycleRow{
				ProfilesPendingReview:           2,
				DomainsPendingReview:            1,
				ActiveProfilesWithoutSiteConfig: 3,
			},
			Orders: resellercontract.OperationsOrdersRow{
				OrdersTotal:               10,
				PaidOrders:                6,
				ActiveResellersWithOrders: 4,
				SelfDealingBlockedOrders:  1,
			},
		},
	})
	resp, err := svc.GetOverview(context.Background(), reportingdomain.Query{Range: "today", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("GetOverview failed: %v", err)
	}
	if resp.Orders.AveragePaidOrdersPerActiveReseller != "1.50" {
		t.Fatalf("unexpected average: %s", resp.Orders.AveragePaidOrdersPerActiveReseller)
	}
	if len(resp.Alerts) != 4 {
		t.Fatalf("expected four alerts, got %+v", resp.Alerts)
	}
	if !strings.HasSuffix(resp.To, "T23:59:59+08:00") {
		t.Fatalf("expected inclusive end-of-day to timestamp, got %s", resp.To)
	}
}

func TestResellerOperationsServiceFinanceFormatsCurrencyRows(t *testing.T) {
	svc := NewOperationsService(operationsStoreStub{
		finance: resellercontract.OperationsFinanceRowSet{
			PeriodCurrencyRows: []resellercontract.OperationsPeriodCurrencyRow{{
				Currency:       "usd",
				GMVPaid:        decimal.RequireFromString("120"),
				ProfitEarned:   decimal.RequireFromString("30"),
				RefundDeducted: decimal.RequireFromString("4"),
			}},
			CurrentCurrencyRows: []resellercontract.OperationsCurrentCurrencyRow{{
				Currency:              "usd",
				AvailableBalance:      decimal.RequireFromString("26"),
				PendingWithdrawAmount: decimal.RequireFromString("8"),
				PendingWithdrawCount:  1,
			}},
		},
	})
	resp, err := svc.GetFinance(context.Background(), reportingdomain.Query{Range: "today", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("GetFinance failed: %v", err)
	}
	if resp.PeriodCurrencyRows[0].Currency != "USD" || resp.PeriodCurrencyRows[0].GMVPaid != "120.00" {
		t.Fatalf("unexpected period row: %+v", resp.PeriodCurrencyRows[0])
	}
	if resp.CurrentCurrencyRows[0].PendingWithdrawAmount != "8.00" {
		t.Fatalf("unexpected current row: %+v", resp.CurrentCurrencyRows[0])
	}
}

var _ resellercontract.OperationsStore = operationsStoreStub{}
