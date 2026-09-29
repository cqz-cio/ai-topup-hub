package adminauthzwiring

import (
	"context"
	"errors"
	"fmt"

	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/cache"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	adminauthapp "github.com/dujiao-next/internal/modules/identity/adminauth/application"
	adminauthztransport "github.com/dujiao-next/internal/modules/identity/adminauthorization/transport/http"
	"github.com/dujiao-next/internal/shared/passwordpolicy"
)

type adminAuthzRolePolicyAdapter struct {
	svc *authz.Service
}

func (a adminAuthzRolePolicyAdapter) GetAdminRoles(adminID uint) ([]string, error) {
	return a.svc.GetAdminRoles(adminID)
}

func (a adminAuthzRolePolicyAdapter) GetAdminPolicies(adminID uint) ([]adminauthztransport.Policy, error) {
	policies, err := a.svc.GetAdminPolicies(adminID)
	if err != nil {
		return nil, err
	}
	return mapAuthzPolicies(policies), nil
}

func (a adminAuthzRolePolicyAdapter) ListRoles() ([]string, error) {
	return a.svc.ListRoles()
}

func (a adminAuthzRolePolicyAdapter) EnsureRole(role string) (string, error) {
	return a.svc.EnsureRole(role)
}

func (a adminAuthzRolePolicyAdapter) DeleteRole(role string) error {
	return a.svc.DeleteRole(role)
}

func (a adminAuthzRolePolicyAdapter) GetRolePolicies(role string) ([]adminauthztransport.Policy, error) {
	policies, err := a.svc.GetRolePolicies(role)
	if err != nil {
		return nil, err
	}
	return mapAuthzPolicies(policies), nil
}

func (a adminAuthzRolePolicyAdapter) GrantRolePolicy(role, object, action string) error {
	return a.svc.GrantRolePolicy(role, object, action)
}

func (a adminAuthzRolePolicyAdapter) RevokeRolePolicy(role, object, action string) error {
	return a.svc.RevokeRolePolicy(role, object, action)
}

func (a adminAuthzRolePolicyAdapter) SetAdminRoles(adminID uint, roles []string) error {
	return a.svc.SetAdminRoles(adminID, roles)
}

type adminAuthzDirectoryAdapter struct {
	admins admincontract.Store
}

func (a adminAuthzDirectoryAdapter) List() ([]admindomain.Admin, error) {
	return a.admins.List()
}

func (a adminAuthzDirectoryAdapter) GetByID(id uint) (*admindomain.Admin, error) {
	return a.admins.GetByID(id)
}

func (a adminAuthzDirectoryAdapter) GetByUsername(username string) (*admindomain.Admin, error) {
	return a.admins.GetByUsername(username)
}

func (a adminAuthzDirectoryAdapter) Create(admin *admindomain.Admin) error {
	return a.admins.Create(admin)
}

func (a adminAuthzDirectoryAdapter) Update(admin *admindomain.Admin) error {
	return a.admins.Update(admin)
}

func (a adminAuthzDirectoryAdapter) Delete(id uint) error {
	return a.admins.Delete(id)
}

func (a adminAuthzDirectoryAdapter) Count() (int64, error) {
	return a.admins.Count()
}

type adminAuthzPasswordAdapter struct {
	auth *adminauthapp.Service
}

func (a adminAuthzPasswordAdapter) ValidatePassword(password string) error {
	return mapAdminAuthzTransportError(a.auth.ValidatePassword(password))
}

func (a adminAuthzPasswordAdapter) HashPassword(password string) (string, error) {
	hash, err := a.auth.HashPassword(password)
	if err != nil {
		return "", mapAdminAuthzTransportError(err)
	}
	return hash, nil
}

type adminAuthzAuthStateAdapter struct{}

func (adminAuthzAuthStateAdapter) SetAdminAuthState(ctx context.Context, admin *admindomain.Admin) error {
	return cache.SetAdminAuthState(ctx, cache.BuildAdminAuthState(admin))
}

func (adminAuthzAuthStateAdapter) DelAdminAuthState(ctx context.Context, adminID uint) error {
	return cache.DelAdminAuthState(ctx, adminID)
}

type adminAuthzAuditAdapter struct {
	svc *auditlogapp.AuthzService
}

func (a adminAuthzAuditAdapter) Record(input auditlogapp.AuthzRecord) error {
	if a.svc == nil {
		return nil
	}
	return a.svc.Record(input)
}

func mapAuthzPolicies(policies []authz.Policy) []adminauthztransport.Policy {
	items := make([]adminauthztransport.Policy, 0, len(policies))
	for _, p := range policies {
		items = append(items, adminauthztransport.Policy{
			Subject: p.Subject,
			Object:  p.Object,
			Action:  p.Action,
		})
	}
	return items
}

func mapAdminAuthzTransportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, passwordpolicy.ErrWeak) {
		type keyed interface {
			Key() string
			Args() []interface{}
		}
		var k keyed
		if errors.As(err, &k) {
			return adminauthztransport.NewWeakPasswordError(k.Key(), k.Args()...)
		}
		if perr, ok := err.(keyed); ok {
			return adminauthztransport.NewWeakPasswordError(perr.Key(), perr.Args()...)
		}
		return fmt.Errorf("%w: %v", adminauthztransport.ErrWeakPassword, err)
	}
	return err
}
