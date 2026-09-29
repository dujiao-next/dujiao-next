package contract

// ListFilter 描述卡密库存筛选和分页条件。
type ListFilter struct {
	ProductID uint
	SKUID     uint
	BatchID   uint
	Status    string
	Secret    string
	BatchNo   string
	Page      int
	PageSize  int
}

// BatchStatusCount 是按批次和状态聚合的数量。
type BatchStatusCount struct {
	BatchID uint   `gorm:"column:batch_id"`
	Status  string `gorm:"column:status"`
	Total   int64  `gorm:"column:total"`
}

// SKUStockCount 是按商品、SKU 和状态聚合的库存数量。
type SKUStockCount struct {
	ProductID uint   `gorm:"column:product_id"`
	SKUID     uint   `gorm:"column:sku_id"`
	Status    string `gorm:"column:status"`
	IsLoop    bool   `gorm:"column:is_loop"`
	Total     int64  `gorm:"column:total"`
}
