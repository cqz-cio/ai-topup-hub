package application

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/modules/auditlog/domain"
)

type userLoginRepositoryStub struct {
	created *domain.UserLoginLog
}

func (s *userLoginRepositoryStub) Create(item *domain.UserLoginLog) error {
	s.created = item
	return nil
}

func (s *userLoginRepositoryStub) ListAdmin(contract.UserLoginFilter) ([]domain.UserLoginLog, int64, error) {
	return nil, 0, nil
}

func (s *userLoginRepositoryStub) ListByUser(uint, int, int) ([]domain.UserLoginLog, int64, error) {
	return nil, 0, nil
}

type authzRepositoryStub struct {
	created *domain.AuthzAuditLog
}

func (s *authzRepositoryStub) Create(item *domain.AuthzAuditLog) error {
	s.created = item
	return nil
}

func (s *authzRepositoryStub) ListAdmin(contract.AuthzFilter) ([]domain.AuthzAuditLog, int64, error) {
	return nil, 0, nil
}

func TestUserLoginServiceRecordPreservesNormalizationContract(t *testing.T) {
	repo := &userLoginRepositoryStub{}
	service := NewUserLoginService(repo)

	err := service.Record(UserLoginRecord{
		UserID:      42,
		Email:       " Alice@Example.COM ",
		Status:      "unexpected",
		ClientIP:    " 127.0.0.1 ",
		UserAgent:   " test-agent ",
		LoginSource: "",
		RequestID:   " request-1 ",
	})
	if err != nil {
		t.Fatalf("record login: %v", err)
	}
	if repo.created == nil {
		t.Fatal("expected login log to be created")
	}
	if repo.created.Email != "alice@example.com" {
		t.Fatalf("expected normalized email, got %q", repo.created.Email)
	}
	if repo.created.Status != constants.LoginLogStatusFailed {
		t.Fatalf("expected failed status, got %q", repo.created.Status)
	}
	if repo.created.FailReason != constants.LoginLogFailReasonInternalError {
		t.Fatalf("expected default failure reason, got %q", repo.created.FailReason)
	}
	if repo.created.LoginSource != constants.LoginLogSourceWeb {
		t.Fatalf("expected default web source, got %q", repo.created.LoginSource)
	}
	if repo.created.ClientIP != "127.0.0.1" || repo.created.UserAgent != "test-agent" || repo.created.RequestID != "request-1" {
		t.Fatalf("expected surrounding whitespace to be trimmed: %#v", repo.created)
	}
}

func TestAuthzServiceRecordValidatesAndNormalizes(t *testing.T) {
	repo := &authzRepositoryStub{}
	service := NewAuthzService(repo)

	if err := service.Record(AuthzRecord{Action: "grant"}); err != nil {
		t.Fatalf("ignore invalid audit record: %v", err)
	}
	if repo.created != nil {
		t.Fatal("record without operator must be ignored")
	}

	err := service.Record(AuthzRecord{
		OperatorAdminID:  7,
		OperatorUsername: " root ",
		Action:           " grant ",
		Object:           " /admin/users ",
		Method:           " post ",
		RequestID:        " request-2 ",
	})
	if err != nil {
		t.Fatalf("record authz audit: %v", err)
	}
	if repo.created == nil {
		t.Fatal("expected authz audit log to be created")
	}
	if repo.created.OperatorUsername != "root" || repo.created.Action != "grant" || repo.created.Object != "/admin/users" {
		t.Fatalf("expected text fields to be trimmed: %#v", repo.created)
	}
	if repo.created.Method != "POST" || repo.created.RequestID != "request-2" {
		t.Fatalf("expected method and request id normalization: %#v", repo.created)
	}
}
