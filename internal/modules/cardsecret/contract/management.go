package contract

import (
	"context"
	"encoding/json"
)

// Management keeps delivered PAIPLUS cards in their authoritative encrypted
// registry. They must not become available inventory that can be sold twice.
type Management interface {
	Call(context.Context, string, ManagementInput, uint) (json.RawMessage, error)
}

type ManagementInput struct {
	CardID    string `json:"card_id,omitempty"`
	Query     string `json:"query,omitempty"`
	Status    string `json:"status,omitempty"`
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
	OrderID   uint   `json:"order_id,omitempty"`
	ProductID uint   `json:"product_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Confirmed bool   `json:"confirmed,omitempty"`
}

type ManagementError struct {
	Code    string
	Message string
}

func (e *ManagementError) Error() string { return e.Message }
