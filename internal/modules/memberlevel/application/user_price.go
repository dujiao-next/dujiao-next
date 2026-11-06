package application

import (
	"strconv"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// --- 用户专属价 CRUD ---

func (s *Service) ListUserPrices(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error) {
	return s.userPriceRepo.ListByUser(userID, page, pageSize)
}

// BatchUpsertUserPrices 校验后批量保存专属价：用户必须存在，价格四舍五入到分后必须大于 0。
func (s *Service) BatchUpsertUserPrices(prices []memberleveldomain.UserProductPrice) error {
	checkedUsers := make(map[uint]struct{}, len(prices))
	normalized := make([]memberleveldomain.UserProductPrice, 0, len(prices))
	for _, item := range prices {
		if item.UserID == 0 || item.ProductID == 0 {
			return memberlevelcontract.ErrUserPriceInvalid
		}
		rounded := item.PriceAmount.Decimal.Round(2)
		if !rounded.GreaterThan(decimal.Zero) {
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
		item.PriceAmount = money.FromDecimal(rounded)
		normalized = append(normalized, item)
	}
	return s.userPriceRepo.BatchUpsert(normalized)
}

func (s *Service) DeleteUserPrice(id uint) error {
	return s.userPriceRepo.Delete(id)
}

// UserPriceBook 是用户全部专属价的内存快照，键为「商品ID:SKU ID」（商品级用 sku=0）。
// 目录接口一页要算几十个 SKU，逐个查库是 N+1；先在 handler 里读一次，之后纯内存解析。
type UserPriceBook map[string]decimal.Decimal

func userPriceKey(productID, skuID uint) string {
	return strconv.FormatUint(uint64(productID), 10) + ":" + strconv.FormatUint(uint64(skuID), 10)
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
		price := p.PriceAmount.Decimal.Round(2)
		if price.GreaterThan(decimal.Zero) {
			book[userPriceKey(p.ProductID, p.SKUID)] = price
		}
	}
	return book, nil
}

// Resolve 在快照里取专属价，优先级 SKU 级 > 商品级，且必须低于 base。
func (b UserPriceBook) Resolve(productID, skuID uint, base decimal.Decimal) (decimal.Decimal, bool) {
	if len(b) == 0 {
		return base, false
	}
	if skuID > 0 {
		if price, ok := b[userPriceKey(productID, skuID)]; ok {
			if price.LessThan(base) {
				return price, true
			}
			return base, false
		}
	}
	if price, ok := b[userPriceKey(productID, 0)]; ok && price.LessThan(base) {
		return price, true
	}
	return base, false
}

// ResolveUserPrice 解析用户专属一口价。
// 优先级：SKU 级专属价 > 商品级专属价。
// 只有专属价低于当前基础价时才生效（返回 true），避免基础价下调后用户反而多付钱；
// 命中后调用方应直接以返回价结算，不再叠加活动价、批发价和会员等级价。
func (s *Service) ResolveUserPrice(userID, productID, skuID uint, basePrice decimal.Decimal) (decimal.Decimal, bool) {
	if userID == 0 || productID == 0 {
		return basePrice, false
	}
	scopes := []uint{0}
	if skuID > 0 {
		scopes = []uint{skuID, 0}
	}
	for _, scope := range scopes {
		row, err := s.userPriceRepo.GetByUserAndProductAndSKU(userID, productID, scope)
		if err != nil || row == nil {
			continue
		}
		price := row.PriceAmount.Decimal.Round(2)
		if price.GreaterThan(decimal.Zero) && price.LessThan(basePrice) {
			return price, true
		}
		return basePrice, false
	}
	return basePrice, false
}
