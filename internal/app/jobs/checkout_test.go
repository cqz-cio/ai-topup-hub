package jobs

import (
	"testing"

	"github.com/dujiao-next/internal/app/container"
	jobconsumer "github.com/dujiao-next/internal/app/jobs/consumer"
	"github.com/dujiao-next/internal/config"
)

func TestCheckoutServiceHasNoRecurringPurchaseScheduler(t *testing.T) {
	service, err := NewCheckoutService(&config.QueueConfig{Enabled: true, Host: "127.0.0.1", Port: 6379}, jobconsumer.New(&container.Container{}))
	if err != nil {
		t.Fatal(err)
	}
	if service.Name() != "checkout-worker" || service.scheduler != nil {
		t.Fatal("checkout mode must only run its restricted consumer")
	}
}

func TestCheckoutServiceRequiresQueue(t *testing.T) {
	if _, err := NewCheckoutService(&config.QueueConfig{}, jobconsumer.New(&container.Container{})); err == nil {
		t.Fatal("checkout mode must not accept orders without a functioning queue")
	}
}

func TestStorefrontServiceRunsRecoveryScheduler(t *testing.T) {
	service, err := NewStorefrontService(&config.QueueConfig{Enabled: true, Host: "127.0.0.1", Port: 6379}, jobconsumer.New(&container.Container{}))
	if err != nil {
		t.Fatal(err)
	}
	if service.Name() != "storefront-worker" || service.scheduler == nil {
		t.Fatal("storefront requires paid-order recovery")
	}
}
