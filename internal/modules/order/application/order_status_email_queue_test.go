package application

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/testkit/memorysettings"

	orderqueueadapter "github.com/dujiao-next/internal/modules/order/infrastructure/queueadapter"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type orderStatusEmailOrderRepoStub struct {
	receiver string
	err      error
}

func (s orderStatusEmailOrderRepoStub) ResolveReceiverEmailByOrderID(_ uint) (string, error) {
	return s.receiver, s.err
}

func TestEnqueueOrderStatusEmailTaskIfEligibleSkipTelegramPlaceholder(t *testing.T) {
	queueClient, err := queue.NewClient(nil)
	if err != nil {
		t.Fatalf("new queue client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = queueClient.Close()
	})

	skipped, err := EnqueueStatusEmailTaskIfEligible(
		orderStatusEmailOrderRepoStub{receiver: "telegram_123@login.local"},
		orderqueueadapter.New(queueClient),
		nil,
		config.EmailConfig{},
		101,
		"paid",
	)
	if err != nil {
		t.Fatalf("enqueue helper returned error: %v", err)
	}
	if !skipped {
		t.Fatalf("expected task skipped for telegram placeholder email")
	}
}

func TestEnqueueOrderStatusEmailTaskIfEligibleSkipEmptyReceiver(t *testing.T) {
	queueClient, err := queue.NewClient(nil)
	if err != nil {
		t.Fatalf("new queue client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = queueClient.Close()
	})

	skipped, err := EnqueueStatusEmailTaskIfEligible(
		orderStatusEmailOrderRepoStub{receiver: "   "},
		orderqueueadapter.New(queueClient),
		nil,
		config.EmailConfig{},
		102,
		"paid",
	)
	if err != nil {
		t.Fatalf("enqueue helper returned error: %v", err)
	}
	if !skipped {
		t.Fatalf("expected task skipped for empty receiver email")
	}
}

func TestEnqueueOrderStatusEmailTaskIfEligibleEnqueueNormalReceiver(t *testing.T) {
	queueClient, err := queue.NewClient(nil)
	if err != nil {
		t.Fatalf("new queue client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = queueClient.Close()
	})

	skipped, err := EnqueueStatusEmailTaskIfEligible(
		orderStatusEmailOrderRepoStub{receiver: "buyer@example.com"},
		orderqueueadapter.New(queueClient),
		nil,
		config.EmailConfig{},
		103,
		"paid",
	)
	if err != nil {
		t.Fatalf("enqueue helper returned error: %v", err)
	}
	if skipped {
		t.Fatalf("expected task enqueued for normal receiver email")
	}
}

func TestEnqueueOrderStatusEmailTaskIfEligibleFallbackWhenLookupFailed(t *testing.T) {
	queueClient, err := queue.NewClient(nil)
	if err != nil {
		t.Fatalf("new queue client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = queueClient.Close()
	})

	skipped, err := EnqueueStatusEmailTaskIfEligible(
		orderStatusEmailOrderRepoStub{err: errors.New("lookup failed")},
		orderqueueadapter.New(queueClient),
		nil,
		config.EmailConfig{},
		104,
		"paid",
	)
	if err != nil {
		t.Fatalf("enqueue helper returned error: %v", err)
	}
	if skipped {
		t.Fatalf("expected fallback enqueue when receiver lookup failed")
	}
}

func TestEnqueueOrderStatusEmailTaskIfEligibleSkipWhenSMTPDisabled(t *testing.T) {
	queueClient, err := queue.NewClient(nil)
	if err != nil {
		t.Fatalf("new queue client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = queueClient.Close()
	})

	repo := memorysettings.New()
	repo.Values[constants.SettingKeySMTPConfig] = jsonmap.JSON{
		"enabled": false,
	}

	skipped, err := EnqueueStatusEmailTaskIfEligible(
		orderStatusEmailOrderRepoStub{receiver: "buyer@example.com"},
		orderqueueadapter.New(queueClient),
		settingsapp.NewService(repo),
		config.EmailConfig{Enabled: true},
		105,
		"paid",
	)
	if err != nil {
		t.Fatalf("enqueue helper returned error: %v", err)
	}
	if !skipped {
		t.Fatalf("expected task skipped when smtp disabled")
	}
}
