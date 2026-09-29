package application

import (
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
)

// UploadResult 是 Media 用例记录上传元数据所需的最小输入。
type UploadResult struct {
	URL      string
	Filename string
	MimeType string
	Size     int64
	Width    int
	Height   int
}

// MediaListQuery 描述素材列表查询。
type MediaListQuery struct {
	Scene    string
	Search   string
	Page     int
	PageSize int
}

// MediaService 实现素材元数据和本地文件清理用例。
type MediaService struct {
	store  contract.MediaStore
	files  contract.FileStore
	logger contract.WarningLogger
}

// NewMediaService 创建素材用例服务。
func NewMediaService(store contract.MediaStore, files contract.FileStore, logger contract.WarningLogger) *MediaService {
	return &MediaService{store: store, files: files, logger: logger}
}

// List 获取素材列表。
func (s *MediaService) List(ctx context.Context, query MediaListQuery) ([]domain.Media, int64, error) {
	return s.store.List(ctx, contract.MediaQuery{
		Page:     query.Page,
		PageSize: query.PageSize,
		Scene:    query.Scene,
		Search:   query.Search,
	})
}

// RecordMedia 记录上传后的素材元数据，并按路径去重。
func (s *MediaService) RecordMedia(ctx context.Context, result UploadResult, scene string) (*domain.Media, error) {
	existing, err := s.store.GetByPath(ctx, result.URL)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	media := &domain.Media{
		Name:     filenameWithoutExtension(result.Filename),
		Filename: result.Filename,
		Path:     result.URL,
		MimeType: result.MimeType,
		Size:     result.Size,
		Scene:    scene,
		Width:    result.Width,
		Height:   result.Height,
	}
	if err := s.store.Create(ctx, media); err != nil {
		return nil, err
	}
	return media, nil
}

// RecordLocalFile 将已存在的本地文件记录到素材库。
func (s *MediaService) RecordLocalFile(ctx context.Context, localPath, scene string) {
	existing, _ := s.store.GetByPath(ctx, localPath)
	if existing != nil {
		return
	}

	diskPath := strings.TrimPrefix(localPath, "/")
	fileInfo, err := s.files.Stat(diskPath)
	if err != nil {
		return
	}

	filename := filepath.Base(localPath)
	mimeType := s.detectMIMEType(diskPath)
	width, height := s.detectImageDimensions(diskPath, mimeType)
	media := &domain.Media{
		Name:     filenameWithoutExtension(filename),
		Filename: filename,
		Path:     localPath,
		MimeType: mimeType,
		Size:     fileInfo.Size(),
		Scene:    scene,
		Width:    width,
		Height:   height,
	}
	if err := s.store.Create(ctx, media); err != nil {
		s.warn("media_record_local_file_failed", "path", localPath, "error", err)
	}
}

// Rename 重命名素材。
func (s *MediaService) Rename(ctx context.Context, id uint, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return contract.ErrMediaNameEmpty
	}
	media, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if media == nil {
		return contract.ErrMediaNotFound
	}
	media.Name = name
	return s.store.Update(ctx, media)
}

// Delete 先软删除元数据，再 best-effort 删除物理文件。
func (s *MediaService) Delete(ctx context.Context, id uint) error {
	media, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if media == nil {
		return contract.ErrMediaNotFound
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}

	diskPath := strings.TrimPrefix(media.Path, "/")
	if err := s.files.Remove(diskPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.warn("media_delete_file_failed", "id", id, "path", diskPath, "error", err)
	}
	return nil
}

// BatchDelete 批量删除素材，并保留逐项成功/失败语义。
func (s *MediaService) BatchDelete(ctx context.Context, ids []uint) (int, []uint) {
	successCount := 0
	failedIDs := make([]uint, 0)
	for _, id := range ids {
		if err := s.Delete(ctx, id); err == nil {
			successCount++
		} else {
			failedIDs = append(failedIDs, id)
		}
	}
	return successCount, failedIDs
}

func (s *MediaService) detectMIMEType(path string) string {
	mimeType := "application/octet-stream"
	file, err := s.files.Open(path)
	if err != nil {
		return mimeType
	}
	defer file.Close()

	buffer := make([]byte, 512)
	if count, _ := file.Read(buffer); count > 0 {
		return http.DetectContentType(buffer[:count])
	}
	return mimeType
}

func (s *MediaService) detectImageDimensions(path, mimeType string) (int, int) {
	if !strings.HasPrefix(mimeType, "image/") || mimeType == "image/svg+xml" {
		return 0, 0
	}
	file, err := s.files.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0
	}
	return config.Width, config.Height
}

func (s *MediaService) warn(message string, keysAndValues ...interface{}) {
	if s.logger != nil {
		s.logger.Warnw(message, keysAndValues...)
	}
}

func filenameWithoutExtension(filename string) string {
	if index := strings.LastIndex(filename, "."); index > 0 {
		return filename[:index]
	}
	return filename
}
