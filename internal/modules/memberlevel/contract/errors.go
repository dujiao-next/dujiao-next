package contract

import "errors"

var (
	ErrNotFound      = errors.New("member_level_not_found")
	ErrSlugExists    = errors.New("member_level_slug_exists")
	ErrSortOrderUsed = errors.New("member_level_sort_order_used")
	ErrDeleteDefault = errors.New("member_level_cannot_delete_default")
	ErrUserNotFound  = errors.New("user_not_found")

	ErrUserPriceInvalid = errors.New("user_product_price_invalid")
)
