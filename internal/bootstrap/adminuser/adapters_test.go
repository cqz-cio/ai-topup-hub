package adminuserwiring

import (
	"errors"
	"testing"

	adminusertransport "github.com/dujiao-next/internal/modules/identity/user/transport/http/admin"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
)

func TestMapAdminUserTransportErrorMapsGoogleUnbindErrors(t *testing.T) {
	tests := []struct {
		name   string
		source error
		target error
	}{
		{name: "not bound", source: userauthapp.ErrUserOAuthNotBound, target: adminusertransport.ErrUserOAuthNotBound},
		{name: "would lock account", source: userauthapp.ErrGoogleUnbindLocked, target: adminusertransport.ErrGoogleUnbindLocked},
	}

	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			got := mapAdminUserTransportError(item.source)
			if !errors.Is(got, item.target) {
				t.Fatalf("mapped error = %v, want errors.Is(_, %v)", got, item.target)
			}
		})
	}
}
