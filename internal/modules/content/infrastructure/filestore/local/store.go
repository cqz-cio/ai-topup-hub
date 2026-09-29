package local

import (
	"io"
	"io/fs"
	"os"

	"github.com/dujiao-next/internal/modules/content/contract"
)

// Store 使用当前工作目录下的本地文件系统。
type Store struct{}

var _ contract.FileStore = Store{}

// New 创建本地文件适配器。
func New() Store {
	return Store{}
}

// Stat 返回文件信息。
func (Store) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// Open 打开文件用于读取。
func (Store) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

// Remove 删除文件。
func (Store) Remove(name string) error {
	return os.Remove(name)
}
