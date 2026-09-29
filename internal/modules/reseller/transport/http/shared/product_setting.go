package shared

import (
	"strings"

	resellerapplication "github.com/dujiao-next/internal/modules/reseller/application"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	"github.com/shopspring/decimal"
)

type ProductSettingRequest struct {
	SKUID             uint   `json:"sku_id"`
	IsListed          bool   `json:"is_listed"`
	PricingMode       string `json:"pricing_mode"`
	MarkupPercent     string `json:"markup_percent"`
	FixedMarkupAmount string `json:"fixed_markup_amount"`
	FixedPriceAmount  string `json:"fixed_price_amount"`
	SortOrder         int    `json:"sort_order"`
}

type ProductSettingsUpdateRequest struct {
	Settings []ProductSettingRequest `json:"settings"`
}

func (req ProductSettingsUpdateRequest) ToInput() (resellerapplication.ProductSettingSaveInput, error) {
	input := resellerapplication.ProductSettingSaveInput{
		Settings: make([]resellerapplication.ProductSettingInput, 0, len(req.Settings)),
	}
	for _, item := range req.Settings {
		markup, err := parseDecimal(item.MarkupPercent)
		if err != nil {
			return input, err
		}
		fixedMarkup, err := parseDecimal(item.FixedMarkupAmount)
		if err != nil {
			return input, err
		}
		fixedPrice, err := parseDecimal(item.FixedPriceAmount)
		if err != nil {
			return input, err
		}
		input.Settings = append(input.Settings, resellerapplication.ProductSettingInput{
			SKUID:             item.SKUID,
			IsListed:          item.IsListed,
			PricingMode:       strings.TrimSpace(item.PricingMode),
			MarkupPercent:     markup,
			FixedMarkupAmount: fixedMarkup,
			FixedPriceAmount:  fixedPrice,
			SortOrder:         item.SortOrder,
		})
	}
	return input, nil
}

func DetailDTOInput(detail *resellerapplication.ProductSettingDetail) dto.ResellerProductSettingDTOInput {
	if detail == nil {
		return dto.ResellerProductSettingDTOInput{}
	}
	return dto.ResellerProductSettingDTOInput{
		Product:          detail.Product,
		Settings:         detail.Settings,
		EffectiveBySKUID: decimalMapToStringMap(detail.EffectiveBySKUID),
		RuleBySKUID:      detail.RuleBySKUID,
	}
}

func ListDTOInput(rows []resellerapplication.ProductSettingListRow) []dto.ResellerProductSettingDTOInput {
	out := make([]dto.ResellerProductSettingDTOInput, 0, len(rows))
	for i := range rows {
		out = append(out, dto.ResellerProductSettingDTOInput{
			Product:          rows[i].Product,
			Settings:         rows[i].Settings,
			EffectiveBySKUID: decimalMapToStringMap(rows[i].EffectiveBySKUID),
			RuleBySKUID:      rows[i].RuleBySKUID,
		})
	}
	return out
}

func PreviewDTOInput(items []resellerapplication.ProductSettingPreviewItem) []dto.ResellerProductSettingPreviewInput {
	out := make([]dto.ResellerProductSettingPreviewInput, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ResellerProductSettingPreviewInput{
			SKUID:          item.SKUID,
			IsListed:       item.IsListed,
			BasePrice:      item.BasePrice.StringFixed(2),
			EffectivePrice: item.EffectivePrice.StringFixed(2),
			Valid:          item.Valid,
			ErrorCode:      item.ErrorCode,
		})
	}
	return out
}

func parseDecimal(raw string) (decimal.Decimal, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
}

func decimalMapToStringMap(input map[uint]decimal.Decimal) map[uint]string {
	out := make(map[uint]string, len(input))
	for key, value := range input {
		out[key] = value.StringFixed(2)
	}
	return out
}
