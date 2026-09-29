package response

// AppError 统一错误包装
type AppError struct {
	Code    int
	Message string
	Err     error `json:"-"`
}

// WrapError 包装错误
func WrapError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
