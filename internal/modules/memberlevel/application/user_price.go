package application

import (
	"errors"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

var errUserPriceRepositoryMissing = errors.New("user product price repository is not configured")

// WithUserPriceRepository 注入用户专属价仓储；未注入时专属价功能整体不生效。
func (s *Service) WithUserPriceRepository(repo memberlevelcontract.UserPriceRepository) *Service {
	s.userPriceRepo = repo
	return s
}

// --- 用户专属价 CRUD ---

func (s *Service) ListUserPrices(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error) {
	if s.userPriceRepo == nil {
		return nil, 0, errUserPriceRepositoryMissing
	}
	return s.userPriceRepo.ListByUser(userID, page, pageSize)
}

// BatchUpsertUserPrices 校验后批量保存专属价：用户必须存在，价格四舍五入到分后必须大于 0。
func (s *Service) BatchUpsertUserPrices(prices []memberleveldomain.UserProductPrice) error {
	if s.userPriceRepo == nil {
		return errUserPriceRepositoryMissing
	}
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
	if s.userPriceRepo == nil {
		return errUserPriceRepositoryMissing
	}
	return s.userPriceRepo.Delete(id)
}

// ResolveUserPrice 解析用户专属一口价。
// 优先级：SKU 级专属价 > 商品级专属价。
// 只有专属价低于当前基础价时才生效（返回 true），避免基础价下调后用户反而多付钱；
// 命中后调用方应直接以返回价结算，不再叠加活动价、批发价和会员等级价。
func (s *Service) ResolveUserPrice(userID, productID, skuID uint, basePrice decimal.Decimal) (decimal.Decimal, bool) {
	if s.userPriceRepo == nil || userID == 0 || productID == 0 {
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
