package models

import (
	"database/sql/driver"
	"encoding/json"
)

// WholesalePriceTier 商品批发价阶梯。
// SKUID/SKUCode 为空表示全 SKU 通用；MinQuantity 表示购买数量达到该值时，
// UnitPrice 作为每件成交价参与下单计价。
type WholesalePriceTier struct {
	SKUID       uint   `json:"sku_id,omitempty"`
	SKUCode     string `json:"sku_code,omitempty"`
	MinQuantity int    `json:"min_quantity"`
	UnitPrice   Money  `json:"unit_price"`
}

// WholesalePriceTiers 商品批发价阶梯列表。
type WholesalePriceTiers []WholesalePriceTier

// Value 实现 driver.Valuer 接口。
func (w WholesalePriceTiers) Value() (driver.Value, error) {
	if w == nil {
		return nil, nil
	}
	return json.Marshal(w)
}

// Scan 实现 sql.Scanner 接口。
func (w *WholesalePriceTiers) Scan(value interface{}) error {
	if value == nil {
		*w = WholesalePriceTiers{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		*w = WholesalePriceTiers{}
		return nil
	}
	if len(bytes) == 0 {
		*w = WholesalePriceTiers{}
		return nil
	}
	return json.Unmarshal(bytes, w)
}
