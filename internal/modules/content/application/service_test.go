package application

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

func TestPostServiceUsesInjectedClockForFirstPublish(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 20, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	store := &postStoreStub{}
	service := NewPostService(store, store, nil, fixedClock{now: now})
	published := true

	post, err := service.Create(context.Background(), CreatePostInput{
		Slug:        "clocked-post",
		Type:        constants.PostTypeBlog,
		IsPublished: &published,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if post.PublishedAt == nil || !post.PublishedAt.Equal(now) {
		t.Fatalf("PublishedAt = %v, want %v", post.PublishedAt, now)
	}
	if store.created != post {
		t.Fatal("store did not receive the created post")
	}
}

func TestBannerServicePassesInjectedClockToPublicQuery(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	store := &bannerStoreStub{}
	service := NewBannerService(store, fixedClock{now: now})

	if _, err := service.ListPublic(context.Background(), PublicBannerQuery{Limit: 3}); err != nil {
		t.Fatalf("ListPublic() error = %v", err)
	}
	if !store.validAt.Equal(now) {
		t.Fatalf("ListValidByPosition now = %v, want %v", store.validAt, now)
	}
	if store.position != constants.BannerPositionHomeHero || store.limit != 3 {
		t.Fatalf("public query = (%q, %d), want (%q, 3)", store.position, store.limit, constants.BannerPositionHomeHero)
	}
}

func TestMediaServiceDeletesMetadataBeforeBestEffortFileRemoval(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 2)
	removeErr := errors.New("permission denied")
	store := &mediaStoreStub{
		media:  &domain.Media{ID: 7, Path: "/uploads/post/image.png"},
		events: &events,
	}
	files := &fileStoreStub{events: &events, removeErr: removeErr}
	logger := &warningLoggerStub{}
	service := NewMediaService(store, files, logger)

	if err := service.Delete(context.Background(), 7); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := strings.Join(events, ","); got != "metadata,file" {
		t.Fatalf("side-effect order = %q, want metadata,file", got)
	}
	if files.removedPath != "uploads/post/image.png" {
		t.Fatalf("removed path = %q", files.removedPath)
	}
	if logger.message != "media_delete_file_failed" {
		t.Fatalf("warning message = %q", logger.message)
	}
}

type postStoreStub struct {
	created   *domain.Post
	post      *domain.Post
	lastQuery contract.PostQuery
}

var (
	_ contract.PostStore                = (*postStoreStub)(nil)
	_ contract.PostProductRelationStore = (*postStoreStub)(nil)
)

func (s *postStoreStub) List(_ context.Context, query contract.PostQuery) ([]domain.Post, int64, error) {
	s.lastQuery = query
	return nil, 0, nil
}
func (s *postStoreStub) WithinPostWriteTransaction(_ context.Context, operation func(contract.PostStore, contract.PostProductRelationStore) error) error {
	return operation(s, s)
}
func (s *postStoreStub) GetBySlug(context.Context, string, bool) (*domain.Post, error) {
	return nil, nil
}
func (s *postStoreStub) GetByID(context.Context, string) (*domain.Post, error) {
	return s.post, nil
}
func (s *postStoreStub) Create(_ context.Context, post *domain.Post) error {
	post.ID = 1
	s.created = post
	return nil
}
func (s *postStoreStub) Update(context.Context, *domain.Post) error { return nil }
func (s *postStoreStub) Delete(context.Context, string) error       { return nil }
func (s *postStoreStub) CountBySlug(context.Context, string, *string) (int64, error) {
	return 0, nil
}
func (s *postStoreStub) GetRelatedProductIDs(context.Context, uint) ([]uint, error) {
	return nil, nil
}
func (s *postStoreStub) SetRelatedProductIDs(context.Context, uint, []uint) error {
	return nil
}
func (s *postStoreStub) ListRelatedProducts(context.Context, uint) ([]contract.RelatedProduct, error) {
	return nil, nil
}
func (s *postStoreStub) ListPostsForProduct(context.Context, uint, string, bool, int) ([]contract.RelatedPost, error) {
	return nil, nil
}

type bannerStoreStub struct {
	position string
	limit    int
	validAt  time.Time
}

var _ contract.BannerStore = (*bannerStoreStub)(nil)

func (s *bannerStoreStub) List(context.Context, contract.BannerQuery) ([]domain.Banner, int64, error) {
	return nil, 0, nil
}
func (s *bannerStoreStub) ListValidByPosition(_ context.Context, position string, limit int, now time.Time) ([]domain.Banner, error) {
	s.position = position
	s.limit = limit
	s.validAt = now
	return nil, nil
}
func (s *bannerStoreStub) GetByID(context.Context, string) (*domain.Banner, error) {
	return nil, nil
}
func (s *bannerStoreStub) Create(context.Context, *domain.Banner) error { return nil }
func (s *bannerStoreStub) Update(context.Context, *domain.Banner) error { return nil }
func (s *bannerStoreStub) Delete(context.Context, string) error         { return nil }

type mediaStoreStub struct {
	media     *domain.Media
	events    *[]string
	deleteErr error
}

var _ contract.MediaStore = (*mediaStoreStub)(nil)

func (s *mediaStoreStub) List(context.Context, contract.MediaQuery) ([]domain.Media, int64, error) {
	return nil, 0, nil
}
func (s *mediaStoreStub) GetByID(context.Context, uint) (*domain.Media, error) {
	return s.media, nil
}
func (s *mediaStoreStub) GetByPath(context.Context, string) (*domain.Media, error) {
	return nil, nil
}
func (s *mediaStoreStub) Create(context.Context, *domain.Media) error { return nil }
func (s *mediaStoreStub) Update(context.Context, *domain.Media) error { return nil }
func (s *mediaStoreStub) Delete(context.Context, uint) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	*s.events = append(*s.events, "metadata")
	return nil
}

