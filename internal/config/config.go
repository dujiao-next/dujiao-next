package config

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/passwordpolicy"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	App          AppConfig          `mapstructure:"app"`
	Server       ServerConfig       `mapstructure:"server"`
	Log          LogConfig          `mapstructure:"log"`
	Database     DatabaseConfig     `mapstructure:"database"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	UserJWT      JWTConfig          `mapstructure:"user_jwt"`
	Bootstrap    BootstrapConfig    `mapstructure:"bootstrap"`
	TelegramAuth TelegramAuthConfig `mapstructure:"telegram_auth"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Queue        QueueConfig        `mapstructure:"queue"`
	Upload       UploadConfig       `mapstructure:"upload"`
	CORS         CORSConfig         `mapstructure:"cors"`
	Security     SecurityConfig     `mapstructure:"security"`
	Email        EmailConfig        `mapstructure:"email"`
	Order        OrderConfig        `mapstructure:"order"`
	Captcha      CaptchaConfig      `mapstructure:"captcha"`
	Web          WebConfig          `mapstructure:"web"`
	Reseller     ResellerConfig     `mapstructure:"reseller"`
}

// AppConfig 应用级配置
type AppConfig struct {
	SecretKey  string `mapstructure:"secret_key"`  // 通用加密密钥（AES-256，用于加密存储敏感信息）
	TOTPIssuer string `mapstructure:"totp_issuer"` // 2FA 验证器中显示的发行方名称（避免使用 & 等特殊字符，Google Authenticator 解析容错差）
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug / release
}

// LogConfig 日志配置
type LogConfig struct {
	Dir        string `mapstructure:"dir"`
	Filename   string `mapstructure:"filename"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
}

// ToLoggerOptions 转换为 logger 配置
func (c LogConfig) ToLoggerOptions() logger.Options {
	return logger.Options{
		Dir:        c.Dir,
		Filename:   c.Filename,
		MaxSizeMB:  c.MaxSizeMB,
		MaxBackups: c.MaxBackups,
		MaxAgeDays: c.MaxAgeDays,
		Compress:   c.Compress,
	}
}

// DatabasePoolConfig 数据库连接池配置
type DatabasePoolConfig struct {
	MaxOpenConns           int `mapstructure:"max_open_conns"`
	MaxIdleConns           int `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeSeconds int `mapstructure:"conn_max_lifetime_seconds"`
	ConnMaxIdleTimeSeconds int `mapstructure:"conn_max_idle_time_seconds"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver string             `mapstructure:"driver"` // 数据库驱动（sqlite/postgres）
	DSN    string             `mapstructure:"dsn"`    // 数据库连接串
	Pool   DatabasePoolConfig `mapstructure:"pool"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	SecretKey             string `mapstructure:"secret"`
	ExpireHours           int    `mapstructure:"expire_hours"`
	RememberMeExpireHours int    `mapstructure:"remember_me_expire_hours"`
}

// BootstrapConfig 启动初始化配置
type BootstrapConfig struct {
	DefaultAdminUsername string `mapstructure:"default_admin_username"`
	DefaultAdminPassword string `mapstructure:"default_admin_password"`
}

