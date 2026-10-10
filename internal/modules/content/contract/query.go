package contract

// PostOrder 限定文章列表可使用的稳定排序策略，避免向持久化层传递任意 SQL。
type PostOrder uint8

const (
	PostOrderCreatedDesc PostOrder = iota
	PostOrderPublishedDesc
)

// PostQuery 描述文章列表查询。
//
// CategoryIDs 是已展开的分类筛选范围，用指针以外的 nil / 空切片区分两种语义：
// nil 表示不按分类筛选；非 nil 时按 IN 匹配，空切片表示目标分类不可用
// （不存在或已停用），此时应返回空结果而不是退化成不筛选。
type PostQuery struct {
	Page          int
	PageSize      int
	Type          string
	Search        string
	CategoryIDs   []uint
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
