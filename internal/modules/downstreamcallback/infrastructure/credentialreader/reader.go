package credentialreader

import (
	"github.com/dujiao-next/internal/constants"
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"
	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
)

// Source 是 API 凭证上下文暴露给防腐适配器的最小端口。
type Source interface {
	GetByID(id uint) (*apicredentialdomain.ApiCredential, error)
	GetByApiKey(apiKey string) (*apicredentialdomain.ApiCredential, error)
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
	// GetByApiKey 预加载 User，与 upstream_auth 中间件读取的是同一份数据。
	withUser, err := r.source.GetByApiKey(credential.ApiKey)
	if err != nil {
		return nil, err
	}
	return &downstreamcontract.Credential{
		ID:        credential.ID,
		APIKey:    credential.ApiKey,
		APISecret: credential.ApiSecret,
		Active:    isCredentialActive(withUser),
	}, nil
}

func isCredentialActive(credential *apicredentialdomain.ApiCredential) bool {
	return credential != nil &&
		credential.Status == constants.ApiCredentialStatusApproved &&
		credential.IsActive &&
		credential.User != nil &&
		credential.User.Status == constants.UserStatusActive
}
