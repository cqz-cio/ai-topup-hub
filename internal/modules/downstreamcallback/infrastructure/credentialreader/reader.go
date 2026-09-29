package credentialreader

import (
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"
	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
)

// Source 是 API 凭证上下文暴露给防腐适配器的最小端口。
type Source interface {
	GetByID(id uint) (*apicredentialdomain.ApiCredential, error)
}

// Reader 将 API 凭证投影为下游回调签名凭证。
type Reader struct {
	source Source
}

var _ downstreamcontract.CredentialReader = (*Reader)(nil)

func New(source Source) *Reader {
	if source == nil {
		panic("downstream callback credential reader: source is nil")
	}
	return &Reader{source: source}
}

func (r *Reader) GetByID(id uint) (*downstreamcontract.Credential, error) {
	credential, err := r.source.GetByID(id)
	if err != nil || credential == nil {
		return nil, err
	}
	return &downstreamcontract.Credential{
		ID:        credential.ID,
		APIKey:    credential.ApiKey,
		APISecret: credential.ApiSecret,
	}, nil
}
