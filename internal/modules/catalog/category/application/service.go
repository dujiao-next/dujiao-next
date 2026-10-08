package categoryapp

import (
	"errors"
	"strconv"
	"strings"

	categorycontract "github.com/dujiao-next/internal/modules/catalog/category/contract"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var (
	ErrNotFound      = errors.New("category not found")
	ErrSlugExists    = errors.New("category slug exists")
	ErrParentInvalid = errors.New("category parent invalid")
	ErrInUse         = errors.New("category in use")
)

// Service 承载分类查询、创建、更新、启停和删除用例。
type Service struct {
	repository categorycontract.Repository
}

func NewService(repository categorycontract.Repository) *Service {
	return &Service{repository: repository}
}

// UpsertInput 是创建与更新分类的应用层输入。
type UpsertInput struct {
	ParentID  uint
	Slug      string
	NameJSON  map[string]interface{}
	Icon      string
	SortOrder int
}

func (service *Service) List() ([]categorydomain.Category, error) {
	return service.repository.List()
}

func (service *Service) ListActive() ([]categorydomain.Category, error) {
	return service.repository.ListActive()
}

func (service *Service) SetActive(id string, active bool) (*categorydomain.Category, error) {
	category, err := service.repository.GetByID(id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, ErrNotFound
	}
	if category.IsActive == active {
		return category, nil
	}
	if err := service.repository.UpdateActive(id, active); err != nil {
		return nil, err
	}
	category.IsActive = active
	return category, nil
}

func (service *Service) Create(input UpsertInput) (*categorydomain.Category, error) {
	if err := service.validateParent(nil, input.ParentID); err != nil {
		return nil, err
	}
	// slug 是 URL 片段，前后空白没有意义；后台表单没做 trim，这里兜住，
	// 否则会存出 "xxx " 这种值，前端按 slug 匹配时永远对不上（详见前端
	// categoryAPI.list 的注释）。
	slug := strings.TrimSpace(input.Slug)
	count, err := service.repository.CountBySlug(slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}
	category := categorydomain.Category{
		ParentID:  input.ParentID,
		Slug:      slug,
		NameJSON:  jsonmap.JSON(input.NameJSON),
		Icon:      input.Icon,
		SortOrder: input.SortOrder,
		IsActive:  true,
	}
	if err := service.repository.Create(&category); err != nil {
		return nil, err
	}
	return &category, nil
}

func (service *Service) Update(id string, input UpsertInput) (*categorydomain.Category, error) {
	category, err := service.repository.GetByID(id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, ErrNotFound
	}
	if err := service.validateParent(category, input.ParentID); err != nil {
		return nil, err
	}
	// 同 Create：slug 前后空白一律去掉
	slug := strings.TrimSpace(input.Slug)
	count, err := service.repository.CountBySlug(slug, &id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}
	category.ParentID = input.ParentID
	category.Slug = slug
	category.NameJSON = jsonmap.JSON(input.NameJSON)
	category.Icon = input.Icon
	category.SortOrder = input.SortOrder
	if err := service.repository.Update(category); err != nil {
		return nil, err
	}
	return category, nil
}

func (service *Service) Delete(id string) error {
	category, err := service.repository.GetByID(id)
	if err != nil {
		return err
	}
	if category == nil {
		return ErrNotFound
	}
	childCount, err := service.repository.CountChildren(id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return ErrInUse
	}
	count, err := service.repository.CountProducts(id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrInUse
	}
	return service.repository.Delete(id)
}

func (service *Service) validateParent(category *categorydomain.Category, parentID uint) error {
	if parentID == 0 {
		return nil
	}
	if category != nil && category.ID == parentID {
		return ErrParentInvalid
	}
	parent, err := service.repository.GetByID(strconv.FormatUint(uint64(parentID), 10))
	if err != nil {
		return err
	}
	if parent == nil || parent.ParentID != 0 {
		return ErrParentInvalid
	}
	if category != nil && category.ParentID == 0 {
		childCount, err := service.repository.CountChildren(strconv.FormatUint(uint64(category.ID), 10))
		if err != nil {
			return err
		}
		if childCount > 0 {
			return ErrParentInvalid
		}
	}
	return nil
}
