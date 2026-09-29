package version

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	// repoOwner GitHub 仓库所有者，用于检测最新发布版本
	repoOwner = "dujiao-next"
	// repoName GitHub 仓库名称
	repoName = "dujiao-next"

	githubAPIBaseURL = "https://api.github.com"
	releaseUserAgent = "dujiao-next-update-checker"
)

// releasePayload GitHub Releases API 响应中本检测器关心的字段
type releasePayload struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	HTMLURL     string         `json:"html_url"`
	Body        string         `json:"body"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []assetPayload `json:"assets"`
}

type assetPayload struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Asset 发行版附件（goreleaser 产出的平台归档与 checksums.txt）
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

// Release GitHub 最新发行版的完整信息，供版本检测与一键升级共用
type Release struct {
	TagName     string
	Name        string
	HTMLURL     string
	Body        string
	PublishedAt time.Time
	Assets      []Asset
}

// CheckResult 检测结果，已包含当前与最新版本以及是否需要更新
type CheckResult struct {
	CurrentVersion string     `json:"current_version"`
	LatestVersion  string     `json:"latest_version"`
	HasUpdate      bool       `json:"has_update"`
	ReleaseURL     string     `json:"release_url,omitempty"`
	ReleaseName    string     `json:"release_name,omitempty"`
	ReleaseNotes   string     `json:"release_notes,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	Source         string     `json:"source"`
}

// ErrRateLimited 触发 GitHub 匿名调用速率限制时返回，便于上层映射成专用提示
var ErrRateLimited = errors.New("github api rate limit exceeded")

// CheckLatestRelease 通过 GitHub Releases API 获取最新发行版并与当前版本比较。
// 仓库地址固定为 dujiao-next/dujiao-next，不接受外部传入，避免 SSRF。
func CheckLatestRelease(ctx context.Context) (*CheckResult, error) {
	release, err := FetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}

	current := strings.TrimSpace(Version)
	latest := strings.TrimSpace(release.TagName)
	hasUpdate, compareErr := IsNewerVersion(latest, current)
	if compareErr != nil {
		return nil, fmt.Errorf("compare latest version %q with current %q: %w", latest, current, compareErr)
	}

	result := &CheckResult{
		CurrentVersion: current,
		LatestVersion:  latest,
		HasUpdate:      hasUpdate,
		ReleaseURL:     release.HTMLURL,
		ReleaseName:    release.Name,
		ReleaseNotes:   release.Body,
		Source:         fmt.Sprintf("https://github.com/%s/%s/releases", repoOwner, repoName),
	}
	if !release.PublishedAt.IsZero() {
		t := release.PublishedAt
		result.PublishedAt = &t
	}
	return result, nil
}

// FetchLatestRelease 拉取最新发行版原始信息（含附件列表），供一键升级下载对应平台归档。
func FetchLatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBaseURL, repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", releaseUserAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return nil, ErrRateLimited
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("github release not found for %s/%s", repoOwner, repoName)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload releasePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	release := &Release{
		TagName:     strings.TrimSpace(payload.TagName),
		Name:        payload.Name,
		HTMLURL:     payload.HTMLURL,
		Body:        payload.Body,
		PublishedAt: payload.PublishedAt,
	}
	for _, a := range payload.Assets {
		release.Assets = append(release.Assets, Asset{
			Name:        a.Name,
			DownloadURL: a.BrowserDownloadURL,
			Size:        a.Size,
		})
	}
	return release, nil
}

// semver 一个解析后的版本号。prerelease 为空表示正式版。
type semver struct {
	// core 也用十进制字符串保存：SemVer 对数字位数没有上限，major/minor/patch
	// 与预发布数字标识符一样不能依赖本机 int 宽度。
	core [3]string
	// prerelease 预发布标识按 "." 拆分后的各段，例如 "rc.1" -> ["rc", "1"]
	prerelease []string
}

// IsNewerVersion 判断 latest 是否比 current 更新。返回 (true, nil) 表示需要更新；
// 当任一版本号无法解析时 fail-closed：返回 false 和错误。这个结果会直接决定
// 是否允许自动替换二进制，不能把“字符串不同”猜成“版本更高”。
func IsNewerVersion(latest, current string) (bool, error) {
	l, lErr := parseSemver(latest)
	c, cErr := parseSemver(current)
	if lErr != nil || cErr != nil {
		return false, errors.Join(lErr, cErr)
	}

	return compareSemver(l, c) > 0, nil
}

// compareSemver 按 SemVer 2.0.0 的优先级规则比较，返回 -1 / 0 / 1。
func compareSemver(a, b semver) int {
	for i := range 3 {
		if c := compareNumericIdentifiers(a.core[i], b.core[i]); c != 0 {
			return c
		}
	}
	return comparePrerelease(a.prerelease, b.prerelease)
}

