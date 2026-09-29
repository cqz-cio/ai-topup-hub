//go:build !(linux || darwin)

package selfupdate

// acquireBinaryLock 在不支持自更新的平台上是空实现。
// Detect() 会先以 unsupported_os 阻断，任何真实调用都到不了这里。
func acquireBinaryLock(string) (func(), error) {
	return func() {}, nil
}
