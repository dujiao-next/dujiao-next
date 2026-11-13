package adminapitokenapp

// RouteAccess accepts registered Gin route templates, never raw request paths
// or wildcard matches. New routes and methods are denied until reviewed here.
// The second result marks money-changing operations requiring the existing
// payment-compliance guard in addition to the owner's current RBAC permissions.
func RouteAccess(method, route string) (allowed, financial bool) {
	key := method + " " + route
	if _, ok := financialRoutes[key]; ok {
		return true, true
	}
	_, ok := businessRoutes[key]
	return ok, false
}

var businessRoutes = map[string]struct{}{
	"GET /api/v1/admin/products":                        {},
	"GET /api/v1/admin/products/:id":                    {},
	"POST /api/v1/admin/products":                       {},
	"PUT /api/v1/admin/products/:id":                    {},
	"PATCH /api/v1/admin/products/:id":                  {},
	"PATCH /api/v1/admin/products/:id/wholesale-prices": {},
	"DELETE /api/v1/admin/products/:id":                 {},
	"POST /api/v1/admin/products/batch-status":          {},
	"POST /api/v1/admin/products/batch-category":        {},
	"POST /api/v1/admin/products/batch-stock-display":   {},
	"POST /api/v1/admin/products/batch-purchase-type":   {},
	"POST /api/v1/admin/products/batch-delete":          {},
	"GET /api/v1/admin/categories":                      {},
	"POST /api/v1/admin/categories":                     {},
	"PUT /api/v1/admin/categories/:id":                  {},
	"PATCH /api/v1/admin/categories/:id/active":         {},
	"DELETE /api/v1/admin/categories/:id":               {},
	"GET /api/v1/admin/card-secrets":                    {},
	"POST /api/v1/admin/card-secrets/batch":             {},
	"POST /api/v1/admin/card-secrets/import":            {},
	"PUT /api/v1/admin/card-secrets/:id":                {},
	"PATCH /api/v1/admin/card-secrets/batch-status":     {},
	"POST /api/v1/admin/card-secrets/batch-delete":      {},
	"POST /api/v1/admin/card-secrets/export":            {},
	"POST /api/v1/admin/card-secrets/export-available":  {},
	"GET /api/v1/admin/card-secrets/stats":              {},
	"GET /api/v1/admin/card-secrets/batches":            {},
	"GET /api/v1/admin/card-secrets/template":           {},
	"GET /api/v1/admin/orders":                          {},
	"GET /api/v1/admin/orders/:id":                      {},
	"GET /api/v1/admin/orders/:id/fulfillment/download": {},
	"POST /api/v1/admin/fulfillments":                   {},
	"GET /api/v1/admin/order-refunds":                   {},
	"GET /api/v1/admin/order-refunds/:id":               {},
	"GET /api/v1/admin/payments":                        {},
	"GET /api/v1/admin/payments/:id":                    {},
	"GET /api/v1/admin/users/:id/wallet":                {},
	"GET /api/v1/admin/users/:id/wallet/transactions":   {},
	"GET /api/v1/admin/wallet/recharges":                {},
}

var financialRoutes = map[string]struct{}{
	"PATCH /api/v1/admin/orders/:id":                       {},
	"POST /api/v1/admin/orders/:id/refund-to-wallet":       {},
	"POST /api/v1/admin/orders/:id/manual-refund":          {},
	"POST /api/v1/admin/orders/:id/manual-confirm-payment": {},
	"PATCH /api/v1/admin/order-refunds/:id/payment-fee":    {},
	"POST /api/v1/admin/users/:id/wallet/adjust":           {},
}
