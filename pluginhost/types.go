package pluginhost

import "context"

// 权限常量。
const (
	PermissionConfig       = "plugin.config"
	PermissionDB           = "plugin.db"
	PermissionDBMain       = "plugin.db.main"
	PermissionRoutePublic  = "plugin.route.public"
	PermissionRouteAdmin   = "plugin.route.admin"
	PermissionRouteAuth    = "plugin.route.auth"
	PermissionRouteUser    = "plugin.route.user"
	PermissionRouteChannel = "plugin.route.channel"
	PermissionPayment      = "plugin.payment"
	PermissionEventOrder   = "plugin.event.order"
	PermissionScheduler    = "plugin.scheduler"
	PermissionAssets       = "plugin.assets"
	PermissionPrivileged   = "plugin.host.privileged"
)

// 类型常量。
const (
	TypeTheme   = "theme"
	TypePayment = "payment"
	TypeFeature = "feature"
)

// 作用域常量。
const (
	ScopePublic  = "public"
	ScopeAdmin   = "admin"
	ScopeAuth    = "auth"
	ScopeUser    = "user"
	ScopeChannel = "channel"
)

// Manifest 插件清单。
type Manifest struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Type           string                 `json:"type"`
	Version        string                 `json:"version"`
	Author         string                 `json:"author"`
	Summary        string                 `json:"summary"`
	Description    string                 `json:"description"`
	Compatible     string                 `json:"compatible"`
	HostAPIVersion string                 `json:"host_api_version"`
	GoVersion      string                 `json:"go_version"`
	BuildTarget    string                 `json:"build_target"`
	EntrySymbol    string                 `json:"entry_symbol"`
	Permissions    []string               `json:"permissions"`
	ConfigSchema   map[string]interface{} `json:"config_schema"`
	Assets         []string               `json:"assets"`
	Homepage       string                 `json:"homepage"`
	Source         string                 `json:"source"`
}

// PageRegistration 后台/前台页面元数据注册。
type PageRegistration struct {
	Scope     string                 `json:"scope"`
	RoutePath string                 `json:"route_path"`
	Title     string                 `json:"title"`
	SortOrder int                    `json:"sort_order"`
	Meta      map[string]interface{} `json:"meta"`
}

// RouteRegistration 路由元数据注册。
type RouteRegistration struct {
	Scope   string                 `json:"scope"`
	Path    string                 `json:"path"`
	Methods []string               `json:"methods"`
	Meta    map[string]interface{} `json:"meta"`
}

// EventSubscription 事件订阅元数据。
type EventSubscription struct {
	EventType string                 `json:"event_type"`
	Meta      map[string]interface{} `json:"meta"`
}

// HTTPRequest 传递给插件的统一请求对象。
type HTTPRequest struct {
	PluginID      string                 `json:"plugin_id"`
	Scope         string                 `json:"scope"`
	Method        string                 `json:"method"`
	RoutePattern  string                 `json:"route_pattern"`
	Path          string                 `json:"path"`
	Query         map[string][]string    `json:"query"`
	Headers       map[string]string      `json:"headers"`
	Body          []byte                 `json:"body"`
	UserID        uint                   `json:"user_id"`
	AdminID       uint                   `json:"admin_id"`
	ContextValues map[string]interface{} `json:"context_values"`
}

// HTTPResponse 插件统一响应。
type HTTPResponse struct {
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Data       interface{}            `json:"data"`
	RawBody    []byte                 `json:"raw_body"`
	Meta       map[string]interface{} `json:"meta"`
}

// PaymentCreateRequest 支付插件创建支付请求。
type PaymentCreateRequest struct {
	OrderNo     string                 `json:"order_no"`
	Amount      string                 `json:"amount"`
	Currency    string                 `json:"currency"`
	ChannelID   uint                   `json:"channel_id"`
	ClientIP    string                 `json:"client_ip"`
	CallbackURL string                 `json:"callback_url"`
	ReturnURL   string                 `json:"return_url"`
	Extra       map[string]interface{} `json:"extra"`
}

// PaymentCreateResponse 支付插件创建支付响应。
type PaymentCreateResponse struct {
	PayURL      string                 `json:"pay_url"`
	QRCode      string                 `json:"qr_code"`
	ProviderRef string                 `json:"provider_ref"`
	Payload     map[string]interface{} `json:"payload"`
}

// PaymentQueryRequest 支付插件查询请求。
type PaymentQueryRequest struct {
	ProviderRef string                 `json:"provider_ref"`
	Extra       map[string]interface{} `json:"extra"`
}

// PaymentQueryResponse 支付插件查询响应。
type PaymentQueryResponse struct {
	Status      string                 `json:"status"`
	Amount      string                 `json:"amount"`
	Currency    string                 `json:"currency"`
	ProviderRef string                 `json:"provider_ref"`
	Payload     map[string]interface{} `json:"payload"`
}

// PaymentDriver 支付插件驱动接口。
type PaymentDriver interface {
	CreatePayment(ctx context.Context, req PaymentCreateRequest) (*PaymentCreateResponse, error)
	QueryPayment(ctx context.Context, req PaymentQueryRequest) (*PaymentQueryResponse, error)
}

// Host 宿主暴露给插件的能力边界。
type Host interface {
	GetPluginID() string
	GetVersion() string
	GetDataDir() string
	GetAssetsDir() string
	GetLogDir() string
	GetSQLitePath() string
	ReadConfig() (map[string]interface{}, error)
	SaveConfig(map[string]interface{}) error
	Log(level string, eventType string, message string, details map[string]interface{})
	RegisterPage(PageRegistration) error
	RegisterRoute(RouteRegistration) error
	RegisterEventSubscription(EventSubscription) error
	RegisterPaymentDriver(PaymentDriver) error
}

// ExtensionHost 宿主扩展能力，仅供特权内置插件按需读取额外上下文。
type ExtensionHost interface {
	GetExtension(name string) interface{}
}

// MainDatabaseHost 宿主主库访问能力。
// 说明：
// 1. 插件私有数据仍建议优先落到插件自己的 SQLite。
// 2. 当插件需要直接读写宿主主库时，可按需断言该接口。
// 3. 宿主可基于权限决定是否返回主库句柄；拿不到时返回 nil / 空串。
type MainDatabaseHost interface {
	GetMainDBDriver() string
	GetMainDB() interface{}
}

// Module 插件运行时模块。
type Module interface {
	OnLoad(ctx context.Context) error
	HandlePublic(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)
	HandleAdmin(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)
}

// ScopedModule 支持按作用域统一处理请求。
type ScopedModule interface {
	Handle(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)
}

// LifecycleModule 支持卸载时清理资源。
type LifecycleModule interface {
	OnUnload(ctx context.Context) error
}

// Entrypoint 插件导出入口。
type Entrypoint func(host Host) (Module, error)
