package application

import (
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	"github.com/shopspring/decimal"
)

// Service 会员等级服务
type Service struct {
	levelRepo     memberlevelcontract.LevelRepository
	priceRepo     memberlevelcontract.PriceRepository
	userRepo      memberlevelcontract.UserRepository
	userPriceRepo memberlevelcontract.UserPriceRepository
}

// NewService 创建会员等级服务
func NewService(
	levelRepo memberlevelcontract.LevelRepository,
	priceRepo memberlevelcontract.PriceRepository,
	userRepo memberlevelcontract.UserRepository,
) *Service {
	return &Service{
		levelRepo: levelRepo,
		priceRepo: priceRepo,
		userRepo:  userRepo,
	}
}

// --- 等级 CRUD ---

func (s *Service) GetByID(id uint) (*memberleveldomain.MemberLevel, error) {
	return s.levelRepo.GetByID(id)
}

func (s *Service) ListLevels(filter memberlevelcontract.ListFilter) ([]memberleveldomain.MemberLevel, int64, error) {
	return s.levelRepo.List(filter)
}

func (s *Service) ListActiveLevels() ([]memberleveldomain.MemberLevel, error) {
	return s.levelRepo.ListAllActive()
}

func (s *Service) CreateLevel(level *memberleveldomain.MemberLevel) error {
	existing, err := s.levelRepo.GetBySlug(level.Slug)
	if err != nil {
		return err
	}
	if existing != nil {
		return memberlevelcontract.ErrSlugExists
	}
	if err := s.ensureActiveSortOrderAvailable(level); err != nil {
		return err
	}
	if level.IsDefault {
		if err := s.levelRepo.ClearDefault(0); err != nil {
			return err
		}
	}
	return s.levelRepo.Create(level)
}

func (s *Service) UpdateLevel(level *memberleveldomain.MemberLevel) error {
	existing, err := s.levelRepo.GetBySlug(level.Slug)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != level.ID {
		return memberlevelcontract.ErrSlugExists
	}
	if err := s.ensureActiveSortOrderAvailable(level); err != nil {
		return err
	}
	if level.IsDefault {
		if err := s.levelRepo.ClearDefault(level.ID); err != nil {
			return err
		}
	}
	return s.levelRepo.Update(level)
}

func (s *Service) ensureActiveSortOrderAvailable(level *memberleveldomain.MemberLevel) error {
	if level == nil || !level.IsActive {
		return nil
	}
	existing, err := s.levelRepo.GetActiveBySortOrder(level.SortOrder, level.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return memberlevelcontract.ErrSortOrderUsed
	}
	return nil
}

func (s *Service) DeleteLevel(id uint) error {
	level, err := s.levelRepo.GetByID(id)
	if err != nil {
		return err
	}
	if level == nil {
		return memberlevelcontract.ErrNotFound
	}
	if level.IsDefault {
		return memberlevelcontract.ErrDeleteDefault
	}
	return s.levelRepo.Delete(id)
}

// --- 等级价 CRUD ---

func (s *Service) GetLevelPricesByProduct(productID uint) ([]memberleveldomain.MemberLevelPrice, error) {
	return s.priceRepo.ListByProduct(productID)
}

func (s *Service) BatchUpsertLevelPrices(prices []memberleveldomain.MemberLevelPrice) error {
	return s.priceRepo.BatchUpsert(prices)
}

func (s *Service) DeleteLevelPrice(id uint) error {
	return s.priceRepo.Delete(id)
}

// --- 价格解析 ---

// ResolveMemberPrice 解析会员价
// 优先级：SKU级覆盖 > 商品级覆盖 > 等级折扣率 * basePrice
// 返回会员价和会员优惠金额
func (s *Service) ResolveMemberPrice(levelID, productID, skuID uint, basePrice decimal.Decimal) (memberPrice decimal.Decimal, memberDiscount decimal.Decimal) {
	if levelID == 0 {
		return basePrice, decimal.Zero
	}

	// 检查会员等级是否启用，停用的等级不享受任何优惠
	level, err := s.levelRepo.GetByID(levelID)
	if err != nil || level == nil || !level.IsActive {
		return basePrice, decimal.Zero
	}

	// 查找 SKU 级覆盖
	if skuID > 0 {
		skuPrice, err := s.priceRepo.GetByLevelAndProductAndSKU(levelID, productID, skuID)
		if err == nil && skuPrice != nil && skuPrice.PriceAmount.Decimal.GreaterThan(decimal.Zero) {
			mp := skuPrice.PriceAmount.Decimal.Round(2)
			if mp.LessThan(basePrice) {
				return mp, basePrice.Sub(mp).Round(2)
			}
			return basePrice, decimal.Zero
		}
	}

	// 查找商品级覆盖
	productPrice, err := s.priceRepo.GetByLevelAndProductAndSKU(levelID, productID, 0)
	if err == nil && productPrice != nil && productPrice.PriceAmount.Decimal.GreaterThan(decimal.Zero) {
		mp := productPrice.PriceAmount.Decimal.Round(2)
		if mp.LessThan(basePrice) {
			return mp, basePrice.Sub(mp).Round(2)
		}
		return basePrice, decimal.Zero
	}

	// 使用等级折扣率
	rate := level.DiscountRate.Decimal
	if rate.LessThanOrEqual(decimal.Zero) || rate.GreaterThanOrEqual(decimal.NewFromInt(100)) {
		return basePrice, decimal.Zero
	}
	mp := basePrice.Mul(rate).Div(decimal.NewFromInt(100)).Round(2)
	if mp.LessThan(basePrice) {
		return mp, basePrice.Sub(mp).Round(2)
	}
	return basePrice, decimal.Zero
}

