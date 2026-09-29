package version

import "testing"

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.0.1", "v1.0.0", true},
		{"v1.1.0", "v1.0.9", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v1.0.1", false},
		{"1.2.3", "v1.2.3", false},
		{"v1.2.3-rc.1", "v1.2.3", false},
		{"v1.2.4", "v1.2.3-rc.1", true},

		// 预发布优先级：正式版高于同核心版本号的预发布版。
		// 这条是关键——少了它，跑 RC 的用户永远收不到同版本号正式版的升级提示。
		{"v1.2.3", "v1.2.3-rc.1", true},
		{"v1.2.3", "v1.2.3-beta.2", true},
		{"v1.2.3", "v1.2.3-alpha", true},
		// 同为预发布时按标识符比较
		{"v1.2.3-rc.2", "v1.2.3-rc.1", true},
		{"v1.2.3-rc.1", "v1.2.3-rc.2", false},
		{"v1.2.3-rc.1", "v1.2.3-rc.1", false},
		{"v1.2.3-rc.10", "v1.2.3-rc.9", true},
		// stable > rc > beta > alpha（rc/beta/alpha 之间按字典序恰好成立）
		{"v1.2.3-rc.1", "v1.2.3-beta.1", true},
		{"v1.2.3-beta.1", "v1.2.3-alpha.1", true},
		{"v1.2.3-alpha.1", "v1.2.3-rc.1", false},
		// 数字段优先级低于非数字段（SemVer §11.4.3）
		{"v1.2.3-rc", "v1.2.3-1", true},
		// 前缀相同时段数多的更大（SemVer §11.4.4）
		{"v1.2.3-rc.1.1", "v1.2.3-rc.1", true},
		{"v1.2.3-rc.1", "v1.2.3-rc.1.1", false},
		// 核心版本号的差异优先于预发布段
		{"v1.2.4-rc.1", "v1.2.3", true},
		{"v1.2.3-rc.1", "v1.2.4", false},
		// 构建元数据不参与优先级比较
		{"v1.2.3+build.9", "v1.2.3+build.1", false},
		{"v1.2.3+build.1", "v1.2.3", false},
		{"v1.2.3", "v1.2.3-rc.1+build.1", true},

		// 超出 int 范围的数字标识符仍须按数值比较，不能降级成字符串比较
		{"v1.2.3-rc.99999999999999999999999", "v1.2.3-rc.99999999999999999999998", true},
		{"v1.2.3-rc.99999999999999999999998", "v1.2.3-rc.99999999999999999999999", false},
		{"v1.2.3-rc.10000000000000000000000", "v1.2.3-rc.9999999999999999999999", true},
		// 核心数字段同样没有位数上限
		{"v10000000000000000000000.0.0", "v9999999999999999999999.999.999", true},
		{"v9999999999999999999999.999.999", "v10000000000000000000000.0.0", false},
	}

	for _, c := range cases {
		got, err := IsNewerVersion(c.latest, c.current)
		if err != nil {
			t.Errorf("IsNewerVersion(%q, %q) unexpected error: %v", c.latest, c.current, err)
			continue
		}
		if got != c.want {
			t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

// TestIsNewerVersionFailsClosed 版本判断直接控制二进制替换；任一端无法解析时
// 必须拒绝更新，而不是用字符串不等猜测优先级。
func TestIsNewerVersionFailsClosed(t *testing.T) {
	cases := [][2]string{
		{"foo", "bar"},
		{"foo", "foo"},
		{"", "v1.0.0"},
		{"v1.2", "v1.2.3"},
		{"v1.2.3.4", "v1.2.3"},
		{"v1.2.3-", "v1.2.3"},
	}
	for _, c := range cases {
		got, err := IsNewerVersion(c[0], c[1])
		if err == nil {
			t.Errorf("IsNewerVersion(%q, %q) error = nil, want parse error", c[0], c[1])
		}
		if got {
			t.Errorf("IsNewerVersion(%q, %q) = true on parse failure, want fail-closed false", c[0], c[1])
		}
	}
}

// TestParseSemverRejectsMalformed 解析结果会被用来决定要不要自动替换二进制，
// 因此畸形标签必须报错让调用方拒绝自动升级，而不是被悄悄猜成某个版本号。
func TestParseSemverRejectsMalformed(t *testing.T) {
	bad := []string{
		"",               // 空
		"v1",             // 核心段不足
		"v1.2",           // 核心段不足
		"v1.2.3.4",       // 核心段过多
		"v1.2.3-",        // 预发布段为空
		"v1.2.3-..1",     // 预发布标识符为空
		"v1.2.3-rc.",     // 尾部空标识符
		"v1.2.3-rc_1",    // 下划线不在 SemVer 允许的字符集内
		"v1.2.3-rc.1中",   // 非 ASCII
		"va.b.c",         // 核心段非数字
		"v-1.2.3",        // 负数
		"v01.2.3",        // 核心数字段前导零
		"v1.02.3",        // 核心数字段前导零
		"v1.2.03",        // 核心数字段前导零
		"v1.2.3-01",      // 纯数字预发布标识符前导零
		"v1.2.3-rc.01",   // 纯数字预发布标识符前导零
		"v1.2.3+",        // 构建元数据为空
		"v1.2.3+foo..1",  // 构建元数据含空标识符
		"v1.2.3+bad_1",   // 构建元数据含非法字符
		"v1.2.3+foo+bar", // 多个加号
	}
	for _, v := range bad {
		if _, err := parseSemver(v); err == nil {
			t.Errorf("parseSemver(%q) = nil error, want an error", v)
		}
	}

	good := []string{
		"1.2.3",
		"v1.2.3",
		"V1.2.3",
		"v1.2.3-rc.1",
		"v1.2.3-rc-1",
		"v1.2.3-0.3.7",
		"v1.2.3+build.5",
		"v1.2.3-rc.1+build.5",
		"v1.2.3+001", // 构建元数据允许前导零
		"v0.0.0",
		"v10000000000000000000000.0.0",
	}
	for _, v := range good {
		if _, err := parseSemver(v); err != nil {
			t.Errorf("parseSemver(%q) = %v, want nil error", v, err)
		}
	}
}

// TestCompareNumericIdentifiers 数字标识符没有位数上限，比较必须对任意长度成立。
func TestCompareNumericIdentifiers(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1", "2", -1},
		{"2", "1", 1},
		{"10", "9", 1},
		{"1", "1", 0},
		// 前导零不改变数值
		{"01", "1", 0},
		{"007", "7", 0},
		// 远超 int64 的位数
		{"99999999999999999999999", "99999999999999999999998", 1},
		{"10000000000000000000000", "9999999999999999999999", 1},
		{"0", "0", 0},
	}
	for _, c := range cases {
		if got := compareNumericIdentifiers(c.a, c.b); got != c.want {
			t.Errorf("compareNumericIdentifiers(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