// TelegramAuthConfig Telegram 登录配置
type TelegramAuthConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	BotUsername        string `mapstructure:"bot_username"`
	BotToken           string `mapstructure:"bot_token"`
	ClientSecret       string `mapstructure:"client_secret"`
	OIDCRedirectURI    string `mapstructure:"oidc_redirect_uri"`
	MiniAppURL         string `mapstructure:"mini_app_url"`
	LoginExpireSeconds int    `mapstructure:"login_expire_seconds"`
	ReplayTTLSeconds   int    `mapstructure:"replay_ttl_seconds"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
}

// QueueConfig 异步队列配置
type QueueConfig struct {
	Enabled              bool           `mapstructure:"enabled"`
	Host                 string         `mapstructure:"host"`
	Port                 int            `mapstructure:"port"`
	Password             string         `mapstructure:"password"`
	DB                   int            `mapstructure:"db"`
	Concurrency          int            `mapstructure:"concurrency"`
	Queues               map[string]int `mapstructure:"queues"`
	UpstreamSyncInterval string         `mapstructure:"upstream_sync_interval"` // 上游库存同步间隔，如 "5m"、"10m"，默认 "5m"
}

// OrderConfig 订单配置
type OrderConfig struct {
	PaymentExpireMinutes int `mapstructure:"payment_expire_minutes"`
	MaxRefundDays        int `mapstructure:"max_refund_days"`
}

// EmailConfig 邮件服务配置
type EmailConfig struct {
	Enabled    bool             `mapstructure:"enabled"`
	Host       string           `mapstructure:"host"`
	Port       int              `mapstructure:"port"`
	Username   string           `mapstructure:"username"`
	Password   string           `mapstructure:"password"`
	From       string           `mapstructure:"from"`
	FromName   string           `mapstructure:"from_name"`
	UseTLS     bool             `mapstructure:"use_tls"`
	UseSSL     bool             `mapstructure:"use_ssl"`
	VerifyCode VerifyCodeConfig `mapstructure:"verify_code"`
}

// VerifyCodeConfig 邮箱验证码配置
type VerifyCodeConfig struct {
	ExpireMinutes       int `mapstructure:"expire_minutes"`
	SendIntervalSeconds int `mapstructure:"send_interval_seconds"`
	MaxAttempts         int `mapstructure:"max_attempts"`
	Length              int `mapstructure:"length"`
}

// CaptchaConfig 验证码配置
type CaptchaConfig struct {
	Provider  string                 `mapstructure:"provider"`
	Scenes    CaptchaSceneConfig     `mapstructure:"scenes"`
	Image     CaptchaImageConfig     `mapstructure:"image"`
	Turnstile CaptchaTurnstileConfig `mapstructure:"turnstile"`
}

// CaptchaSceneConfig 验证码场景开关
type CaptchaSceneConfig struct {
	Login            bool `mapstructure:"login"`
	RegisterSendCode bool `mapstructure:"register_send_code"`
	ResetSendCode    bool `mapstructure:"reset_send_code"`
	GuestCreateOrder bool `mapstructure:"guest_create_order"`
	GiftCardRedeem   bool `mapstructure:"gift_card_redeem"`
}

// CaptchaImageConfig 图片验证码配置
type CaptchaImageConfig struct {
	Length        int `mapstructure:"length"`
	Width         int `mapstructure:"width"`
	Height        int `mapstructure:"height"`
	NoiseCount    int `mapstructure:"noise_count"`
	ShowLine      int `mapstructure:"show_line"`
	ExpireSeconds int `mapstructure:"expire_seconds"`
	MaxStore      int `mapstructure:"max_store"`
}

// CaptchaTurnstileConfig Cloudflare Turnstile 配置
type CaptchaTurnstileConfig struct {
	SiteKey   string `mapstructure:"site_key"`
	SecretKey string `mapstructure:"secret_key"`
	VerifyURL string `mapstructure:"verify_url"`
	TimeoutMS int    `mapstructure:"timeout_ms"`
}

// UploadConfig 文件上传配置
type UploadConfig struct {
	MaxSize           int64    `mapstructure:"max_size"`
	AllowedTypes      []string `mapstructure:"allowed_types"`
	AllowedExtensions []string `mapstructure:"allowed_extensions"`
	MaxWidth          int      `mapstructure:"max_width"`
	MaxHeight         int      `mapstructure:"max_height"`
}

// CORSConfig 跨域配置
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

var (
	defaultCORSAllowedOrigins = []string{"*"}
	defaultCORSAllowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodPatch}
	defaultCORSAllowedHeaders = []string{
		"Content-Type",
		"Content-Length",
		"Accept-Encoding",
		"Authorization",
		"Cache-Control",
		"X-Requested-With",
		"X-CSRF-Token",
	}
)

// DefaultCORSAllowedOrigins 返回默认允许跨域来源（副本，避免调用方修改全局默认值）
func DefaultCORSAllowedOrigins() []string {
	return append([]string(nil), defaultCORSAllowedOrigins...)
}

// DefaultCORSAllowedMethods 返回默认允许跨域方法（副本，避免调用方修改全局默认值）
func DefaultCORSAllowedMethods() []string {
	return append([]string(nil), defaultCORSAllowedMethods...)
}

// DefaultCORSAllowedHeaders 返回默认允许跨域请求头（副本，避免调用方修改全局默认值）
func DefaultCORSAllowedHeaders() []string {
	return append([]string(nil), defaultCORSAllowedHeaders...)
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	LoginRateLimit LoginRateLimitConfig `mapstructure:"login_rate_limit"`
	PasswordPolicy PasswordPolicyConfig `mapstructure:"password_policy"`
}

// LoginRateLimitConfig 登录限流配置
type LoginRateLimitConfig struct {
	WindowSeconds int `mapstructure:"window_seconds"`
	MaxAttempts   int `mapstructure:"max_attempts"`
	BlockSeconds  int `mapstructure:"block_seconds"`
}

// PasswordPolicyConfig 密码策略配置
type PasswordPolicyConfig struct {
	MinLength      int  `mapstructure:"min_length"`
	RequireUpper   bool `mapstructure:"require_upper"`
	RequireLower   bool `mapstructure:"require_lower"`
	RequireNumber  bool `mapstructure:"require_number"`
	RequireSpecial bool `mapstructure:"require_special"`
}

// ValidationPolicy converts configuration into the shared identity policy.
func (p PasswordPolicyConfig) ValidationPolicy() passwordpolicy.Policy {
	return passwordpolicy.Policy{
		MinLength:      p.MinLength,
		RequireUpper:   p.RequireUpper,
		RequireLower:   p.RequireLower,
		RequireNumber:  p.RequireNumber,
		RequireSpecial: p.RequireSpecial,
	}
}

// WebConfig 仅在 fullstack 二进制模式下生效。
// 默认构建模式（无 -tags fullstack）下这些字段不被任何代码读取。
type WebConfig struct {
	// AdminPath 后台访问路径前缀，例如 "/admin" 或 "/dj-mgmt-7x9k2"。
	// 校验规则见 internal/web.ValidateAdminPath。
	AdminPath string `mapstructure:"admin_path"`
	// UserSPADisabled 为 true 时不挂载用户站 SPA（主站与分销 storefront 一并关闭），
	// 只保留 admin + API。默认 false（即默认启用 user SPA）。
	UserSPADisabled bool `mapstructure:"user_spa_disabled"`
}

// ResellerConfig 分销商模式配置。
type ResellerConfig struct {
	Enabled              bool     `mapstructure:"enabled"`
	MainHosts            []string `mapstructure:"main_hosts"`
	TrustedForwardedHost bool     `mapstructure:"trusted_forwarded_host"`
	SubdomainBase        string   `mapstructure:"subdomain_base"`
	SelfApplyEnabled     bool     `mapstructure:"self_apply_enabled"`
	// SettlementConfirmDays 分销利润入账后转为可提现的确认天数（0 表示即时到账）。
	SettlementConfirmDays int `mapstructure:"settlement_confirm_days"`
}

// Load 从 config.yml 加载配置
func Load() *Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")     // 从当前目录查找
	viper.AddConfigPath("./")    // 备用路径
	viper.AddConfigPath("../")   // 如果从 cmd/server 运行
	viper.AddConfigPath("./etc") // etc 文件夹

	// 设置默认值（可选）
	viper.SetDefault("app.secret_key", "change-me-32-byte-secret-key!!")
	viper.SetDefault("app.totp_issuer", "Dujiao-Next")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("log.dir", "")
	viper.SetDefault("log.filename", "app.log")
	viper.SetDefault("log.max_size_mb", 100)
	viper.SetDefault("log.max_backups", 7)
	viper.SetDefault("log.max_age_days", 30)
	viper.SetDefault("log.compress", true)
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dsn", "./db/dujiao.db")
	viper.SetDefault("database.pool.max_open_conns", 1)
	viper.SetDefault("database.pool.max_idle_conns", 1)
	viper.SetDefault("database.pool.conn_max_lifetime_seconds", 0)
	viper.SetDefault("database.pool.conn_max_idle_time_seconds", 0)
	viper.SetDefault("jwt.secret", "change-me-in-production")
	viper.SetDefault("jwt.expire_hours", 24)
	viper.SetDefault("user_jwt.secret", "user-change-me-in-production")
	viper.SetDefault("user_jwt.expire_hours", 24)
	viper.SetDefault("user_jwt.remember_me_expire_hours", 168)
	viper.SetDefault("bootstrap.default_admin_username", "")
	viper.SetDefault("bootstrap.default_admin_password", "")
	viper.SetDefault("telegram_auth.enabled", false)
	viper.SetDefault("telegram_auth.bot_username", "")
	viper.SetDefault("telegram_auth.bot_token", "")
	viper.SetDefault("telegram_auth.login_expire_seconds", 300)
	viper.SetDefault("telegram_auth.replay_ttl_seconds", 300)
	viper.SetDefault("redis.enabled", true)
	viper.SetDefault("redis.host", "127.0.0.1")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.prefix", constants.RedisPrefixDefault)
	viper.SetDefault("queue.enabled", true)
	viper.SetDefault("queue.host", "127.0.0.1")
	viper.SetDefault("queue.port", 6379)
	viper.SetDefault("queue.password", "")
	viper.SetDefault("queue.db", 1)
	viper.SetDefault("queue.concurrency", 10)
	viper.SetDefault("queue.queues", map[string]int{
		"default":  10,
		"critical": 5,
	})
	viper.SetDefault("upload.max_size", 10485760)
	viper.SetDefault("upload.allowed_types", []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
	})
	viper.SetDefault("upload.allowed_extensions", []string{
		".jpg",
		".jpeg",
		".png",
		".gif",
		".webp",
	})
	viper.SetDefault("upload.max_width", 4096)
	viper.SetDefault("upload.max_height", 4096)
	viper.SetDefault("cors.allowed_origins", DefaultCORSAllowedOrigins())
	viper.SetDefault("cors.allowed_methods", DefaultCORSAllowedMethods())
	viper.SetDefault("cors.allowed_headers", DefaultCORSAllowedHeaders())
	viper.SetDefault("cors.allow_credentials", true)
	viper.SetDefault("cors.max_age", 600)
	viper.SetDefault("security.login_rate_limit.window_seconds", 300)
	viper.SetDefault("security.login_rate_limit.max_attempts", 5)
	viper.SetDefault("security.login_rate_limit.block_seconds", 900)
	viper.SetDefault("security.password_policy.min_length", 8)
	viper.SetDefault("security.password_policy.require_upper", true)
	viper.SetDefault("security.password_policy.require_lower", true)
	viper.SetDefault("security.password_policy.require_number", true)
	viper.SetDefault("security.password_policy.require_special", false)
	viper.SetDefault("email.enabled", false)
	viper.SetDefault("email.host", "")
	viper.SetDefault("email.port", 587)
	viper.SetDefault("email.username", "")
	viper.SetDefault("email.password", "")
	viper.SetDefault("email.from", "")
	viper.SetDefault("email.from_name", "")
	viper.SetDefault("email.use_tls", true)
	viper.SetDefault("email.use_ssl", false)
	viper.SetDefault("email.verify_code.expire_minutes", 10)
	viper.SetDefault("email.verify_code.send_interval_seconds", 60)
	viper.SetDefault("email.verify_code.max_attempts", 5)
	viper.SetDefault("email.verify_code.length", 6)
	viper.SetDefault("order.payment_expire_minutes", 15)
	viper.SetDefault("order.max_refund_days", 30)
	viper.SetDefault("captcha.provider", "none")
	viper.SetDefault("captcha.scenes.login", false)
	viper.SetDefault("captcha.scenes.register_send_code", false)
	viper.SetDefault("captcha.scenes.reset_send_code", false)
	viper.SetDefault("captcha.scenes.guest_create_order", false)
	viper.SetDefault("captcha.scenes.gift_card_redeem", false)
	viper.SetDefault("captcha.image.length", 5)
	viper.SetDefault("captcha.image.width", 240)
	viper.SetDefault("captcha.image.height", 80)
	viper.SetDefault("captcha.image.noise_count", 2)
	viper.SetDefault("captcha.image.show_line", 2)
	viper.SetDefault("captcha.image.expire_seconds", 300)
	viper.SetDefault("captcha.image.max_store", 10240)
	viper.SetDefault("captcha.turnstile.site_key", "")
	viper.SetDefault("captcha.turnstile.secret_key", "")
	viper.SetDefault("captcha.turnstile.verify_url", "https://challenges.cloudflare.com/turnstile/v0/siteverify")
	viper.SetDefault("captcha.turnstile.timeout_ms", 2000)
	viper.SetDefault("web.admin_path", "/admin")
	viper.SetDefault("reseller.enabled", false)
	viper.SetDefault("reseller.main_hosts", []string{"localhost", "127.0.0.1", "::1"})
	viper.SetDefault("reseller.trusted_forwarded_host", false)
	viper.SetDefault("reseller.subdomain_base", "")
	viper.SetDefault("reseller.self_apply_enabled", true)
	viper.SetDefault("reseller.settlement_confirm_days", 7)

	// 环境变量支持
	viper.AutomaticEnv()                                   // 自动读取环境变量
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // 将 . 替换为 _ (例如 server.port -> SERVER_PORT)

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		logger.Warnw("config_file_read_failed",
			"error", err,
			"fallback", "env_or_defaults",
		)
	} else {
		logger.Infow("config_file_loaded", "file", viper.ConfigFileUsed())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Errorw("config_unmarshal_failed", "error", err)
		panic(fmt.Errorf("配置解析失败: %w", err))
	}

	return &cfg
}
