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
)
