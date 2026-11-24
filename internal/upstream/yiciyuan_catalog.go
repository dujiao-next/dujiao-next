package upstream

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

func (a *YiciyuanAdapter) catalog(ctx context.Context) ([]map[string]any, error) {
	var groups []map[string]any
	err := a.request(ctx, "/shared/commodity/items", nil, &groups)
	return groups, err
}
func (a *YiciyuanAdapter) ListCategories(ctx context.Context) (*CategoryListResult, error) {
	groups, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	result := &CategoryListResult{Supported: true, Categories: []UpstreamCategory{}}
	for _, group := range groups {
		id := ycyUint(group["id"])
		if id == 0 {
			continue
		}
		result.Categories = append(result.Categories, UpstreamCategory{ID: id, Slug: fmt.Sprintf("yiciyuan-%d", id), Name: ycyLocalized(ycyText(group["name"]))})
	}
	return result, nil
}
func (a *YiciyuanAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	groups, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	all := []UpstreamProduct{}
	for _, group := range groups {
		children, _ := group["children"].([]any)
		for _, raw := range children {
			item := ycyMap(raw)
			if ycyUint(item["id"]) == 0 || ycyText(item["code"]) == "" {
				return nil, fmt.Errorf("yiciyuan: catalog item has no ID or code")
			}
			all = append(all, ycyProduct(item, ycyUint(group["id"])))
		}
	}
	page := opts.Page
	if page < 1 {
		page = 1
	}
	size := opts.PageSize
	if size < 1 {
		size = 20
	}
	if size > 1000 {
		size = 1000
	}
	start := (page - 1) * size
	if start > len(all) || start < 0 {
		start = len(all)
	}
	end := start + size
	if end > len(all) {
		end = len(all)
	}
	return &ProductListResult{Total: len(all), Items: all[start:end], IncludesInactive: false}, nil
}

func (a *YiciyuanAdapter) GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error) {
	groups, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	var code string
	var categoryID uint
	for _, group := range groups {
		children, _ := group["children"].([]any)
		for _, raw := range children {
			item := ycyMap(raw)
			if ycyUint(item["id"]) == productID {
				code = ycyText(item["code"])
				categoryID = ycyUint(group["id"])
			}
		}
	}
	// items excludes unavailable products. Absence is not proof of deletion.
	if code == "" {
		return nil, ErrUpstreamProductUnavailable
	}
	var item map[string]any
	if err := a.request(ctx, "/shared/commodity/item", url.Values{"code": {code}}, &item); err != nil {
		return nil, err
	}
	if ycyUint(item["id"]) != productID || ycyText(item["code"]) != code {
		return nil, fmt.Errorf("yiciyuan: product identity changed")
	}
	product := ycyProduct(item, categoryID)
	minimum, maximum := ycyInt(item["minimum"]), ycyInt(item["maximum"])
	if minimum < 0 {
		minimum = 0
	}
	if maximum < 0 {
		maximum = 0
	}
	product.MinPurchaseQuantity = &minimum
	product.MaxPurchaseQuantity = &maximum
	variants, err := ycyVariants(ycyMap(item["config"]))
	if err != nil {
		return nil, err
	}
	schema, widgetFields, err := ycyWidgetSchema(item)
	if err != nil {
		return nil, err
	}
	product.ManualFormSchema = schema
	min := ycyInt(item["minimum"])
	if min < 1 {
		min = 1
	}
	product.SKUs = make([]UpstreamSKU, 0, len(variants))
	for _, variant := range variants {
		values := ycySelection(code, variant.race, variant.sku)
		values.Set("num", strconv.Itoa(min))
		var quote map[string]any
		if err := a.request(ctx, "/shared/commodity/valuation", values, &quote); err != nil {
			return nil, err
		}
		amount, err := decimal.NewFromString(ycyText(quote["price"]))
		if err != nil || amount.IsNegative() {
			return nil, fmt.Errorf("yiciyuan: invalid variant price")
		}
		price := amount.Div(decimal.NewFromInt(int64(min))).Round(2).StringFixed(2)
		currency := strings.ToUpper(ycyText(quote["currency_code"]))
		if currency == "" {
			currency = "CNY"
		}
		if len(product.SKUs) > 0 && product.Currency != currency {
			return nil, fmt.Errorf("yiciyuan: inconsistent variant currencies")
		}
		product.Currency = currency
		var stock map[string]any
		if err := a.request(ctx, "/shared/commodity/stock", values, &stock); err != nil {
			return nil, err
		}
		quantity, err := strconv.Atoi(ycyText(stock["stock"]))
		if err != nil || quantity < -1 {
			return nil, fmt.Errorf("yiciyuan: invalid variant stock")
		}
		identity, _ := json.Marshal(map[string]any{"code": code, "race": variant.race, "sku": variant.sku})
		hash := sha256.Sum256(identity)
		id := uint(binary.BigEndian.Uint64(hash[:8]) >> 12)
		if id == 0 {
			id = 1
		}
		status := constants.ProductStockStatusInStock
		if quantity == -1 {
			status = constants.ProductStockStatusUnlimited
		} else if quantity == 0 {
			status = constants.ProductStockStatusOutOfStock
		}
		metadata := jsonmap.JSON{"code": code, "race": variant.race, "sku": variant.sku, "unit_price": price, "currency": currency, "minimum": min, "maximum": ycyInt(item["maximum"]), "delivery_way": ycyInt(item["delivery_way"]), "widget_fields": widgetFields}
		metadata["reference_amount"] = amount.String()
		metadata["reference_quantity"] = min
		product.SKUs = append(product.SKUs, UpstreamSKU{ID: id, SKUCode: "YCY-" + hex.EncodeToString(hash[:12]), SpecValues: variant.specs, PriceAmount: price, StockStatus: status, StockQuantity: quantity, IsActive: product.IsActive, Metadata: metadata})
		if len(product.SKUs) == 1 {
			product.PriceAmount = price
		} else if old, _ := decimal.NewFromString(product.PriceAmount); amount.Div(decimal.NewFromInt(int64(min))).LessThan(old) {
			product.PriceAmount = price
		}
	}
	return &product, nil
}

