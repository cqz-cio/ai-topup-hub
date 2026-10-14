package integrationtest

import (
	"github.com/dujiao-next/internal/constants"
	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/testkit/memorysettings"
	"testing"
)

func TestDashboardAvailabilityTracksSettingsAndProfile(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, status := range []string{"", constants.AffiliateProfileStatusActive, constants.AffiliateProfileStatusDisabled} {
			name := status
			if enabled {
				name += "_enabled"
			} else {
				name += "_closed"
			}
			t.Run(name, func(t *testing.T) {
				_, db, store := setupAffiliateServiceTest(t)
				if err := db.AutoMigrate(&affiliatedomain.Commission{}, &affiliatedomain.WithdrawRequest{}); err != nil {
					t.Fatal(err)
				}
				user := createAffiliateTestUser(t, db, "availability@example.test")
				if status != "" {
					createAffiliateTestProfile(t, db, user.ID, "AVAIL001", status)
				}
				settings := settingsapp.NewService(memorysettings.New())
				if _, err := settings.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{Enabled: enabled}); err != nil {
					t.Fatal(err)
				}
				svc := affiliateapp.NewService(store, userstore.New(db), nil, nil, settings)
				dashboard, err := svc.GetUserDashboard(user.ID)
				if err != nil {
					t.Fatal(err)
				}
				if dashboard.Enabled != enabled || dashboard.Opened != (status != "") || dashboard.CanOpen != (enabled && status == "") || dashboard.CanWithdraw != (enabled && status == constants.AffiliateProfileStatusActive) || dashboard.ProfileStatus != status {
					t.Fatalf("incorrect availability: %+v", dashboard)
				}
				if !enabled {
					if _, err := svc.OpenAffiliate(user.ID); err != affiliateapp.ErrDisabled {
						t.Fatalf("closed feature allowed enrollment: %v", err)
					}
				}
			})
		}
	}
}
