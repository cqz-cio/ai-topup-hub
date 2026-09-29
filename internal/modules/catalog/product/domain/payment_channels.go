package productdomain

import (
	"encoding/json"
	"strings"
)

// DecodePaymentChannelIDs 将商品持久化的支付渠道 JSON 解码为有效 ID 列表。
func DecodePaymentChannelIDs(raw string) []uint {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(trimmed), &ids); err != nil {
		return nil
	}
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// EncodePaymentChannelIDs 将有效支付渠道 ID 编码为商品持久化格式。
func EncodePaymentChannelIDs(ids []uint) string {
	if len(ids) == 0 {
		return ""
	}
	payload, err := json.Marshal(ids)
	if err != nil {
		return ""
	}
	return string(payload)
}
