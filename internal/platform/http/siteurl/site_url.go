// Package siteurl resolves the public storefront origin used by SEO responses.
package siteurl

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Resolve prefers a configured public HTTP(S) origin. Development addresses and
// malformed settings fall back to the current request, including its local port
// during previews. The gateway must preserve Host; arbitrary forwarded hosts
// are deliberately not used to construct canonical URLs.
func Resolve(configured string, request *http.Request) string {
	if origin := publicOrigin(configured); origin != "" {
		return origin
	}
	scheme := "http"
	if request.TLS != nil || strings.EqualFold(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}
	return scheme + "://" + request.Host
}

func publicOrigin(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return ""
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() {
			return ""
		}
	} else if !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return ""
	}
	return u.Scheme + "://" + strings.ToLower(u.Host)
}