type fileStoreStub struct {
	events      *[]string
	removedPath string
	removeErr   error
}

var _ contract.FileStore = (*fileStoreStub)(nil)

func (s *fileStoreStub) Stat(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
func (s *fileStoreStub) Open(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (s *fileStoreStub) Remove(path string) error {
	*s.events = append(*s.events, "file")
	s.removedPath = path
	return s.removeErr
}

type warningLoggerStub struct {
	message string
}

var _ contract.WarningLogger = (*warningLoggerStub)(nil)

func (l *warningLoggerStub) Warnw(message string, _ ...interface{}) {
	l.message = message
}

// postCategoryStoreStub 只实现分类展开用到的读取能力，其余方法保持最小实现。
type postCategoryStoreStub struct {
	byID     map[uint]*domain.PostCategory
	children map[uint][]domain.PostCategory
}

var _ contract.PostCategoryStore = (*postCategoryStoreStub)(nil)

func (s *postCategoryStoreStub) ListAll(_ context.Context, parentID *uint) ([]domain.PostCategory, error) {
	if parentID == nil {
		return nil, nil
	}
	return s.children[*parentID], nil
}
func (s *postCategoryStoreStub) ListActive(context.Context) ([]domain.PostCategory, error) {
	return nil, nil
}
func (s *postCategoryStoreStub) ListTree(context.Context) ([]domain.PostCategory, error) {
	return nil, nil
}
func (s *postCategoryStoreStub) GetByID(_ context.Context, id uint) (*domain.PostCategory, error) {
	return s.byID[id], nil
}

// GetBySlug 按 slug 反查分类，与公开列表的分类筛选入口保持一致。
// slug 在分类表内唯一，因此遍历匹配不会出现歧义。
func (s *postCategoryStoreStub) GetBySlug(_ context.Context, slug string) (*domain.PostCategory, error) {
	for _, category := range s.byID {
		if category.Slug == slug {
			return category, nil
		}
	}
	return nil, nil
}
func (s *postCategoryStoreStub) Create(context.Context, *domain.PostCategory) error { return nil }
func (s *postCategoryStoreStub) Update(context.Context, *domain.PostCategory) error { return nil }
func (s *postCategoryStoreStub) UpdateActive(context.Context, uint, bool) error     { return nil }
func (s *postCategoryStoreStub) Delete(context.Context, uint) error                 { return nil }
func (s *postCategoryStoreStub) CountBySlug(context.Context, string, *uint) (int64, error) {
	return 0, nil
}
func (s *postCategoryStoreStub) CountChildren(context.Context, uint) (int64, error) {
	return 0, nil
}
func (s *postCategoryStoreStub) CountPostsByCategory(context.Context, uint) (int64, error) {
	return 0, nil
}

// TestListPublicExpandsCategoryFilterScope 锁定公开列表分类筛选的范围语义。
// nil 与空切片的区别是这里最容易被改坏的地方：空切片必须传递「目标分类不可用」，
// 仓储层据此返回空结果，一旦写成「空切片不筛选」就会把全部文章暴露出去。
//
// 分类标识统一用 slug（对外参数里不再出现自增 id），因此「slug 无匹配」与
// 「分类已停用」是仅有的两种不可用情形，原先按 id 解析才有的「参数非法」分支已随之消失。
func TestListPublicExpandsCategoryFilterScope(t *testing.T) {
	t.Parallel()

	parentID := uint(1)
	childID := uint(2)
	leafID := uint(3)
	inactiveID := uint(4)
	inactiveChildID := uint(5)

	categories := &postCategoryStoreStub{
		byID: map[uint]*domain.PostCategory{
			parentID:        {ID: parentID, Slug: "parent", IsActive: true},
			childID:         {ID: childID, Slug: "child", ParentID: &parentID, IsActive: true},
			inactiveChildID: {ID: inactiveChildID, Slug: "child-off", ParentID: &parentID, IsActive: false},
			leafID:          {ID: leafID, Slug: "leaf", IsActive: true},
			inactiveID:      {ID: inactiveID, Slug: "inactive", IsActive: false},
		},
		children: map[uint][]domain.PostCategory{
			parentID: {
				{ID: childID, Slug: "child", ParentID: &parentID, IsActive: true},
				{ID: inactiveChildID, Slug: "child-off", ParentID: &parentID, IsActive: false},
			},
		},
	}

	cases := []struct {
		name         string
		categorySlug string
		wantNil      bool
		want         []uint
	}{
		{name: "未传参数时不筛选", categorySlug: "", wantNil: true},
		{name: "slug 无匹配时结果为空", categorySlug: "does-not-exist", want: []uint{}},
		{name: "纯数字 slug 不再被当成 id", categorySlug: "99", want: []uint{}},
		{name: "分类已停用时结果为空", categorySlug: "inactive", want: []uint{}},
		{name: "叶子分类只匹配自身", categorySlug: "leaf", want: []uint{leafID}},
		{name: "父分类展开启用子分类", categorySlug: "parent", want: []uint{parentID, childID}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &postStoreStub{}
			service := NewPostService(store, store, categories, fixedClock{now: time.Now()})

			if _, _, err := service.ListPublic(context.Background(), PublicPostQuery{
				CategorySlug: tc.categorySlug,
				Page:         1,
				PageSize:     20,
			}); err != nil {
				t.Fatalf("ListPublic() error = %v", err)
			}

			got := store.lastQuery.CategoryIDs
			if tc.wantNil {
				if got != nil {
					t.Fatalf("CategoryIDs = %v, want nil（不筛选）", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("CategoryIDs = %v, want %v", got, tc.want)
			}
			for index := range got {
				if got[index] != tc.want[index] {
					t.Fatalf("CategoryIDs = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
