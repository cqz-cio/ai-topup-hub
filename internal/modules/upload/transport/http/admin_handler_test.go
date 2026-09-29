package uploadhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	uploadcontract "github.com/dujiao-next/internal/modules/upload/contract"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type recordingMediaRecorder struct {
	ctx    context.Context
	result contentapp.UploadResult
	scene  string
	media  *contentdomain.Media
	err    error
}

var _ MediaRecorder = (*recordingMediaRecorder)(nil)

func (r *recordingMediaRecorder) RecordMedia(ctx context.Context, result contentapp.UploadResult, scene string) (*contentdomain.Media, error) {
	r.ctx = ctx
	r.result = result
	r.scene = scene
	return r.media, r.err
}

type fakeValidationError struct {
	msg string
}

func (e *fakeValidationError) Error() string          { return e.msg }
func (e *fakeValidationError) UploadValidationError() {}

type fakeUploader struct {
	result *uploadcontract.Result
	err    error
	file   *multipart.FileHeader
	scene  string
}

func (u *fakeUploader) SaveFileWithMeta(file *multipart.FileHeader, scene string) (*uploadcontract.Result, error) {
	u.file = file
	u.scene = scene
	return u.result, u.err
}

func TestUploadFileReturnsValidationMessageForOversizedFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "oversized.png")
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err := part.Write([]byte("oversized")); err != nil {
		t.Fatalf("write form file failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}

	handler := NewAdminHandler(&fakeUploader{
		err: &fakeValidationError{msg: "文件大小超过限制: 9 / 4 bytes"},
	}, &recordingMediaRecorder{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/upload", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	handler.UploadFile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("http status want 200 got %d body=%s", w.Code, w.Body.String())
	}
	var got response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
	}
	if got.StatusCode != response.CodeBadRequest {
		t.Fatalf("business status want 400 got %d body=%s", got.StatusCode, w.Body.String())
	}
	if !strings.Contains(got.Msg, "文件大小超过限制") {
		t.Fatalf("message should include validation reason, got %q", got.Msg)
	}
}

func TestUploadFileUsesInjectedMediaRecorderAndRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "asset.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("content asset")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.WriteField("scene", "common"); err != nil {
		t.Fatalf("write scene: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	media := &contentdomain.Media{}
	media.ID = 91
	recorder := &recordingMediaRecorder{media: media}
	uploader := &fakeUploader{result: &uploadcontract.Result{
		URL:      "/uploads/common/asset.txt",
		Filename: "asset.txt",
		MimeType: "text/plain",
		Size:     13,
	}}
	handler := NewAdminHandler(uploader, recorder)

	type contextKey struct{}
	requestContext := context.WithValue(context.Background(), contextKey{}, "upload-request")
	request := httptest.NewRequest(http.MethodPost, "/admin/upload", body).WithContext(requestContext)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = request

	handler.UploadFile(c)

	if uploader.scene != "common" || uploader.file == nil || uploader.file.Filename != "asset.txt" {
		t.Fatalf("uploader input mismatch: scene=%q file=%#v", uploader.scene, uploader.file)
	}
	if recorder.ctx.Value(contextKey{}) != "upload-request" {
		t.Fatal("request context was not propagated")
	}
	if recorder.scene != "common" || recorder.result.Filename != "asset.txt" || recorder.result.URL == "" {
		t.Fatalf("media recorder input mismatch: scene=%q result=%#v", recorder.scene, recorder.result)
	}
	var got response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := got.Data.(map[string]interface{})
	if !ok || uint(data["media_id"].(float64)) != media.ID {
		t.Fatalf("response media_id mismatch: %#v", got.Data)
	}
}

func TestUploadFileKeepsUploadSuccessWhenMediaRecordingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "asset.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("content asset")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	recorder := &recordingMediaRecorder{err: context.Canceled}
	handler := NewAdminHandler(&fakeUploader{result: &uploadcontract.Result{
		URL:      "/uploads/common/asset.txt",
		Filename: "asset.txt",
		Size:     13,
	}}, recorder)
	request := httptest.NewRequest(http.MethodPost, "/admin/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = request

	handler.UploadFile(c)

	var got response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.StatusCode != response.CodeOK {
		t.Fatalf("media recording failure changed upload result: %#v", got)
	}
	data, ok := got.Data.(map[string]interface{})
	if !ok || uint(data["media_id"].(float64)) != 0 || data["url"] == "" {
		t.Fatalf("best-effort response mismatch: %#v", got.Data)
	}
}

func TestIsUploadValidationErrorRecognizesMarker(t *testing.T) {
	if !isUploadValidationError(&fakeValidationError{msg: "bad"}) {
		t.Fatal("expected validation marker to be recognized")
	}
	if isUploadValidationError(errors.New("other")) {
		t.Fatal("unexpected match for plain error")
	}
}
