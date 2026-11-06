package supportagent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicOfflineBoundary(t *testing.T) {
	c := OfflineConfig()
	c.PublicOrigin = "https://www.qzweb.cloud"
	s := NewService(c, nil, NewCatalog(), nil)
	request := func(method, path, origin, remote string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8085/api/v1/support/"+path, strings.NewReader(body))
		r.RemoteAddr = remote
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	for _, origin := range []string{"https://evil.example", "http://127.0.0.1:5175", "https://www.qzweb.cloud.evil.example"} {
		if w := request("GET", "session", origin, "127.0.0.1:1", nil, ""); w.Code != 403 {
			t.Fatal(origin, w.Code)
		}
	}
	if w := request("GET", "health", c.PublicOrigin, "203.0.113.5:1", nil, ""); w.Code != 403 {
		t.Fatal("external peer allowed")
	}
	w := request("GET", "session", c.PublicOrigin, "127.0.0.1:1", nil, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe public cookie")
	}
	v := input(1, "全部商品")
	b, _ := json.Marshal(v)
	w = request("POST", "chat", c.PublicOrigin, "127.0.0.1:1", cookie, string(b))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mode":"local"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "chat", "", "127.0.0.1:1", cookie, string(b)); w.Code != 403 {
		t.Fatal("missing origin accepted")
	}
	if w := request("GET", "health", c.PublicOrigin, "127.0.0.1:1", nil, ""); !strings.Contains(w.Body.String(), `"model_enabled":false`) {
		t.Fatal(w.Body.String())
	}
}

func TestPublicOriginValidation(t *testing.T) {
	for _, s := range []string{"http://www.qzweb.cloud", "https://user@www.qzweb.cloud", "https://www.qzweb.cloud/", "https://www.qzweb.cloud?x=1", "https://www.qzweb.cloud:443", "https://"} {
		if ValidatePublicOrigin(s) {
			t.Fatal(s)
		}
	}
	if !ValidatePublicOrigin("https://www.qzweb.cloud") {
		t.Fatal("valid origin rejected")
	}
}
