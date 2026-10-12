package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxToolRejectsProductionBeforeUsingCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"gateway_url":"https://openapi.alipay.com/gateway.do","private_key":"DO-NOT-PRINT"}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := loadSandboxConfig(path)
	if err == nil || !strings.Contains(err.Error(), "only the official sandbox") || strings.Contains(err.Error(), "DO-NOT-PRINT") {
		t.Fatalf("unsafe config result: %v", err)
	}
}

func TestSandboxToolMissingConfigCannotRun(t *testing.T) {
	if err := run(filepath.Join(t.TempDir(), "missing.json"), "create", "", "0.01"); err == nil {
		t.Fatal("expected missing config error")
	}
}

func TestOfficialConsoleCopyUsesPlatformPublicKeyAndNonJavaPrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sandbox.env")
	data := "# Official console copy format\nAIPAY_APP_ID= sandbox-app\nAIPAY_PUBLIC_KEY= application-public-key\nAIPAY_PRIVATE_PKCS_KEY= non-java-private==\nAIPAY_PRIVATE_KEY= java-private\nAIPAY_ALIPAY_PUBLIC_KEY= platform-public-key\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadSandboxConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppID != "sandbox-app" || cfg.PrivateKey != "non-java-private==" || cfg.AlipayPublicKey != "platform-public-key" {
		t.Fatal("official console fields were mapped incorrectly")
	}
}