// comparePrerelease 比较预发布段的优先级（SemVer 2.0.0 §11.3、§11.4）。
//
// 核心规则：有预发布段的版本优先级**低于**同样核心版本号的正式版。
// 这正是之前直接丢弃预发布信息会出问题的地方 —— v1.2.3-rc.1 和 v1.2.3 都被解析成
// [1,2,3]，于是 RC 用户永远收不到同版本号正式版的升级提示。
//
// 段内比较：纯数字段按数值比，其余按 ASCII 字典序比，数字段优先级低于非数字段；
// 前面各段都相等时，段数多的一方优先级更高（rc.1.1 > rc.1）。
func comparePrerelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	// 正式版 > 预发布版
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}

	for i := 0; i < len(a) && i < len(b); i++ {
		aNum := isNumericIdentifier(a[i])
		bNum := isNumericIdentifier(b[i])
		switch {
		case aNum && bNum:
			if c := compareNumericIdentifiers(a[i], b[i]); c != 0 {
				return c
			}
		case aNum != bNum:
			// 数字段优先级低于非数字段
			if aNum {
				return -1
			}
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}

	// 前缀相同时段数多的更大：rc.1.1 > rc.1
	switch {
	case len(a) > len(b):
		return 1
	case len(a) < len(b):
		return -1
	default:
		return 0
	}
}

// isNumericIdentifier 判断预发布段是否为纯数字标识符
func isNumericIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// compareNumericIdentifiers 按数值比较两个纯数字标识符，返回 -1 / 0 / 1。
//
// 不走 strconv.Atoi：SemVer 对数字标识符的位数没有上限，超出 int 范围时 Atoi 会报错，
// 于是两个合法的超大数字会被降级成字符串比较，给出错误的顺序。
// 去掉前导零之后先比长度再比字典序，对任意长度都成立。
func compareNumericIdentifiers(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) > len(b) {
			return 1
		}
		return -1
	}
	return strings.Compare(a, b)
}

// semverIdentifier 是预发布和构建元数据标识符共同允许的字符集：
// ASCII 字母、数字与连字符。
var semverIdentifier = regexp.MustCompile(`^[0-9A-Za-z-]+$`)

// parseSemver 将 "v1.2.3" / "1.2.3" / "v1.2.3-rc.1" / "v1.2.3-rc.1+build.5" 等格式
// 解析为核心三段加预发布段。构建元数据（+ 之后）按 SemVer 规定不参与优先级比较，直接丢弃。
//
// 严格要求核心版本恰好是 X.Y.Z 三段：宽松接受 "1.2" 或 "1.2.3.4" 会把一个明显畸形的
// 标签悄悄解析成某个版本号，然后据此判断要不要自动替换二进制 —— 这种地方宁可报错，
// 让调用方拒绝自动升级，也不要猜。
func parseSemver(v string) (semver, error) {
	var out semver
	s := strings.TrimSpace(v)
	if s == "" {
		return out, errors.New("empty version")
	}
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	// 构建元数据不参与优先级比较，但仍必须验证格式。直接把 "+" 后内容
	// 丢掉会把 "1.2.3+"、"1.2.3+bad_ id" 这类畸形发布标签当成合法版本。
	if i := strings.IndexByte(s, '+'); i >= 0 {
		build := s[i+1:]
		s = s[:i]
		if build == "" {
			return out, fmt.Errorf("empty build metadata in %s", v)
		}
		for _, id := range strings.Split(build, ".") {
			if id == "" {
				return out, fmt.Errorf("empty build metadata identifier in %s", v)
			}
			if !semverIdentifier.MatchString(id) {
				return out, fmt.Errorf("invalid build metadata identifier %q in %s", id, v)
			}
		}
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre := s[i+1:]
		s = s[:i]
		// "1.2.3-" 不是正式版，而是一个畸形标签：SemVer 要求预发布段非空
		if pre == "" {
			return out, fmt.Errorf("empty prerelease in %s", v)
		}
		out.prerelease = strings.Split(pre, ".")
		for _, id := range out.prerelease {
			if id == "" {
				return out, fmt.Errorf("empty prerelease identifier in %s", v)
			}
			if !semverIdentifier.MatchString(id) {
				return out, fmt.Errorf("invalid prerelease identifier %q in %s", id, v)
			}
			// SemVer 2.0.0 §9：纯数字预发布标识符不能有前导零。
			if isNumericIdentifier(id) && len(id) > 1 && id[0] == '0' {
				return out, fmt.Errorf("numeric prerelease identifier %q has leading zeros in %s", id, v)
			}
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("core version must have exactly 3 segments, got %d in %s", len(parts), v)
	}
	for i, part := range parts {
		if !isNumericIdentifier(part) {
			return out, fmt.Errorf("invalid version segment %q in %s", part, v)
		}
		// SemVer 2.0.0 §2：major/minor/patch 不能有前导零。
		if len(part) > 1 && part[0] == '0' {
			return out, fmt.Errorf("version segment %q has leading zeros in %s", part, v)
		}
		out.core[i] = part
	}
	return out, nil
}
