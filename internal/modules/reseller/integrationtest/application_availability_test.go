package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dujiao-next/internal/config"
	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"
)

func TestApplicationAdmissionMatchesAdminFeatureStatus(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, selfApply := range []bool{false, true} {
			t.Run(fmt.Sprintf("enabled_%t_self_apply_%t", enabled, selfApply), func(t *testing.T) {
				db := openResellerManagementServiceTestDB(t)
				user := seedResellerManagementUser(t, db, "availability@example.test")
				cfg := config.ResellerConfig{Enabled: enabled, SelfApplyEnabled: selfApply}
				svc := NewResellerManagementService(resellergormstore.New(db), cfg)
				overview, err := resellermodule.NewOperationsService(nil, cfg).GetOverview(context.Background(), reportingdomain.Query{Range: "today"})
				if err != nil {
					t.Fatal(err)
				}
				_, _, canApply, err := svc.GetUserManagementSnapshot(user.ID)
				if err != nil || overview.Feature.Enabled != enabled || overview.Feature.ApplicationsOpen != canApply || canApply != (enabled && selfApply) {
					t.Fatalf("inconsistent admission: feature=%+v canApply=%t err=%v", overview.Feature, canApply, err)
				}
				profile, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "test"})
				if !canApply {
					if !errors.Is(err, ErrResellerApplyDisabled) {
						t.Fatalf("closed application accepted: %v", err)
					}
					var count int64
					db.Model(&resellerdomain.Profile{}).Count(&count)
					if count != 0 {
						t.Fatalf("closed application created %d profiles", count)
					}
					return
				}
				if err != nil || profile.Status != resellerdomain.ProfileStatusPendingReview {
					t.Fatalf("open application did not enter review: %v", err)
				}
				_, _, canApply, err = svc.GetUserManagementSnapshot(user.ID)
				if err != nil || canApply {
					t.Fatalf("pending profile should not offer another application: %v", err)
				}
			})
		}
	}
}
