package ginutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// RequestLog 提供携带 request_id 的日志实例。
func RequestLog(c *gin.Context) *zap.SugaredLogger {
	if c == nil {
		return logger.S()
	}
	if requestID, ok := c.Get("request_id"); ok {
		if id, ok := requestID.(string); ok && id != "" {
			return logger.SW("request_id", id)
		}
	}
	return logger.S()
}

// RespondError 返回国际化错误响应，并在有原始错误时记录日志。
func RespondError(c *gin.Context, code int, key string, err error) {
	locale := i18n.ResolveLocale(c)
	msg := i18n.T(locale, key)
	appErr := response.WrapError(code, msg, err)
	if err != nil {
		RequestLog(c).Errorw("handler_error",
			"code", appErr.Code,
			"message", appErr.Message,
			"error", err,
		)
	}
	response.Error(c, appErr.Code, appErr.Message)
}

// RespondBindError 处理 ShouldBindJSON 等绑定错误，返回具体的字段校验提示。
func RespondBindError(c *gin.Context, err error) {
	locale := i18n.ResolveLocale(c)

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		details := make([]string, 0, len(ve))
		for _, fe := range ve {
			details = append(details, formatFieldError(locale, fe))
		}
		msg := strings.Join(details, "; ")
		RequestLog(c).Warnw("bind_validation_error", "details", msg, "error", err)
		response.Error(c, response.CodeBadRequest, msg)
		return
	}

	// 非校验错误（JSON 格式错误等），回退到通用提示
	msg := i18n.T(locale, "error.bad_request")
	RequestLog(c).Warnw("bind_error", "message", msg, "error", err)
	response.Error(c, response.CodeBadRequest, msg)
}

// formatFieldError 将单个字段校验错误格式化为可读消息。
func formatFieldError(locale string, fe validator.FieldError) string {
	field := fe.Field()
	tag := fe.Tag()
	param := fe.Param()

	// 尝试查找 i18n 中的自定义翻译: validation.Field.tag
	customKey := fmt.Sprintf("validation.%s.%s", field, tag)
	if msg := i18n.T(locale, customKey); msg != customKey {
		return msg
	}

	// 通用规则翻译
	ruleKey := "validation.rule." + tag
	if ruleMsg := i18n.T(locale, ruleKey); ruleMsg != ruleKey {
		if param != "" {
			return fmt.Sprintf("%s: %s", field, fmt.Sprintf(ruleMsg, param))
		}
		return fmt.Sprintf("%s: %s", field, ruleMsg)
	}

	// 最终回退
	if param != "" {
		return fmt.Sprintf("%s: %s=%s", field, tag, param)
	}
	return fmt.Sprintf("%s: %s", field, tag)
}

// RespondErrorWithMsg 返回自定义消息错误响应，并在有原始错误时记录日志。
func RespondErrorWithMsg(c *gin.Context, code int, msg string, err error) {
	appErr := response.WrapError(code, msg, err)
	if err != nil {
		RequestLog(c).Errorw("handler_error",
			"code", appErr.Code,
			"message", appErr.Message,
			"error", err,
		)
	}
	response.Error(c, appErr.Code, appErr.Message)
}
