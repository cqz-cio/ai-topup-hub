package wallethttp

import (
	"errors"
	"net/http"

	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/platform/http/channelresponse"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

func channelIdentityError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, telegramauthapp.ErrTelegramAuthPayloadInvalid):
		channelresponse.Error(context, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
	case errors.Is(err, userauthapp.ErrInvalidEmail):
		channelresponse.Error(context, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.email_invalid", nil)
	case errors.Is(err, userauthapp.ErrNotFound):
		channelresponse.Error(context, http.StatusNotFound, response.CodeNotFound, "user_not_found", "error.user_not_found", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeInvalid):
		channelresponse.Error(context, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_invalid", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeExpired):
		channelresponse.Error(context, http.StatusBadRequest, response.CodeBadRequest, "verify_code_expired", "error.verify_code_expired", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeAttemptsExceeded):
		channelresponse.Error(context, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_attempts_exceeded", nil)
	case errors.Is(err, userauthapp.ErrUserDisabled):
		channelresponse.Error(context, http.StatusUnauthorized, response.CodeUnauthorized, "user_disabled", "error.user_disabled", nil)
	case errors.Is(err, userauthapp.ErrUserOAuthIdentityExists):
		channelresponse.Error(context, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_bind_conflict", nil)
	case errors.Is(err, userauthapp.ErrUserOAuthAlreadyBound):
		channelresponse.Error(context, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_already_bound", nil)
	default:
		channelresponse.Error(context, http.StatusInternalServerError, response.CodeInternal, "internal_error", "error.internal_error", err)
	}
}
