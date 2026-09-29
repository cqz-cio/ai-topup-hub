package contract

// ValidationError 表示上传内容不符合业务校验规则。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// UploadValidationError 供 HTTP 边界识别可安全展示的校验错误。
func (e *ValidationError) UploadValidationError() {}
