package gormstore

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/persistence/gormutil"
	"strings"

	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
	"gorm.io/gorm"
)

// MediaStore 使用 GORM 持久化素材元数据。
type MediaStore struct {
	db *gorm.DB
}

var _ contract.MediaStore = (*MediaStore)(nil)

// NewMediaStore 创建素材元数据持久化适配器。
func NewMediaStore(db *gorm.DB) *MediaStore {
	return &MediaStore{db: db}
}

func (s *MediaStore) List(ctx context.Context, query contract.MediaQuery) ([]domain.Media, int64, error) {
	items := make([]domain.Media, 0)
	db := withContext(s.db, ctx)
	statement := db.Model(&domain.Media{}).Where("deleted_at IS NULL")
	if query.Scene != "" {
		statement = statement.Where("scene = ?", query.Scene)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + search + "%"
		condition, argCount := gormutil.BuildLocalizedLikeCondition(db, []string{"name", "filename"}, nil)
		statement = statement.Where(condition, gormutil.RepeatLikeArgs(like, argCount)...)
	}

	var total int64
	if err := statement.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	statement = gormutil.ApplyPagination(statement, query.Page, query.PageSize)
	if err := statement.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *MediaStore) GetByID(ctx context.Context, id uint) (*domain.Media, error) {
	var media domain.Media
	if err := withContext(s.db, ctx).Where("deleted_at IS NULL").First(&media, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &media, nil
}

func (s *MediaStore) GetByPath(ctx context.Context, path string) (*domain.Media, error) {
	var media domain.Media
	if err := withContext(s.db, ctx).Where("path = ? AND deleted_at IS NULL", path).First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &media, nil
}

func (s *MediaStore) Create(ctx context.Context, media *domain.Media) error {
	return withContext(s.db, ctx).Create(media).Error
}

func (s *MediaStore) Update(ctx context.Context, media *domain.Media) error {
	return withContext(s.db, ctx).Save(media).Error
}

func (s *MediaStore) Delete(ctx context.Context, id uint) error {
	return withContext(s.db, ctx).Model(&domain.Media{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", s.db.NowFunc()).Error
}
