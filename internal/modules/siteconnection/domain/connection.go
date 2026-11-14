package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/shopspring/decimal"
)

// Connection 表示一个上游站点连接。
type Connection struct {
	ID                 uint            `gorm:"primarykey" json:"id"`
	Name               string          `gorm:"type:varchar(100);not null" json:"name"`
	BaseURL            string          `gorm:"type:varchar(500);not null" json:"base_url"`
	ApiKey             string          `gorm:"type:varchar(64);not null" json:"api_key"`
	ApiSecret          string          `gorm:"type:varchar(512);not null" json:"-"` // AES-256 加密存储
	Protocol           string          `gorm:"type:varchar(20);not null;default:'dujiao-next'" json:"protocol"`
	CallbackURL        string          `gorm:"type:varchar(500)" json:"callback_url"`
	Status             string          `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	LastPingAt         *time.Time      `json:"last_ping_at,omitempty"`
	LastPingOK         bool            `gorm:"not null;default:false" json:"last_ping_ok"`
	RetryMax           int             `gorm:"not null;default:5" json:"retry_max"`
	RetryIntervals     string          `gorm:"type:varchar(200);not null;default:'[30,60,300]'" json:"retry_intervals"`
	ExchangeRate       decimal.Decimal `gorm:"type:decimal(16,6);not null;default:1" json:"exchange_rate"`          // 汇率，上游价格 × 汇率 = 本地价格，默认 1
	PriceMarkupPercent decimal.Decimal `gorm:"type:decimal(10,4);not null;default:0" json:"price_markup_percent"`   // 加价百分比，如 100 = +100%（翻倍）
	PriceRoundingMode  string          `gorm:"type:varchar(20);not null;default:'none'" json:"price_rounding_mode"` // none / ceil_int / ceil_tenth
	CategoryMarkups    jsonmap.JSON    `gorm:"type:json" json:"category_markups"`                                   // 按上游分类设置的加价百分比 {上游分类ID: 百分比}，未配置的分类回落到 PriceMarkupPercent；子分类继承父分类
	AutoSyncPrice      bool            `gorm:"not null;default:false" json:"auto_sync_price"`                       // 同步时自动更新本地价格
	CreatedAt          time.Time       `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time       `gorm:"index" json:"updated_at"`
	DeletedAt          *time.Time      `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Connection) TableName() string {
	return "site_connections"
}