func ycyProduct(item map[string]any, categoryID uint) UpstreamProduct {
	id := ycyUint(item["id"])
	fulfillment := constants.FulfillmentTypeAuto
	if ycyInt(item["delivery_way"]) != 0 {
		fulfillment = constants.FulfillmentTypeManual
	}
	price := ycyText(item["user_price"])
	if price == "" {
		price = ycyText(item["price"])
	}
	if price == "" {
		price = "0.00"
	}
	active := true
	if status, ok := item["status"]; ok {
		active = ycyText(status) == "1"
	}
	images := []string{}
	for _, key := range []string{"cover", "image"} {
		if value := ycyText(item[key]); value != "" {
			images = append(images, value)
			break
		}
	}
	return UpstreamProduct{ID: id, Slug: fmt.Sprintf("yiciyuan-%d", id), Title: ycyLocalized(ycyText(item["name"])), Content: ycyLocalized(ycyText(item["description"])), PriceAmount: price, Currency: "CNY", FulfillmentType: fulfillment, IsActive: active, CategoryID: categoryID, Images: images}
}
func ycyLocalized(value string) jsonmap.JSON {
	return jsonmap.JSON{"zh-CN": value, "zh-TW": value, "en-US": value}
}
func ycyUint(value any) uint { n, _ := strconv.ParseUint(ycyText(value), 10, 64); return uint(n) }

type ycyVariant struct {
	race  string
	sku   map[string]any
	specs jsonmap.JSON
}

