package main

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
)

var bannerANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// bannerLinks 提取横幅里的链接，剥掉 ANSI 颜色码并按空白切分。
// 断言链接本身而非"标签 + 对齐空白"的整行字面量，横幅排版微调就不会连带弄红 CI。
func bannerLinks(banner string) []string {
	var links []string
	for _, field := range strings.Fields(bannerANSIPattern.ReplaceAllString(banner, "")) {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			links = append(links, field)
		}
	}
	return links
}

func TestWriteStartupBannerOmitsRetiredFrontendRepositories(t *testing.T) {
	var output strings.Builder
	writeStartupBanner(&output)

	banner := output.String()
	for _, removed := range []string{
		"https://github.com/dujiao-next/user",
		"https://github.com/dujiao-next/admin",
	} {
		if strings.Contains(banner, removed) {
			t.Errorf("startup banner still contains retired repository: %s", removed)
		}
	}

	// 用整 token 相等比较：组织地址是主仓地址的前缀，子串匹配挡不住"只剩主仓"的退化。
	links := bannerLinks(banner)
	for _, retained := range []string{
		"https://github.com/dujiao-next",
		"https://github.com/dujiao-next/dujiao-next",
		"https://dujiao-next.com",
	} {
		if !slices.Contains(links, retained) {
			t.Errorf("startup banner is missing retained repository: %s (got %v)", retained, links)
		}
	}
}

func TestWeakRuntimeSecretNamesCoversEveryRootSecret(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "change-me-32-byte-secret-key!!"},
		JWT:     config.JWTConfig{SecretKey: "your-secret-key-change-in-production-please"},
		UserJWT: config.JWTConfig{SecretKey: "user-change-me-in-production"},
	}

	want := []string{"app.secret_key", "jwt.secret", "user_jwt.secret"}
	if got := weakRuntimeSecretNames(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("weak runtime secrets want %v got %v", want, got)
	}
}

func TestWeakRuntimeSecretNamesAcceptsStrongIndependentSecrets(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "2f8d164772cd4bbcaef8fa4ad19a2a26f7a15505"},
		JWT:     config.JWTConfig{SecretKey: "dd914407e55c4528a393fe522215c18f5fc8687b"},
		UserJWT: config.JWTConfig{SecretKey: "ca36df49b49446d2a9b2cac7f035d11574575b53"},
	}

	if got := weakRuntimeSecretNames(cfg); len(got) != 0 {
		t.Fatalf("strong runtime secrets reported as weak: %v", got)
	}
}

func TestWeakRuntimeSecretNamesRejectsReusedStrongSecrets(t *testing.T) {
	shared := "2f8d164772cd4bbcaef8fa4ad19a2a26f7a15505"
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: shared},
		JWT:     config.JWTConfig{SecretKey: shared},
		UserJWT: config.JWTConfig{SecretKey: "ca36df49b49446d2a9b2cac7f035d11574575b53"},
	}

	want := []string{"app.secret_key", "jwt.secret"}
	if got := weakRuntimeSecretNames(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("reused runtime secrets want %v got %v", want, got)
	}
}

func TestUnsafeBootstrapAdminPasswordRejectsDefaultsAndPolicyViolations(t *testing.T) {
	cfg := &config.Config{Security: config.SecurityConfig{PasswordPolicy: config.PasswordPolicyConfig{
		MinLength:     10,
		RequireUpper:  true,
		RequireLower:  true,
		RequireNumber: true,
	}}}

	for _, password := range []string{"admin123", "alllowercase1", "NOLOWERCASE1", "NoNumberHere"} {
		if !unsafeBootstrapAdminPassword(cfg, password) {
			t.Errorf("expected bootstrap password %q to be rejected", password)
		}
	}
	if unsafeBootstrapAdminPassword(cfg, "StrongBootstrap123") {
		t.Fatal("expected strong bootstrap password to be accepted")
	}
	if unsafeBootstrapAdminPassword(cfg, "") {
		t.Fatal("empty bootstrap password should keep the skip-initialization behavior")
	}
}
