package smtp

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/modules/notification/contract"
)

func TestBalanceRecoveryEmailHasFixedBrandAndEscapedMultipartContent(t *testing.T) {
	input := contract.BalanceRecoveryEmailInput{OrderNo: "TEST-20261107", Products: `5x <script>alert("x")</script> & 商品`}
	message, err := buildBalanceRecoveryMessage(buildFromAddress("sender@example.invalid", contract.BalanceRecoveryBrand), "customer@example.invalid", input)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(message))
	if err != nil {
		t.Fatal(err)
	}
	from, err := mail.ParseAddress(parsed.Header.Get("From"))
	if err != nil || from.Name != "π星卡社" || from.Address != "sender@example.invalid" {
		t.Fatalf("wrong sender: %v %v", from, err)
	}
	subject, err := (&mime.WordDecoder{}).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || subject != contract.BalanceRecoverySubject {
		t.Fatal(subject, err)
	}
	contentType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || contentType != "multipart/alternative" {
		t.Fatal(contentType, err)
	}
	reader := multipart.NewReader(parsed.Body, params["boundary"])
	for i, kind := range []string{"text/plain", "text/html"} {
		part, err := reader.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(part.Header.Get("Content-Type"), kind) {
			t.Fatal("wrong MIME part")
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"π星卡社", input.OrderNo, "Cqz0924ttxk", contract.BalanceRecoveryStoreURL, "如原订单已过期"} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("part %d missing %s", i, want)
			}
		}
		if i == 1 && (strings.Contains(string(body), "<script>") || !strings.Contains(string(body), "&lt;script&gt;")) {
			t.Fatal("unescaped product title")
		}
	}
	if _, err := reader.NextPart(); err != io.EOF {
		t.Fatal("unexpected MIME part", err)
	}
	if err := New(&config.EmailConfig{}).SendBalanceRecoveryEmail("customer@example.invalid", input); err != contract.ErrEmailServiceDisabled {
		t.Fatal("disabled SMTP sent", err)
	}
}

func TestBalanceRecoveryEmailPreview(t *testing.T) {
	// Optional artifact export uses the production renderer and never sends mail.
	dir := os.Getenv("BALANCE_RECOVERY_PREVIEW_DIR")
	if dir == "" {
		return
	}
	plain, html, err := RenderBalanceRecoveryEmail(contract.BalanceRecoveryEmailInput{OrderNo: "PREVIEW-20261107-001", Products: "ChatGPT Pro 5x 兑换码｜埃及｜自动发码 × 1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{"email-preview.html": html, "email-preview.txt": plain} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
