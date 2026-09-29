package application

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
)

// ExportAvailableCardSecretInput 可用卡密出库导出输入。
type ExportAvailableCardSecretInput struct {
	ProductID         uint
	SKUID             uint
	BatchID           uint
	Limit             int
	Format            string
	DeleteAfterExport bool
}

// ExportAvailableCardSecretResult 可用卡密出库导出结果。
type ExportAvailableCardSecretResult struct {
	Content     []byte
	ContentType string
	Count       int
}

// ExportCardSecrets 批量导出卡密（txt/csv）
func (s *Service) ExportCardSecrets(ids []uint, batchID uint, filter ListCardSecretInput, format string) ([]byte, string, error) {
	normalizedFormat, err := normalizeCardSecretExportFormat(format)
	if err != nil {
		return nil, "", ErrInvalid
	}
	normalizedIDs, err := s.resolveExportTargetCardSecretIDs(ids, batchID, filter)
	if err != nil {
		return nil, "", err
	}

	items, err := s.secretRepo.ListByIDs(normalizedIDs)
	if err != nil {
		return nil, "", ErrFetchFailed
	}
	if len(items) == 0 {
		return nil, "", ErrNotFound
	}
	return buildCardSecretExportContent(items, normalizedFormat)
}

// ExportAvailableCardSecrets 从可用库存中导出指定数量卡密，并在同一事务内标记已用或删除。
func (s *Service) ExportAvailableCardSecrets(input ExportAvailableCardSecretInput) (*ExportAvailableCardSecretResult, error) {
	normalizedFormat, err := normalizeCardSecretExportFormat(input.Format)
	if err != nil || input.ProductID == 0 || input.Limit <= 0 {
		return nil, ErrInvalid
	}
	if err := s.validateAutoCardSecretExportScope(input.ProductID, input.SKUID); err != nil {
		return nil, err
	}

	var result *ExportAvailableCardSecretResult
	if s.transactions == nil {
		return nil, ErrFetchFailed
	}
	err = s.transactions.Transaction(func(secretRepo cardsecretcontract.Repository, _ cardsecretcontract.BatchRepository) error {
		items, err := secretRepo.ListAvailableByProductBatchForUpdate(input.ProductID, input.SKUID, input.BatchID, input.Limit)
		if err != nil {
			return ErrFetchFailed
		}
		if len(items) == 0 {
			return ErrNotFound
		}
		if len(items) < input.Limit {
			return ErrInsufficient
		}

		content, contentType, err := buildCardSecretExportContent(items, normalizedFormat)
		if err != nil {
			return err
		}

		ids := make([]uint, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}

		var affected int64
		if input.DeleteAfterExport {
			affected, err = secretRepo.BatchDeleteByIDs(ids)
			if err != nil {
				return ErrDeleteFailed
			}
		} else {
			affected, err = secretRepo.BatchUpdateStatus(ids, cardsecretdomain.StatusUsed, time.Now())
			if err != nil {
				return ErrUpdateFailed
			}
		}
		if affected != int64(len(ids)) {
			if input.DeleteAfterExport {
				return ErrDeleteFailed
			}
			return ErrUpdateFailed
		}

		result = &ExportAvailableCardSecretResult{
			Content:     content,
			ContentType: contentType,
			Count:       len(items),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeCardSecretExportFormat(format string) (string, error) {
	normalizedFormat := strings.ToLower(strings.TrimSpace(format))
	switch normalizedFormat {
	case constants.ExportFormatTXT, constants.ExportFormatCSV:
		return normalizedFormat, nil
	default:
		return "", ErrInvalid
	}
}

func buildCardSecretExportContent(items []cardsecretdomain.Secret, normalizedFormat string) ([]byte, string, error) {
	if normalizedFormat == constants.ExportFormatTXT {
		lines := make([]string, 0, len(items))
		for _, item := range items {
			secret := strings.TrimSpace(item.Secret)
			if secret == "" {
				continue
			}
			lines = append(lines, secret)
		}
		return []byte(strings.Join(lines, "\n")), "text/plain; charset=utf-8", nil
	}

	buffer := bytes.NewBuffer(nil)
	writer := csv.NewWriter(buffer)
	header := []string{"id", "secret", "status", "product_id", "sku_id", "order_id", "batch_id", "created_at"}
	if err := writer.Write(header); err != nil {
		return nil, "", ErrFetchFailed
	}
	for _, item := range items {
		orderID := ""
		if item.OrderID != nil {
			orderID = strconv.FormatUint(uint64(*item.OrderID), 10)
		}
		batchID := ""
		if item.BatchID != nil {
			batchID = strconv.FormatUint(uint64(*item.BatchID), 10)
		}
		row := []string{
			strconv.FormatUint(uint64(item.ID), 10),
			item.Secret,
			item.Status,
			strconv.FormatUint(uint64(item.ProductID), 10),
			strconv.FormatUint(uint64(item.SKUID), 10),
			orderID,
			batchID,
			item.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return nil, "", ErrFetchFailed
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", ErrFetchFailed
	}
	return buffer.Bytes(), "text/csv; charset=utf-8", nil
}

func (s *Service) validateAutoCardSecretExportScope(productID, skuID uint) error {
	if productID == 0 || s.productRepo == nil || s.productSKURepo == nil {
		return ErrInvalid
	}
	product, err := s.productRepo.GetByID(strings.TrimSpace(strconv.FormatUint(uint64(productID), 10)))
	if err != nil {
		return err
	}
	if product == nil {
		return ErrProductNotFound
	}
	if strings.TrimSpace(product.FulfillmentType) != constants.FulfillmentTypeAuto {
		return ErrInvalid
	}
	if skuID == 0 {
		return nil
	}
	sku, err := s.productSKURepo.GetByID(skuID)
	if err != nil {
		return err
	}
	if sku == nil || sku.ProductID != productID || !sku.IsActive {
		return ErrProductSKUInvalid
	}
	return nil
}

func (s *Service) resolveExportTargetCardSecretIDs(ids []uint, batchID uint, filter ListCardSecretInput) ([]uint, error) {
	normalizedIDs, err := s.resolveBatchTargetCardSecretIDs(ids, batchID, filter)
	if err == nil {
		return normalizedIDs, nil
	}
	if !errors.Is(err, ErrInvalid) || len(normalizeCardSecretIDs(ids)) > 0 || batchID != 0 || s.hasListFilter(filter) {
		return nil, err
	}

	targetIDs, err := s.secretRepo.ListIDs(s.buildRepositoryFilter(filter))
	if err != nil {
		return nil, ErrFetchFailed
	}
	if len(targetIDs) == 0 {
		return nil, ErrNotFound
	}
	return targetIDs, nil
}

func (s *Service) resolveBatchTargetCardSecretIDs(ids []uint, batchID uint, filter ListCardSecretInput) ([]uint, error) {
	normalizedIDs := normalizeCardSecretIDs(ids)
	if len(normalizedIDs) > 0 {
		return normalizedIDs, nil
	}
	if s.hasListFilter(filter) {
		targetIDs, err := s.secretRepo.ListIDs(s.buildRepositoryFilter(filter))
		if err != nil {
			return nil, ErrFetchFailed
		}
		if len(targetIDs) == 0 {
			return nil, ErrNotFound
		}
		return targetIDs, nil
	}
	if batchID == 0 {
		return nil, ErrInvalid
	}
	targetIDs, err := s.secretRepo.ListIDsByBatchID(batchID)
	if err != nil {
		return nil, ErrFetchFailed
	}
	if len(targetIDs) == 0 {
		return nil, ErrNotFound
	}
	return targetIDs, nil
}
