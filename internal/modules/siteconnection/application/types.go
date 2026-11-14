package application

// CreateInput 创建对接连接输入。
type CreateInput struct {
	Name               string  `json:"name"`
	BaseURL            string  `json:"base_url"`
	ApiKey             string  `json:"api_key"`
	ApiSecret          string  `json:"api_secret"`
	Protocol           string  `json:"protocol"`
	CallbackURL        string  `json:"callback_url"`
	RetryMax           int     `json:"retry_max"`
	RetryIntervals     string  `json:"retry_intervals"`
	ExchangeRate       float64 `json:"exchange_rate"`
	PriceMarkupPercent float64 `json:"price_markup_percent"`
	PriceRoundingMode  string  `json:"price_rounding_mode"`
	AutoSyncPrice      bool    `json:"auto_sync_price"`
	// CategoryMarkups 按上游分类设置的加价百分比 {上游分类ID: 百分比}
	CategoryMarkups map[string]float64 `json:"category_markups"`
}

// UpdateInput 更新对接连接输入。
type UpdateInput struct {
	Name               string   `json:"name"`
	BaseURL            string   `json:"base_url"`
	ApiKey             string   `json:"api_key"`
	ApiSecret          string   `json:"api_secret"` // 为空则不更新
	Protocol           string   `json:"protocol"`
	CallbackURL        string   `json:"callback_url"`
	RetryMax           int      `json:"retry_max"`
	RetryIntervals     string   `json:"retry_intervals"`
	ExchangeRate       *float64 `json:"exchange_rate"`
	PriceMarkupPercent *float64 `json:"price_markup_percent"` // 指针类型，区分 0 和未传
	PriceRoundingMode  *string  `json:"price_rounding_mode"`
	AutoSyncPrice      *bool    `json:"auto_sync_price"`
	// CategoryMarkups 为 nil（字段未传）时不修改；传 {} 表示清空所有分类加价。
	CategoryMarkups map[string]float64 `json:"category_markups"`
}

// PingResult 连接测试结果。
type PingResult struct {
	SiteName        string                 `json:"site_name"`
	ProtocolVersion string                 `json:"protocol_version"`
	UserID          uint                   `json:"user_id"`
	Balance         string                 `json:"balance"`
	Currency        string                 `json:"currency"`
	MemberLevel     map[string]interface{} `json:"member_level,omitempty"`
}