// Both flat INI keys and the nested form produced by the vendor's Ini::toArray are accepted.
func ycyVariants(config map[string]any) ([]ycyVariant, error) {
	categories := ycyMap(config["category"])
	races := ycyKeys(categories)
	if len(races) == 0 {
		races = []string{""}
	}
	dimensions := map[string]map[string]any{}
	for key, raw := range ycyMap(config["sku"]) {
		if nested := ycyMap(raw); len(nested) > 0 {
			dimensions[key] = nested
			continue
		}
		dimension, option, ok := strings.Cut(key, ".")
		if !ok || dimension == "" || option == "" {
			return nil, fmt.Errorf("yiciyuan: unsupported SKU configuration")
		}
		if dimensions[dimension] == nil {
			dimensions[dimension] = map[string]any{}
		}
		dimensions[dimension][option] = raw
	}
	variants := []ycyVariant{}
	for _, race := range races {
		specs := jsonmap.JSON{}
		if race != "" {
			specs["种类"] = race
		}
		variants = append(variants, ycyVariant{race: race, sku: map[string]any{}, specs: specs})
	}
	for _, dimension := range ycyKeysOfMaps(dimensions) {
		if dimension == "" || strings.ContainsAny(dimension, "[]") {
			return nil, fmt.Errorf("yiciyuan: invalid SKU dimension")
		}
		options := ycyKeys(dimensions[dimension])
		if len(options) == 0 || len(variants) > 100/len(options) {
			return nil, fmt.Errorf("yiciyuan: more than 100 SKU combinations; split this upstream product first")
		}
		next := []ycyVariant{}
		for _, variant := range variants {
			for _, option := range options {
				sku := map[string]any{}
				specs := jsonmap.JSON{}
				for k, v := range variant.sku {
					sku[k] = v
				}
				for k, v := range variant.specs {
					specs[k] = v
				}
				sku[dimension] = option
				specs[dimension] = option
				next = append(next, ycyVariant{race: variant.race, sku: sku, specs: specs})
			}
		}
		variants = next
	}
	if len(variants) > 100 {
		return nil, fmt.Errorf("yiciyuan: more than 100 SKU combinations")
	}
	return variants, nil
}
func ycyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func ycyKeysOfMaps(values map[string]map[string]any) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ycyWidgetNameAllowed(name string) bool {
	if name == "" || strings.ContainsAny(name, "[]") {
		return false
	}
	switch name {
	case "app_id", "app_key", "sign", "num", "code", "shared_code", "race", "sku", "card_id", "request_no", "contact", "password", "coupon", "device", "pay_id", "item_id":
		return false
	}
	return true
}
func ycyWidgetSchema(item map[string]any) (jsonmap.JSON, map[string]any, error) {
	fields := []any{}
	mapping := map[string]any{}
	contactType := ycyInt(item["contact_type"])
	if contactType > 0 {
		kind := "text"
		if contactType == 1 {
			kind = "phone"
		}
		if contactType == 2 {
			kind = "email"
		}
		fields = append(fields, map[string]any{"key": "ycy_contact", "type": kind, "label": map[string]any{"zh-CN": "联系方式", "zh-TW": "聯絡方式", "en-US": "Contact"}, "placeholder": map[string]any{}, "required": true})
	}
	widgets, _ := item["widget"].([]any)
	for i, raw := range widgets {
		widget := ycyMap(raw)
		name := ycyText(widget["name"])
		if !ycyWidgetNameAllowed(name) {
			return nil, nil, fmt.Errorf("yiciyuan: invalid widget field name")
		}
		key := fmt.Sprintf("ycy_widget_%d", i)
		label := ycyText(widget["title"])
		if label == "" {
			label = name
		}
		kind := ycyText(widget["type"])
		// Choice widgets carry vendor-specific value mappings. Do not silently replace them with text inputs.
		if kind != "" && kind != "text" && kind != "input" && kind != "textarea" {
			return nil, nil, fmt.Errorf("yiciyuan: unsupported widget type %q; configure a supported upstream product", kind)
		}
		if kind == "" || kind == "input" {
			kind = "text"
		}
		fields = append(fields, map[string]any{"key": key, "type": kind, "label": map[string]any{"zh-CN": label, "zh-TW": label, "en-US": label}, "placeholder": map[string]any{}, "required": true})
		mapping[key] = name
	}
	return jsonmap.JSON{"fields": fields}, mapping, nil
}
