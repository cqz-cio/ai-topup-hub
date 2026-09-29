package contract

import "io"

// Result 是上传成功后的文件元数据。
type Result struct {
	URL      string
	Filename string
	MimeType string
	Size     int64
	Width    int
	Height   int
}

// StoreInput 描述本地存储文件所需的信息。
type StoreInput struct {
	Source   io.Reader
	Scene    string
	Year     string
	Month    string
	Filename string
}

// Store 是上传应用层写入文件所需的端口。
type Store interface {
	Save(input StoreInput) (publicURL string, err error)
}
