package contract

const BalanceRecoveryBrand = "π星卡社"
const BalanceRecoverySubject = "【π星卡社】卡台余额已恢复，可继续购买"
const BalanceRecoveryStoreURL = "https://www.qzweb.cloud/"

// Customer-safe fields only: never put supplier IDs, balances or cost in this input.
type BalanceRecoveryEmailInput struct {
	OrderNo  string
	Products string
}

type BalanceRecoveryEmailSender interface {
	SendBalanceRecoveryEmail(string, BalanceRecoveryEmailInput) error
}
