package application_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	localfilestore "github.com/dujiao-next/internal/modules/content/infrastructure/filestore/local"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupMediaServiceTest(t *testing.T) (*contentapp.MediaService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:media_service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&contentdomain.Media{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	return contentapp.NewMediaService(
		gormstore.NewMediaStore(db),
		localfilestore.New(),
		nil,
	), db
}

func createMediaFile(t *testing.T, path string) {
	t.Helper()
	diskPath := filepath.FromSlash(path)
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
		t.Fatalf("create media dir failed: %v", err)
	}
	if err := os.WriteFile(diskPath, []byte("image"), 0644); err != nil {
		t.Fatalf("write media file failed: %v", err)
	}
}

func TestMediaServiceBatchDeleteDeletesFilesAndReportsFailures(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)

	svc, db := setupMediaServiceTest(t)
	createMediaFile(t, "uploads/common/first.png")
	createMediaFile(t, "uploads/common/second.png")

	first := contentdomain.Media{
		Name:     "first",
		Filename: "first.png",
		Path:     "/uploads/common/first.png",
		MimeType: "image/png",
		Size:     5,
		Scene:    "common",
	}
	second := contentdomain.Media{
		Name:     "second",
		Filename: "second.png",
		Path:     "/uploads/common/second.png",
		MimeType: "image/png",
		Size:     5,
		Scene:    "common",
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first media failed: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second media failed: %v", err)
	}

	successCount, failedIDs := svc.BatchDelete(context.Background(), []uint{first.ID, 9999, second.ID})

	if successCount != 2 {
		t.Fatalf("success count want 2 got %d", successCount)
	}
	if len(failedIDs) != 1 || failedIDs[0] != 9999 {
		t.Fatalf("failed IDs want [9999] got %#v", failedIDs)
	}
	if _, err := os.Stat(filepath.FromSlash("uploads/common/first.png")); !os.IsNotExist(err) {
		t.Fatalf("expected first file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.FromSlash("uploads/common/second.png")); !os.IsNotExist(err) {
		t.Fatalf("expected second file to be removed, stat err=%v", err)
	}

	var remaining int64
	if err := db.Model(&contentdomain.Media{}).Where("deleted_at IS NULL").Count(&remaining).Error; err != nil {
		t.Fatalf("count media failed: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected selected media to be soft-deleted, remaining=%d", remaining)
	}
}

func TestMediaServiceRecordMediaDeduplicatesByPath(t *testing.T) {
	svc, db := setupMediaServiceTest(t)
	result := contentapp.UploadResult{
		URL:      "/uploads/common/asset.final.png",
		Filename: "asset.final.png",
		MimeType: "image/png",
		Size:     128,
		Width:    20,
		Height:   10,
	}

	first, err := svc.RecordMedia(context.Background(), result, "common")
	if err != nil {
		t.Fatalf("record media: %v", err)
	}
	if first.Name != "asset.final" || first.Path != result.URL || first.Scene != "common" {
		t.Fatalf("recorded metadata mismatch: %#v", first)
	}

	duplicate := result
	duplicate.Filename = "changed.png"
	second, err := svc.RecordMedia(context.Background(), duplicate, "product")
	if err != nil {
		t.Fatalf("record duplicate media: %v", err)
	}
	if second.ID != first.ID || second.Filename != result.Filename || second.Scene != "common" {
		t.Fatalf("duplicate path should return original metadata: first=%#v second=%#v", first, second)
	}

	var count int64
	if err := db.Model(&contentdomain.Media{}).Where("deleted_at IS NULL").Count(&count).Error; err != nil {
		t.Fatalf("count media: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one row after duplicate record, got %d", count)
	}
}

