package contract

import "errors"

var (
	ErrMappingNotFound           = errors.New("product mapping not found")
	ErrMappingAlreadyExists      = errors.New("product mapping already exists for this upstream product")
	ErrUpstreamProductNotFound   = errors.New("upstream product not found")
	ErrMappingInactive           = errors.New("product mapping is inactive")
	ErrMediaRecorderRequired     = errors.New("product mapping media recorder is required")
	ErrUpstreamStockInsufficient = errors.New("upstream stock insufficient")
	ErrBatchImportInProgress     = errors.New("batch import already running for this connection")
	ErrUpstreamProductNoCategory = errors.New("upstream product has no category, select a local category manually")
	// ErrUpstreamCategoryUnavailable 表示连接配置了分类加价，但上游分类树拿不到。
	// 这时不能退回默认加价改价，否则子分类商品会被按成本价卖出。
	ErrUpstreamCategoryUnavailable = errors.New("upstream category tree unavailable, prices not recalculated")
)
