package uploadhttp

import (
	"context"
	"errors"
	"mime/multipart"

	"github.com/dujiao-next/internal/logger"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	uploadcontract "github.com/dujiao-next/internal/modules/upload/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// FileUploader 是文件落盘端口。
type FileUploader interface {
	SaveFileWithMeta(file *multipart.FileHeader, scene string) (*uploadcontract.Result, error)
}

// MediaRecorder 是上传 HTTP 消费方所需的最小 Content 写入接口。
type MediaRecorder interface {
	RecordMedia(ctx context.Context, result contentapp.UploadResult, scene string) (*contentdomain.Media, error)
}

// AdminHandler 处理后台文件上传请求。
type AdminHandler struct {
	uploader FileUploader
	media    MediaRecorder
}

func NewAdminHandler(uploader FileUploader, media MediaRecorder) *AdminHandler {
	if uploader == nil || media == nil {
		panic("upload admin handler: required dependency is nil")
	}
	return &AdminHandler{uploader: uploader, media: media}
}

// UploadFile 文件上传
func (h *AdminHandler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.file_missing", nil)
		return
	}
	scene := c.DefaultPostForm("scene", "common")

	result, err := h.uploader.SaveFileWithMeta(file, scene)
	if err != nil {
		if isUploadValidationError(err) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.upload_failed", err)
		return
	}

	var mediaID uint
	media, err := h.media.RecordMedia(c.Request.Context(), contentapp.UploadResult{
		URL:      result.URL,
		Filename: result.Filename,
		MimeType: result.MimeType,
		Size:     result.Size,
		Width:    result.Width,
		Height:   result.Height,
	}, scene)
	if err != nil {
		logger.Warnw("upload_record_media_failed", "error", err, "url", result.URL)
	} else if media != nil {
		mediaID = media.ID
	}

	response.Success(c, gin.H{
		"url":      result.URL,
		"filename": result.Filename,
		"size":     result.Size,
		"media_id": mediaID,
	})
}

func isUploadValidationError(err error) bool {
	var marker interface{ UploadValidationError() }
	return errors.As(err, &marker)
}
