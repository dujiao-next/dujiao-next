package container

import (
	"context"
	"strings"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
)

// normalizeProdSellerStorefront applies the agreed USDT settlement defaults
// to an already-created ProdSeller storefront. It deliberately changes only an
// existing legacy USD setting; other site currencies remain operator-owned.
func (c *Container) normalizeProdSellerStorefront() {
	if c == nil || c.SiteConnectionService == nil || c.SettingService == nil {
		return
	}

	connections, _, err := c.SiteConnectionService.List(siteconnectioncontract.ListFilter{})
	if err != nil {
		logger.Warnw("normalize_prodseller_storefront_connections_failed", "error", err)
		return
	}
	prodSellerConnectionIDs := make([]uint, 0)
	for _, conn := range connections {
		if conn.Protocol == constants.ConnectionProtocolProdSeller {
			prodSellerConnectionIDs = append(prodSellerConnectionIDs, conn.ID)
		}
	}
	if len(prodSellerConnectionIDs) == 0 {
		return
	}

	if err := c.SiteConnectionService.NormalizeProdSellerDefaults(); err != nil {
		logger.Warnw("normalize_prodseller_pricing_defaults_failed", "error", err)
	}
	for _, connectionID := range prodSellerConnectionIDs {
		// Reapply the current connection rule on every startup so an existing
		// import cannot remain on a stale cost price after a deployment. The
		// operation is idempotent and never calls the upstream API.
		if c.ProductMappingService != nil {
			if _, err := c.ProductMappingService.ReapplyMarkup(connectionID); err != nil {
				logger.Warnw("normalize_prodseller_markup_reapply_failed", "connection_id", connectionID, "error", err)
			}
			if count, err := c.ProductMappingService.RepairProdSellerCatalog(connectionID); err != nil {
				logger.Warnw("normalize_prodseller_category_repair_failed", "connection_id", connectionID, "error", err)
			} else if count > 0 {
				logger.Infow("normalize_prodseller_category_repair_done", "connection_id", connectionID, "updated", count)
			}
		}
	}

	currency, err := c.SettingService.GetSiteCurrency(constants.SiteCurrencyDefault)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err != nil || (currency != "USD" && currency != "CNY") {
		return
	}
	value, err := c.SettingService.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		logger.Warnw("normalize_prodseller_site_currency_read_failed", "error", err)
		return
	}
	settings := map[string]interface{}{}
	for key, item := range value {
		settings[key] = item
	}
	settings[constants.SettingFieldSiteCurrency] = "USDT"
	if _, err := c.SettingService.Update(constants.SettingKeySiteConfig, settings); err != nil {
		logger.Warnw("normalize_prodseller_site_currency_update_failed", "error", err)
		return
	}
	if err := cache.DelAllPublicConfig(context.Background()); err != nil {
		logger.Warnw("normalize_prodseller_public_config_cache_clear_failed", "error", err)
		return
	}
	logger.Infow("normalize_prodseller_site_currency_done", "currency", "USDT")
}
