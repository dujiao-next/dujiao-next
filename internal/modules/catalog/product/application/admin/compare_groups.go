package productadmin

import (
	"strings"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// CompareGroupRepository 是对比维度组用例所需的持久化端口。
type CompareGroupRepository interface {
	productcontract.CompareGroupRepository
}

// CompareGroupService 负责对比维度组的后台读写。
type CompareGroupService struct {
	groups CompareGroupRepository
}

func NewCompareGroupService(groups CompareGroupRepository) *CompareGroupService {
	return &CompareGroupService{groups: groups}
}

// CompareGroupInput 是新建/更新维度组的入参。
type CompareGroupInput struct {
	NameJSON   map[string]interface{}
	CategoryID *uint
	Dimensions map[string]interface{}
	IsActive   *bool
	SortOrder  int
}

// List 返回全部维度组。
func (s *CompareGroupService) List() ([]productdomain.CompareGroup, error) {
	return s.groups.ListAll()
}

// Get 按 ID 返回维度组。
func (s *CompareGroupService) Get(id uint) (*productdomain.CompareGroup, error) {
	return s.groups.GetByID(id)
}

// Create 新建维度组。
func (s *CompareGroupService) Create(input CompareGroupInput) (*productdomain.CompareGroup, error) {
	item := &productdomain.CompareGroup{
		NameJSON:   jsonmap.JSON(input.NameJSON),
		CategoryID: input.CategoryID,
		Dimensions: jsonmap.JSON(input.Dimensions),
		IsActive:   input.IsActive == nil || *input.IsActive,
		SortOrder:  input.SortOrder,
	}
	if err := s.groups.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

// Update 更新维度组。
func (s *CompareGroupService) Update(id uint, input CompareGroupInput) (*productdomain.CompareGroup, error) {
	item, err := s.groups.GetByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, productcontract.ErrNotFound
	}
	if input.NameJSON != nil {
		item.NameJSON = jsonmap.JSON(input.NameJSON)
	}
	item.CategoryID = input.CategoryID
	if input.Dimensions != nil {
		item.Dimensions = jsonmap.JSON(input.Dimensions)
	}
	if input.IsActive != nil {
		item.IsActive = *input.IsActive
	}
	item.SortOrder = input.SortOrder
	if err := s.groups.Update(item); err != nil {
		return nil, err
	}
	return item, nil
}

// Delete 删除维度组。
func (s *CompareGroupService) Delete(id uint) error {
	return s.groups.Delete(id)
}

// NormalizeDimensionKey 给新维度生成稳定 key（前端没填时兜底）。
func NormalizeDimensionKey(key, fallback string) string {
	key = strings.TrimSpace(key)
	if key != "" {
		return key
	}
	return fallback
}