// ResolveMemberPriceForProducts 批量解析会员价（用于商品列表）
func (s *Service) ResolveMemberPriceForProducts(levelID uint, productIDs []uint) (map[uint][]memberleveldomain.MemberLevelPrice, error) {
	if levelID == 0 || len(productIDs) == 0 {
		return nil, nil
	}
	// 检查会员等级是否启用
	level, err := s.levelRepo.GetByID(levelID)
	if err != nil || level == nil || !level.IsActive {
		return nil, nil
	}
	prices, err := s.priceRepo.ListByLevelAndProducts(levelID, productIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[uint][]memberleveldomain.MemberLevelPrice)
	for _, p := range prices {
		result[p.ProductID] = append(result[p.ProductID], p)
	}
	return result, nil
}

// --- 等级升级 ---

// CheckAndUpgrade 检查用户是否满足升级条件，只升不降
func (s *Service) CheckAndUpgrade(userID uint) error {
	const maxUpgradeAttempts = 3
	for attempt := 0; attempt < maxUpgradeAttempts; attempt++ {
		user, err := s.userRepo.GetByID(userID)
		if err != nil || user == nil {
			return err
		}

		levels, err := s.levelRepo.ListAllActive()
		if err != nil {
			return err
		}
		if len(levels) == 0 {
			return nil
		}

		target, err := s.findUpgradeTarget(user, levels)
		if err != nil || target == nil {
			return err
		}
		affected, err := s.userRepo.UpdateMemberLevelIfCurrent(user.ID, user.MemberLevelID, target.ID)
		if err != nil {
			return err
		}
		if affected > 0 {
			return nil
		}
	}
	return nil
}

func (s *Service) findUpgradeTarget(user *userdomain.User, levels []memberleveldomain.MemberLevel) (*memberleveldomain.MemberLevel, error) {
	if user == nil {
		return nil, nil
	}
	currentSortOrder, ok, err := s.resolveCurrentSortOrder(user.MemberLevelID, levels)
	if err != nil || !ok {
		return nil, err
	}
	for i := range levels {
		level := &levels[i]
		if level.ID == user.MemberLevelID {
			continue
		}
		if level.SortOrder <= currentSortOrder {
			continue
		}
		if s.meetsThreshold(user, level) {
			return level, nil
		}
	}
	return nil, nil
}

func (s *Service) resolveCurrentSortOrder(levelID uint, activeLevels []memberleveldomain.MemberLevel) (int, bool, error) {
	if levelID == 0 {
		return -1 << 60, true, nil
	}
	for _, level := range activeLevels {
		if level.ID == levelID {
			return level.SortOrder, true, nil
		}
	}
	level, err := s.levelRepo.GetByID(levelID)
	if err != nil || level == nil {
		return 0, false, err
	}
	return level.SortOrder, true, nil
}

// meetsThreshold 判断用户是否满足等级阈值（充值累计 OR 消费累计）
func (s *Service) meetsThreshold(user *userdomain.User, level *memberleveldomain.MemberLevel) bool {
	rechargeThreshold := level.RechargeThreshold.Decimal
	spendThreshold := level.SpendThreshold.Decimal

	if rechargeThreshold.GreaterThan(decimal.Zero) &&
		user.TotalRecharged.Decimal.GreaterThanOrEqual(rechargeThreshold) {
		return true
	}
	if spendThreshold.GreaterThan(decimal.Zero) &&
		user.TotalSpent.Decimal.GreaterThanOrEqual(spendThreshold) {
		return true
	}
	return false
}

// OnRechargeCompleted 充值到账后触发
func (s *Service) OnRechargeCompleted(userID uint, amount decimal.Decimal) error {
	if userID == 0 {
		return nil
	}
	if err := s.userRepo.IncrementTotalRecharged(userID, amount); err != nil {
		return err
	}
	return s.CheckAndUpgrade(userID)
}

// OnOrderPaid 订单支付成功后触发
func (s *Service) OnOrderPaid(userID uint, amount decimal.Decimal) error {
	if userID == 0 {
		return nil
	}
	if err := s.userRepo.IncrementTotalSpent(userID, amount); err != nil {
		return err
	}
	return s.CheckAndUpgrade(userID)
}

// AssignDefaultLevel 为新用户分配默认等级
func (s *Service) AssignDefaultLevel(userID uint) error {
	defaultLevel, err := s.levelRepo.GetDefault()
	if err != nil || defaultLevel == nil {
		return err
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return err
	}
	if user.MemberLevelID == 0 {
		user.MemberLevelID = defaultLevel.ID
		return s.userRepo.Update(user)
	}
	return nil
}

// SetUserLevel 管理员手动设置用户等级
func (s *Service) SetUserLevel(userID, levelID uint) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return memberlevelcontract.ErrUserNotFound
	}
	if levelID > 0 {
		level, err := s.levelRepo.GetByID(levelID)
		if err != nil {
			return err
		}
		if level == nil {
			return memberlevelcontract.ErrNotFound
		}
	}
	user.MemberLevelID = levelID
	return s.userRepo.Update(user)
}

// BackfillDefaultLevel 为所有未分配等级的老用户批量分配默认等级，返回影响行数
func (s *Service) BackfillDefaultLevel() (int64, error) {
	defaultLevel, err := s.levelRepo.GetDefault()
	if err != nil {
		return 0, err
	}
	if defaultLevel == nil {
		return 0, memberlevelcontract.ErrNotFound
	}
	return s.userRepo.AssignDefaultMemberLevel(defaultLevel.ID)
}
