package contract

// PostOrder 限定文章列表可使用的稳定排序策略，避免向持久化层传递任意 SQL。
type PostOrder uint8

const (
	PostOrderCreatedDesc PostOrder = iota
	PostOrderPublishedDesc
)

// PostQuery 描述文章列表查询。
type PostQuery struct {
	Page          int
	PageSize      int
	Type          string
	ExcludeTypes  []string
	Search        string
	OnlyPublished bool
	Order         PostOrder
}

// BannerQuery 描述后台 Banner 列表查询。
type BannerQuery struct {
	Page     int
	PageSize int
	Position string
	Search   string
	IsActive *bool
}

// MediaQuery 描述素材列表查询。
type MediaQuery struct {
	Page     int
	PageSize int
	Scene    string
	Search   string
}
