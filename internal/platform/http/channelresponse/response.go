package channelresponse

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func Success(c *gin.Context, data interface{}) {
	response.ChannelSuccess(c, data)
}

func Error(c *gin.Context, httpCode, code int, errorCode, key string, err error) {
	locale := i18n.ResolveLocale(c)
	message := i18n.T(locale, key)
	if err != nil {
		ginutil.RequestLog(c).Errorw(
			"channel_handler_error",
			"http_code", httpCode,
			"code", code,
			"error_code", errorCode,
			"message", message,
			"error", err,
		)
	}
	response.ChannelError(c, httpCode, code, message, errorCode)
}

func BindError(c *gin.Context, err error) {
	locale := i18n.ResolveLocale(c)
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		details := make([]string, 0, len(validationErrors))
		for _, fieldError := range validationErrors {
			details = append(details, formatFieldError(locale, fieldError))
		}
		message := strings.Join(details, "; ")
		ginutil.RequestLog(c).Warnw("channel_bind_validation_error", "details", message, "error", err)
		response.ChannelError(c, http.StatusBadRequest, response.CodeBadRequest, message, "validation_error")
		return
	}

	message := i18n.T(locale, "error.bad_request")
	ginutil.RequestLog(c).Warnw("channel_bind_error", "message", message, "error", err)
	response.ChannelError(c, http.StatusBadRequest, response.CodeBadRequest, message, "validation_error")
}

func UserIDValue(primary, legacy string) string {
	if value := strings.TrimSpace(primary); value != "" {
		return value
	}
	return strings.TrimSpace(legacy)
}

func formatFieldError(locale string, fieldError validator.FieldError) string {
	field := fieldError.Field()
	tag := fieldError.Tag()
	param := fieldError.Param()

	customKey := "validation." + field + "." + tag
	if message := i18n.T(locale, customKey); message != customKey {
		return message
	}
	ruleKey := "validation.rule." + tag
	if message := i18n.T(locale, ruleKey); message != ruleKey {
		if param != "" {
			return field + ": " + i18n.Sprintf(locale, ruleKey, param)
		}
		return field + ": " + message
	}
	if param != "" {
		return field + ": " + tag + "=" + param
	}
	return field + ": " + tag
}
