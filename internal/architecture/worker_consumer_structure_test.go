package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkerConsumerIsSplitByJobDomain(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	jobsDirectory := filepath.Join(repositoryRoot, "internal", "app", "jobs")
	consumerDirectory := filepath.Join(jobsDirectory, "consumer")
	legacyPath := filepath.Join(repositoryRoot, "internal", "worker")
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatalf("legacy worker root must stay deleted")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy worker root: %v", err)
	}

	expectedOwner := map[string]string{
		"New":                               "consumer_core.go",
		"Register":                          "consumer_core.go",
		"handleOrderStatusEmail":            "consumer_order.go",
		"handleOrderAutoFulfill":            "consumer_order.go",
		"handleOrderTimeoutCancel":          "consumer_order.go",
		"handleWalletRechargeExpire":        "consumer_order.go",
		"buildOrderInstructionsEmailText":   "consumer_order.go",
		"localizedInstructionsText":         "consumer_order.go",
		"buildOrderFulfillmentEmailPayload": "consumer_order.go",
		"handleNotificationDispatch":        "consumer_maintenance.go",
		"handleAffiliateConfirmCommissions": "consumer_maintenance.go",
		"handleResellerConfirmLedger":       "consumer_maintenance.go",
		"handleReconciliationRun":           "consumer_maintenance.go",
		"handleUpstreamSyncStock":           "consumer_upstream.go",
		"handleProcurementSubmit":           "consumer_upstream.go",
		"handleProcurementPollStatus":       "consumer_upstream.go",
		"handleProcurementSyncAccepted":     "consumer_upstream.go",
		"handleDownstreamCallback":          "consumer_upstream.go",
		"handleBotNotify":                   "consumer_bot.go",
		"buildBotNotifyRequestURL":          "consumer_bot.go",
		"handleTelegramBroadcast":           "consumer_bot.go",
	}

	files := []string{
		"consumer_core.go",
		"consumer_order.go",
		"consumer_maintenance.go",
		"consumer_upstream.go",
		"consumer_bot.go",
	}
	actualOwners := make(map[string][]string, len(expectedOwner))
	consumerTypeOwners := make([]string, 0, 1)
	for _, file := range files {
		parsed := parseProductionGoFile(t, filepath.Join(consumerDirectory, file))
		for _, function := range declaredFunctionNames(parsed) {
			if _, tracked := expectedOwner[function]; tracked {
				actualOwners[function] = append(actualOwners[function], file)
			}
		}
		for _, typeName := range declaredTypeNames(parsed) {
			if typeName == "Consumer" {
				consumerTypeOwners = append(consumerTypeOwners, file)
			}
		}
	}

	for function, wantFile := range expectedOwner {
		gotFiles := actualOwners[function]
		if len(gotFiles) != 1 || gotFiles[0] != wantFile {
			t.Errorf("%s ownership mismatch: want [%s], got %v", function, wantFile, gotFiles)
		}
	}
	if len(consumerTypeOwners) != 1 || consumerTypeOwners[0] != "consumer_core.go" {
		t.Errorf("Consumer ownership mismatch: want [consumer_core.go], got %v", consumerTypeOwners)
	}

	assertFileDeclaresTypes(t, filepath.Join(jobsDirectory, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(jobsDirectory, "service.go"), []string{"NewService"})
}
