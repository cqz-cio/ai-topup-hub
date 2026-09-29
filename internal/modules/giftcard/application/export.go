package application

import (
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"

	"github.com/dujiao-next/internal/constants"
)

// Export 导出礼品卡。
func (s *Service) Export(ids []uint, format string) ([]byte, string, error) {
	if s == nil || s.repo == nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	normalizedIDs := normalizeIDs(ids)
	if len(normalizedIDs) == 0 {
		return nil, "", giftcardcontract.ErrInvalid
	}
	normalizedFormat := strings.TrimSpace(strings.ToLower(format))
	if normalizedFormat != constants.ExportFormatCSV && normalizedFormat != constants.ExportFormatTXT {
		return nil, "", giftcardcontract.ErrInvalid
	}

	cards, err := s.repo.ListByIDs(normalizedIDs)
	if err != nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	if len(cards) == 0 {
		return nil, "", giftcardcontract.ErrNotFound
	}

	if normalizedFormat == constants.ExportFormatTXT {
		lines := make([]string, 0, len(cards))
		for _, card := range cards {
			lines = append(lines, strings.TrimSpace(card.Code))
		}
		return []byte(strings.Join(lines, "\n")), "text/plain; charset=utf-8", nil
	}

	builder := &strings.Builder{}
	writer := csv.NewWriter(builder)
	if err := writer.Write([]string{
		"id",
		"batch_no",
		"name",
		"code",
		"amount",
		"currency",
		"status",
		"redeemed_user_id",
		"redeemed_at",
		"expires_at",
		"created_at",
	}); err != nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	for _, card := range cards {
		batchNo := ""
		if card.Batch != nil {
			batchNo = card.Batch.BatchNo
		}
		redeemedUserID := ""
		if card.RedeemedUserID != nil {
			redeemedUserID = strconv.FormatUint(uint64(*card.RedeemedUserID), 10)
		}
		record := []string{
			strconv.FormatUint(uint64(card.ID), 10),
			batchNo,
			card.Name,
			card.Code,
			card.Amount.String(),
			card.Currency,
			card.Status,
			redeemedUserID,
			formatNullableTime(card.RedeemedAt),
			formatNullableTime(card.ExpiresAt),
			card.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(record); err != nil {
			return nil, "", giftcardcontract.ErrFetchFailed
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	return []byte(builder.String()), "text/csv; charset=utf-8", nil
}
