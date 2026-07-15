package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SiteConnection 对接连接表
type SiteConnection struct {
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
	AutoSyncPrice      bool            `gorm:"not null;default:false" json:"auto_sync_price"`                       // 同步时自动更新本地价格
	ExcludedProductIDs string          `gorm:"type:text" json:"excluded_product_ids"`                               // JSON 数组，存储要排除的上游商品 ID
	CreatedAt          time.Time       `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time       `gorm:"index" json:"updated_at"`
	DeletedAt          gorm.DeletedAt  `gorm:"index" json:"-"`
}

// TableName 指定表名
func (SiteConnection) TableName() string {
	return "site_connections"
}

// IsProductExcluded 判断指定上游商品 ID 是否在排除列表中
func (c *SiteConnection) IsProductExcluded(upstreamProductID uint) bool {
	set := c.GetExcludedProductIDSet()
	if set == nil {
		return false
	}
	return set[upstreamProductID]
}

// GetExcludedProductIDSet 返回排除 ID 的集合（map[uint]bool），用于 O(1) 批量查找。
// 仅解析一次 JSON，调用方应缓存结果用于循环场景。
func (c *SiteConnection) GetExcludedProductIDSet() map[uint]bool {
	if c.ExcludedProductIDs == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(c.ExcludedProductIDs), &ids); err != nil {
		return nil
	}
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// ValidateExcludedProductIDs 校验排除列表是否为有效 JSON 正整数数组或空字符串
func ValidateExcludedProductIDs(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return fmt.Errorf("excluded_product_ids must be a valid JSON array of positive integers")
	}
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		if id == 0 {
			return fmt.Errorf("excluded_product_ids contains invalid item: 0 (must be positive integer)")
		}
	}
	return nil
}
