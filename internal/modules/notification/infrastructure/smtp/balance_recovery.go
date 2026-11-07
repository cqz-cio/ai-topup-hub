package smtp

import (
	"bytes"
	"fmt"
	"html/template"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"

	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/telegramidentity"
)

const recoveryHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>π星卡社 · 购买恢复通知</title></head>
<body style="margin:0;padding:24px 12px;background:#f4f6f5;color:#202923;font-family:Arial,'Microsoft YaHei',sans-serif;line-height:1.8">
<table role="presentation" style="width:100%;max-width:620px;margin:0 auto;border-collapse:collapse;background:#fff;border:1px solid #e1e6e3"><tr><td style="padding:28px 28px 18px;border-bottom:1px solid #e1e6e3;font-size:24px;font-weight:bold;color:#174b36">π星卡社</td></tr>
<tr><td style="padding:24px 28px"><h1 style="margin:0 0 22px;font-size:22px;line-height:1.5">卡台余额已恢复，可继续购买</h1>
<p>您好！</p><p>您此前在 π星卡社的购买因卡台余额不足而暂时中断，给您带来不便，我们深表歉意。</p>
<p>系统现已确认卡台余额恢复，满足您此前订单的采购金额要求。您可以返回商城，重新尝试付款。</p>
<p style="padding:14px 0;border-top:1px solid #e1e6e3;border-bottom:1px solid #e1e6e3;overflow-wrap:anywhere;word-break:break-word">商品：{{.Products}}<br>订单号：{{.OrderNo}}</p>
<p style="margin:26px 0"><a href="https://www.qzweb.cloud/" style="display:inline-block;padding:12px 22px;background:#216344;color:#fff;text-decoration:none;border-radius:6px;font-weight:bold">返回商城继续购买</a></p>
<p style="font-size:14px;color:#535d57">如原订单已过期，请重新下单。商品价格、库存及付款条件以重新提交时的页面为准。</p>
<p>如需帮助，请联系客服微信：<strong>Cqz0924ttxk</strong>。</p></td></tr>
<tr><td style="padding:18px 28px;border-top:1px solid #e1e6e3;font-size:14px;color:#535d57">π星卡社<br><a href="https://www.qzweb.cloud/" style="color:#216344">www.qzweb.cloud</a></td></tr></table></body></html>`

var recoveryTemplate = template.Must(template.New("balance-recovery").Parse(recoveryHTML))

// RenderBalanceRecoveryEmail is also used to generate the reviewable preview.
func RenderBalanceRecoveryEmail(input contract.BalanceRecoveryEmailInput) (plain, html string, err error) {
	plain = fmt.Sprintf("π星卡社\n\n卡台余额已恢复，可继续购买\n\n您好！\n\n您此前在 π星卡社的购买因卡台余额不足而暂时中断，给您带来不便，我们深表歉意。\n\n系统现已确认卡台余额恢复，满足您此前订单的采购金额要求。您可以返回商城，重新尝试付款。\n\n商品：%s\n订单号：%s\n\n返回商城继续购买：%s\n\n如原订单已过期，请重新下单。商品价格、库存及付款条件以重新提交时的页面为准。\n\n如需帮助，请联系客服微信：Cqz0924ttxk。\n\nπ星卡社\nwww.qzweb.cloud\n", input.Products, input.OrderNo, contract.BalanceRecoveryStoreURL)
	var body bytes.Buffer
	err = recoveryTemplate.Execute(&body, input)
	return plain, body.String(), err
}

func buildBalanceRecoveryMessage(from, to string, input contract.BalanceRecoveryEmailInput) ([]byte, error) {
	plain, html, err := RenderBalanceRecoveryEmail(input)
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writeStandardHeaders(&body, from, to, contract.BalanceRecoverySubject, "")
	fmt.Fprintf(&body, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", writer.Boundary())
	for _, part := range []struct{ contentType, text string }{{"text/plain; charset=UTF-8", plain}, {"text/html; charset=UTF-8", html}} {
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", part.contentType)
		header.Set("Content-Transfer-Encoding", "quoted-printable")
		out, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		encoded := quotedprintable.NewWriter(out)
		if _, err = encoded.Write([]byte(part.text)); err != nil {
			return nil, err
		}
		if err = encoded.Close(); err != nil {
			return nil, err
		}
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func (s *Service) SendBalanceRecoveryEmail(to string, input contract.BalanceRecoveryEmailInput) error {
	if telegramidentity.IsPlaceholderEmail(to) {
		return contract.ErrInvalidEmail
	}
	from, addr, err := s.prepareSMTPEnvelope(to, contract.BalanceRecoveryBrand)
	if err != nil {
		return err
	}
	message, err := buildBalanceRecoveryMessage(from, to, input)
	if err != nil {
		return err
	}
	return s.sendSMTPMessage(addr, to, message)
}
