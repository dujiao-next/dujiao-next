package gormstore

import (
	"fmt"
	"slices"
	"testing"
	"time"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCategoryStoreTest(t *testing.T) *CategoryStore {
	t.Helper()
	dsn := fmt.Sprintf("file:category_repository_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}); err != nil {
		t.Fatalf("migrate category failed: %v", err)
	}
	return NewCategoryStore(db)
}

func TestCategoryStoreUpdatesRootAndChildrenTogether(t *testing.T) {
	store := setupCategoryStoreTest(t)
	create := func(slug string, parentID uint) uint {
		t.Helper()
		row := categorydomain.Category{Slug: slug, ParentID: parentID, NameJSON: jsonmap.JSON{}}
		if err := store.Create(&row); err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
		return row.ID
	}
	root := create("root", 0)
	childA := create("child-a", root)
	childB := create("child-b", root)
	otherRoot := create("other-root", 0)
	otherChild := create("other-child", otherRoot)

	assertActive := func(id uint, want bool) {
		t.Helper()
		row, err := store.GetByID(fmt.Sprint(id))
		if err != nil || row == nil || row.IsActive != want {
			t.Fatalf("id %d: got %+v and %v, want active=%t", id, row, err, want)
		}
	}
	update := func(id uint, active bool) {
		t.Helper()
		if err := store.UpdateActive(fmt.Sprint(id), active); err != nil {
			t.Fatalf("set id %d active=%t: %v", id, active, err)
		}
	}

	update(root, false)
	for _, id := range []uint{root, childA, childB} {
		assertActive(id, false)
	}
	update(childA, true)
	assertActive(root, false)
	assertActive(childB, false)
	rows, err := store.ListActive()
	if err != nil {
		t.Fatalf("list active categories: %v", err)
	}
	got := make([]uint, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.ID)
	}
	if want := []uint{otherRoot, otherChild}; !slices.Equal(got, want) {
		t.Fatalf("visible IDs = %v, want %v", got, want)
	}
	update(root, false)
	assertActive(childA, false)

	update(root, true)
	for _, id := range []uint{root, childA, childB, otherRoot, otherChild} {
		assertActive(id, true)
	}
}

func TestCategoryStoreListSortOrderDescending(t *testing.T) {
	repo := setupCategoryStoreTest(t)

	high := &categorydomain.Category{
		Slug:      "high",
		NameJSON:  jsonmap.JSON{"zh-CN": "high"},
		SortOrder: 100,
	}
	low := &categorydomain.Category{
		Slug:      "low",
		NameJSON:  jsonmap.JSON{"zh-CN": "low"},
		SortOrder: 1,
	}
	if err := repo.Create(high); err != nil {
		t.Fatalf("create high sort category failed: %v", err)
	}
	if err := repo.Create(low); err != nil {
		t.Fatalf("create low sort category failed: %v", err)
	}

	rows, err := repo.List()
	if err != nil {
		t.Fatalf("list categories failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(rows))
	}
	if rows[0].Slug != "high" || rows[1].Slug != "low" {
		t.Fatalf("expected high sort_order first, got %s then %s", rows[0].Slug, rows[1].Slug)
	}
}

// TestCategoryStoreRestoreRevivesSoftDeletedSlug 验证软删除分类的复活链路：
// GetBySlug 不可见 → GetBySlugUnscoped 可见 → Restore 后重新可见且仅存一行。
// 上游映射自动建分类依赖该语义避免 UNIQUE 冲突。
func TestCategoryStoreRestoreRevivesSoftDeletedSlug(t *testing.T) {
	store := setupCategoryStoreTest(t)

	existing := &categorydomain.Category{
		ParentID: 0,
		Slug:     "softdel-streaming",
		NameJSON: jsonmap.JSON{"zh-CN": "旧名"},
		IsActive: true,
	}
	if err := store.Create(existing); err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	if err := store.Delete(fmt.Sprintf("%d", existing.ID)); err != nil {
		t.Fatalf("soft delete category failed: %v", err)
	}

	if got, err := store.GetBySlug("softdel-streaming"); err != nil || got != nil {
		t.Fatalf("expected slug to be invisible after soft delete, got=%v err=%v", got, err)
	}

	deleted, err := store.GetBySlugUnscoped("softdel-streaming")
	if err != nil {
		t.Fatalf("GetBySlugUnscoped failed: %v", err)
	}
	if deleted == nil || deleted.ID != existing.ID {
		t.Fatalf("expected soft-deleted row visible via unscoped lookup, got %+v", deleted)
	}

	deleted.NameJSON = jsonmap.JSON{"zh-CN": "新名"}
	deleted.IsActive = true
	if err := store.Restore(deleted); err != nil {
		t.Fatalf("restore category failed: %v", err)
	}

	visible, err := store.GetBySlug("softdel-streaming")
	if err != nil {
		t.Fatalf("GetBySlug after restore failed: %v", err)
	}
	if visible == nil || visible.ID != existing.ID {
		t.Fatalf("expected category visible after restore, got %+v", visible)
	}
	if name, _ := visible.NameJSON["zh-CN"].(string); name != "新名" {
		t.Fatalf("expected NameJSON refreshed to 新名, got %q", name)
	}
	if !visible.IsActive {
		t.Fatalf("expected restored category to be active")
	}

	rows, err := store.List()
	if err != nil {
		t.Fatalf("list categories failed: %v", err)
	}
	count := 0
	for _, row := range rows {
		if row.Slug == "softdel-streaming" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 visible row for slug after restore, got %d", count)
	}
}
