package architecture

import (
	"path/filepath"
	"testing"
)

func TestAffiliateApplicationOwnsUseCasesAndContracts(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "affiliate")
	applicationRoot := filepath.Join(moduleRoot, "application")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "attribution.go"), []string{"ResolveOrderAffiliateSnapshot", "TrackClick"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "commission.go"), []string{"HandleOrderPaid", "ConfirmDueCommissions", "HandleOrderCanceled", "HandleOrderRefunded"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "profile.go"), []string{"UpdateAffiliateProfileStatus", "BatchUpdateAffiliateProfileStatus", "OpenAffiliate"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "query.go"), []string{"GetUserDashboard", "ListUserCommissions", "ListUserWithdraws", "ListAdminUsers", "ListAdminCommissions", "ListAdminWithdraws"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "withdraw.go"), []string{"ApplyWithdraw", "ReviewWithdraw"})
	assertDirectoryGoFileBudget(t, applicationRoot, 8)

	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "handler.go"), []string{"Handler"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"AdminHandler"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "channel_handler.go"), []string{"ChannelHandler"})
	assertDirectoryGoFileBudget(t, transportRoot, 4)
}
