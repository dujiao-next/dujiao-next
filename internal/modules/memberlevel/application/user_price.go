package application

import (
	"strconv"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// 折扣模式允许的百分比区间：1% 起步（挡住把"八折"误填成 8 的手滑），
// 100 及以上没有任何意义（等于原价），直接拒绝。
var (
	minUserPriceDiscountRate = decimal.NewFromInt(1)
	maxUserPriceDiscountRate = decimal.NewFromInt(100)
)

// --- 用户专属价 CRUD ---

func (s *Service) ListUserPrices(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error) {
	return s.userPriceRepo.ListByUser(userID, page, pageSize)
}

// BatchUpsertUserPrices 校验后批量保存专属价，两种定价模式二选一：
//   - 一口价：PriceAmount 四舍五入到分后必须大于 0，DiscountRate 必须为 0；
//   - 百分比折扣：DiscountRate 必须在 [1, 99]，PriceAmount 必须为 0。
//
// 两个字段同时有值直接判为非法，而不是隐式取其一：后台误填的代价是资金损失，
// 宁可让保存失败。规范化时把未使用的字段清零，避免切换模式后留下脏值。
func (s *Service) BatchUpsertUserPrices(prices []memberleveldomain.UserProductPrice) error {
	checkedUsers := make(map[uint]struct{}, len(prices))
	normalized := make([]memberleveldomain.UserProductPrice, 0, len(prices))
	for _, item := range prices {
		if item.UserID == 0 || item.ProductID == 0 {
			return memberlevelcontract.ErrUserPriceInvalid
		}
		price := item.PriceAmount.Decimal.Round(2)
		rate := item.DiscountRate.Decimal.Round(2)
		hasPrice := price.GreaterThan(decimal.Zero)
		hasRate := rate.GreaterThan(decimal.Zero)
		switch {
		case hasPrice && hasRate:
			// 固定价与折扣互斥，同时填写视为误操作。
			return memberlevelcontract.ErrUserPriceInvalid
		case hasRate:
			if rate.LessThan(minUserPriceDiscountRate) || rate.GreaterThanOrEqual(maxUserPriceDiscountRate) {
				return memberlevelcontract.ErrUserPriceInvalid
			}
			item.PriceAmount = money.FromDecimal(decimal.Zero)
			item.DiscountRate = money.FromDecimal(rate)
		case hasPrice:
			item.PriceAmount = money.FromDecimal(price)
			item.DiscountRate = money.FromDecimal(decimal.Zero)
		default:
			return memberlevelcontract.ErrUserPriceInvalid
		}
		if _, ok := checkedUsers[item.UserID]; !ok {
			user, err := s.userRepo.GetByID(item.UserID)
			if err != nil {
				return err
			}
			if user == nil {
				return memberlevelcontract.ErrUserNotFound
			}
			checkedUsers[item.UserID] = struct{}{}
		}
		normalized = append(normalized, item)
	}
	return s.userPriceRepo.BatchUpsert(normalized)
}

func (s *Service) DeleteUserPrice(id uint) error {
	return s.userPriceRepo.Delete(id)
}

// UserPriceRule 是一条专属价规则：一口价与百分比折扣二选一。
type UserPriceRule struct {
	Fixed decimal.Decimal // 一口价，0 表示未设置
	Rate  decimal.Decimal // 折扣百分比（1~99），0 表示未设置
}

// NewUserPriceRule 把存储字段规范化成规则：一口价优先于折扣，
// 两者都不可用时返回零值（IsActive 为 false）。
func NewUserPriceRule(fixed, rate decimal.Decimal) UserPriceRule {
	fixed = fixed.Round(2)
	rate = rate.Round(2)
	if fixed.GreaterThan(decimal.Zero) {
		return UserPriceRule{Fixed: fixed}
	}
	if rate.GreaterThanOrEqual(minUserPriceDiscountRate) && rate.LessThan(maxUserPriceDiscountRate) {
		return UserPriceRule{Rate: rate}
	}
	return UserPriceRule{}
}

// IsFixed 表示这条规则是一口价；只有一口价命中时才需要隐藏活动价，
// 折扣价随原价浮动，压掉活动价会让用户看到比实付更高的价格。
func (r UserPriceRule) IsFixed() bool {
	return r.Fixed.GreaterThan(decimal.Zero)
}

// IsActive 表示这条规则可用。
func (r UserPriceRule) IsActive() bool {
	return r.IsFixed() || r.Rate.GreaterThan(decimal.Zero)
}

// Price 按基础价算出专属价，只有严格低于 base 时才算命中。
// 折扣口径与会员等级折扣一致：base × rate ÷ 100，四舍五入到分。
// base ≤ 0 或折扣后不足一分时视为未命中，避免把商品卖成 0 元。
func (r UserPriceRule) Price(base decimal.Decimal) (decimal.Decimal, bool) {
	if base.LessThanOrEqual(decimal.Zero) {
		return base, false
	}
	if r.IsFixed() {
		if r.Fixed.LessThan(base) {
			return r.Fixed, true
		}
		return base, false
	}
	if r.Rate.GreaterThan(decimal.Zero) && r.Rate.LessThan(maxUserPriceDiscountRate) {
		price := base.Mul(r.Rate).Div(decimal.NewFromInt(100)).Round(2)
		if price.GreaterThan(decimal.Zero) && price.LessThan(base) {
			return price, true
		}
	}
	return base, false
}

// UserPriceBook 是用户全部专属价的内存快照，键为「商品ID:SKU ID」（商品级用 sku=0）。
// 目录接口一页要算几十个 SKU，逐个查库是 N+1；先在 handler 里读一次，之后纯内存解析。
type UserPriceBook map[string]UserPriceRule

func userPriceKey(productID, skuID uint) string {
	return strconv.FormatUint(uint64(productID), 10) + ":" + strconv.FormatUint(uint64(skuID), 10)
}

// SetFixed / SetDiscount 供快照的构造方（测试、上游对接）直接写入规则。
func (b UserPriceBook) SetFixed(productID, skuID uint, price decimal.Decimal) {
	b[userPriceKey(productID, skuID)] = NewUserPriceRule(price, decimal.Zero)
}

func (b UserPriceBook) SetDiscount(productID, skuID uint, rate decimal.Decimal) {
	b[userPriceKey(productID, skuID)] = NewUserPriceRule(decimal.Zero, rate)
}

// LoadUserPriceBook 一次读出用户的全部专属价；用户未登录时返回空快照，不查库。
func (s *Service) LoadUserPriceBook(userID uint) (UserPriceBook, error) {
	book := UserPriceBook{}
	if userID == 0 {
		return book, nil
	}
	prices, _, err := s.userPriceRepo.ListByUser(userID, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, p := range prices {
		rule := NewUserPriceRule(p.PriceAmount.Decimal, p.DiscountRate.Decimal)
		if rule.IsActive() {
			book[userPriceKey(p.ProductID, p.SKUID)] = rule
		}
	}
	return book, nil
}

// Rule 取命中优先级的规则：SKU 级 > 商品级，不关心基础价。
// 目录需要在拿到价格之外，再判断这条规则是一口价还是折扣。
func (b UserPriceBook) Rule(productID, skuID uint) (UserPriceRule, bool) {
	if len(b) == 0 {
		return UserPriceRule{}, false
	}
	if skuID > 0 {
		if rule, ok := b[userPriceKey(productID, skuID)]; ok {
			return rule, true
		}
	}
	rule, ok := b[userPriceKey(productID, 0)]
	return rule, ok
}

// Resolve 在快照里取专属价，优先级 SKU 级 > 商品级，且必须低于 base。
// SKU 级那行算出的价格不低于 base 时不再回退商品级（与旧的一口价行为保持一致）。
func (b UserPriceBook) Resolve(productID, skuID uint, base decimal.Decimal) (decimal.Decimal, bool) {
	rule, ok := b.Rule(productID, skuID)
	if !ok {
		return base, false
	}
	return rule.Price(base)
}