func TestMediaServiceRecordLocalFileCapturesMetadataAndDeduplicates(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	svc, db := setupMediaServiceTest(t)

	path := filepath.FromSlash("uploads/upstream/manual.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create local media directory: %v", err)
	}
	content := []byte("hello media")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write local media: %v", err)
	}

	svc.RecordLocalFile(context.Background(), "/uploads/upstream/manual.txt", "upstream")
	svc.RecordLocalFile(context.Background(), "/uploads/upstream/manual.txt", "other-scene")
	svc.RecordLocalFile(context.Background(), "/uploads/upstream/missing.txt", "upstream")

	var media contentdomain.Media
	if err := db.Where("path = ?", "/uploads/upstream/manual.txt").First(&media).Error; err != nil {
		t.Fatalf("load recorded local media: %v", err)
	}
	if media.Name != "manual" || media.Filename != "manual.txt" || media.Scene != "upstream" || media.Size != int64(len(content)) {
		t.Fatalf("local media metadata mismatch: %#v", media)
	}
	if media.MimeType != "text/plain; charset=utf-8" || media.Width != 0 || media.Height != 0 {
		t.Fatalf("local media detection mismatch: %#v", media)
	}

	var count int64
	if err := db.Model(&contentdomain.Media{}).Where("deleted_at IS NULL").Count(&count).Error; err != nil {
		t.Fatalf("count local media: %v", err)
	}
	if count != 1 {
		t.Fatalf("duplicate and missing local paths must not add rows, got %d", count)
	}
}

func TestMediaServiceRenameTrimsAndValidatesName(t *testing.T) {
	svc, db := setupMediaServiceTest(t)
	media := contentdomain.Media{
		Name:     "before",
		Filename: "asset.png",
		Path:     "/uploads/common/asset.png",
		MimeType: "image/png",
		Size:     5,
		Scene:    "common",
	}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("create media: %v", err)
	}

	if err := svc.Rename(context.Background(), media.ID, "   "); err != contentcontract.ErrMediaNameEmpty {
		t.Fatalf("blank name should return domaincontent.ErrMediaNameEmpty, got %v", err)
	}
	if err := svc.Rename(context.Background(), 9999, "missing"); err != contentcontract.ErrMediaNotFound {
		t.Fatalf("missing media should return domaincontent.ErrMediaNotFound, got %v", err)
	}
	if err := svc.Rename(context.Background(), media.ID, "  after  "); err != nil {
		t.Fatalf("rename media: %v", err)
	}

	var updated contentdomain.Media
	if err := db.First(&updated, media.ID).Error; err != nil {
		t.Fatalf("reload renamed media: %v", err)
	}
	if updated.Name != "after" {
		t.Fatalf("renamed media should be trimmed, got %q", updated.Name)
	}
}

func TestMediaServiceDeleteKeepsBestEffortFileFailureSemantics(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	svc, db := setupMediaServiceTest(t)

	directoryPath := filepath.FromSlash("uploads/common/not-a-file")
	if err := os.MkdirAll(directoryPath, 0o755); err != nil {
		t.Fatalf("create non-empty directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directoryPath, "child.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write directory child: %v", err)
	}

	media := contentdomain.Media{
		Name:     "directory",
		Filename: "not-a-file",
		Path:     "/uploads/common/not-a-file",
		MimeType: "application/octet-stream",
		Size:     0,
		Scene:    "common",
	}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("create media: %v", err)
	}

	if err := svc.Delete(context.Background(), media.ID); err != nil {
		t.Fatalf("physical file failure should remain best effort, got %v", err)
	}
	if _, err := os.Stat(directoryPath); err != nil {
		t.Fatalf("failed physical removal target should remain: %v", err)
	}

	var visible int64
	if err := db.Model(&contentdomain.Media{}).Where("id = ? AND deleted_at IS NULL", media.ID).Count(&visible).Error; err != nil {
		t.Fatalf("count visible media: %v", err)
	}
	if visible != 0 {
		t.Fatalf("media should be soft-deleted even when physical removal fails, visible=%d", visible)
	}
	var unscoped int64
	if err := db.Model(&contentdomain.Media{}).Where("id = ?", media.ID).Count(&unscoped).Error; err != nil {
		t.Fatalf("count unscoped media: %v", err)
	}
	if unscoped != 1 {
		t.Fatalf("soft-deleted media row should remain unscoped, count=%d", unscoped)
	}
}
