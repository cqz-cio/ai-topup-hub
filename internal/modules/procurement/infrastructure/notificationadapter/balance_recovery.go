package notificationadapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/constants"
	nc "github.com/dujiao-next/internal/modules/notification/contract"
	nd "github.com/dujiao-next/internal/modules/notification/domain"
	pc "github.com/dujiao-next/internal/modules/procurement/contract"
	pd "github.com/dujiao-next/internal/modules/procurement/domain"
)

const recoveryBizType = "supplier_balance_recovered"

type RecoveryLogs interface {
	nc.LogRepository
	nc.SuccessfulDeliveryReader
}

type RecoveryMailer struct {
	sender nc.BalanceRecoveryEmailSender
	logs   RecoveryLogs
}

func NewRecoveryMailer(sender nc.BalanceRecoveryEmailSender, logs RecoveryLogs) pc.BalanceRecoveryMailer {
	return &RecoveryMailer{sender: sender, logs: logs}
}

func (m *RecoveryMailer) SendBalanceRecoveryEmail(ctx context.Context, wait *pd.BalanceWait) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.sender == nil || m.logs == nil || wait == nil {
		return errors.New("recovery email unavailable")
	}
	alreadySent, err := m.logs.HasSuccessfulDelivery(constants.NotificationEventExceptionAlert, recoveryBizType, wait.ID, constants.NotificationChannelEmail, wait.Recipient)
	if err != nil || alreadySent {
		return err
	}
	err = m.sender.SendBalanceRecoveryEmail(wait.Recipient, nc.BalanceRecoveryEmailInput{OrderNo: wait.OrderNo, Products: wait.Products})
	status, message := "success", ""
	if err != nil {
		status, message = "failed", "SMTP delivery failed"
	}
	logErr := m.logs.Create(&nd.NotificationLog{EventType: constants.NotificationEventExceptionAlert, BizType: recoveryBizType, BizID: wait.ID, Channel: constants.NotificationChannelEmail, Recipient: wait.Recipient, Locale: constants.LocaleZhCN, Title: nc.BalanceRecoverySubject, Body: fmt.Sprintf("卡台余额已恢复，可继续购买。\n商品：%s\n订单号：%s", wait.Products, wait.OrderNo), Status: status, ErrorMessage: message, CreatedAt: time.Now().UTC()})
	if err == nil && logErr != nil {
		return errors.Join(pc.ErrBalanceRecoveryMailAccepted, logErr)
	}
	return errors.Join(err, logErr)
}
