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
	created *domain.Post
	post    *domain.Post
}

var (
	_ contract.PostStore                = (*postStoreStub)(nil)
	_ contract.PostProductRelationStore = (*postStoreStub)(nil)
)

func (s *postStoreStub) List(context.Context, contract.PostQuery) ([]domain.Post, int64, error) {
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
func (s *postStoreStub) UpdatePublished(context.Context, string, bool, *time.Time) error {
	return nil
}
func (s *postStoreStub) UpdateCategory(context.Context, string, *uint) error { return nil }
func (s *postStoreStub) Delete(context.Context, string) error                { return nil }
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
