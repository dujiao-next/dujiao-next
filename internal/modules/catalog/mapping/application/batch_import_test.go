package application

import (
	"strings"
	"testing"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// fakeCategoryRepo 以内存实现分类查找/复活端口，验证 findOrCreateLocalCategory 的调用编排。
// GetBySlug/GetBySlugUnscoped/Restore 的真实数据库语义由 catalog category store 测试覆盖。
type fakeCategoryRepo struct {
	visible  map[string]*categorydomain.Category
	deleted  map[string]*categorydomain.Category
	restored int
}

func (f *fakeCategoryRepo) GetByID(id string) (*categorydomain.Category, error) { return nil, nil }

func (f *fakeCategoryRepo) CountChildren(categoryID string) (int64, error) { return 0, nil }

func (f *fakeCategoryRepo) GetBySlug(slug string) (*categorydomain.Category, error) {
	return f.visible[slug], nil
}

func (f *fakeCategoryRepo) GetBySlugUnscoped(slug string) (*categorydomain.Category, error) {
	return f.deleted[slug], nil
}

func (f *fakeCategoryRepo) Restore(category *categorydomain.Category) error {
	f.restored++
	delete(f.deleted, category.Slug)
	f.visible[category.Slug] = category
	return nil
}

// TestFindOrCreateLocalCategoryRestoresSoftDeleted 验证当存在同 slug 的软删除分类时，
// 自动创建路径会复活该分类（并刷新名称/父级/启用状态），而非走创建流程触发 UNIQUE 冲突。
func TestFindOrCreateLocalCategoryRestoresSoftDeleted(t *testing.T) {
	softDeleted := &categorydomain.Category{
		ID:       7,
		ParentID: 0,
		Slug:     "softdel-streaming",
		NameJSON: jsonmap.JSON{"zh-CN": "旧名"},
		IsActive: false,
	}
	repo := &fakeCategoryRepo{
		visible: map[string]*categorydomain.Category{},
		deleted: map[string]*categorydomain.Category{"softdel-streaming": softDeleted},
	}
	svc := &Service{categories: repo}

	restored, err := svc.findOrCreateLocalCategory("softdel-streaming", jsonmap.JSON{"zh-CN": "新名"}, 3)
	if err != nil {
		t.Fatalf("findOrCreateLocalCategory failed: %v", err)
	}
	if restored == nil || restored.ID != softDeleted.ID {
		t.Fatalf("expected restored category id=%d, got=%+v", softDeleted.ID, restored)
	}
	if repo.restored != 1 {
		t.Fatalf("expected exactly one restore call, got %d", repo.restored)
	}
	if name, _ := restored.NameJSON["zh-CN"].(string); name != "新名" {
		t.Fatalf("expected NameJSON refreshed to 新名, got %q", name)
	}
	if !restored.IsActive {
		t.Fatalf("expected restored category to be active")
	}
	if restored.ParentID != 3 {
		t.Fatalf("expected restored category parent refreshed to 3, got %d", restored.ParentID)
	}
	if repo.visible["softdel-streaming"] == nil {
		t.Fatalf("expected category to be visible after restore")
	}
}

// TestFindOrCreateLocalCategoryReturnsVisibleMatchWithoutRestore 验证同 slug 分类已存在时直接复用。
func TestFindOrCreateLocalCategoryReturnsVisibleMatchWithoutRestore(t *testing.T) {
	existing := &categorydomain.Category{ID: 5, Slug: "streaming", NameJSON: jsonmap.JSON{"zh-CN": "已有"}, IsActive: true}
	repo := &fakeCategoryRepo{
		visible: map[string]*categorydomain.Category{"streaming": existing},
		deleted: map[string]*categorydomain.Category{},
	}
	svc := &Service{categories: repo}

	got, err := svc.findOrCreateLocalCategory("streaming", jsonmap.JSON{"zh-CN": "新名"}, 0)
	if err != nil {
		t.Fatalf("findOrCreateLocalCategory failed: %v", err)
	}
	if got != existing {
		t.Fatalf("expected existing category reused, got %+v", got)
	}
	if repo.restored != 0 {
		t.Fatalf("expected no restore call for visible category, got %d", repo.restored)
	}
	if name, _ := got.NameJSON["zh-CN"].(string); name != "已有" {
		t.Fatalf("existing category name must not be overwritten, got %q", name)
	}
}

// TestFindOrCreateLocalCategoryRequiresCreatorForNewSlug 验证需要新建分类但未注入创建端口时报错。
func TestFindOrCreateLocalCategoryRequiresCreatorForNewSlug(t *testing.T) {
	repo := &fakeCategoryRepo{
		visible: map[string]*categorydomain.Category{},
		deleted: map[string]*categorydomain.Category{},
	}
	svc := &Service{categories: repo}

	if _, err := svc.findOrCreateLocalCategory("brand-new", jsonmap.JSON{"zh-CN": "新"}, 0); err == nil ||
		!strings.Contains(err.Error(), "category service not available") {
		t.Fatalf("expected category service not available error, got %v", err)
	}
}

func TestClassifyAutoCategory(t *testing.T) {
	tests := []struct {
		name     string
		values   []interface{}
		expected string
	}{
		{name: "ai product", values: []interface{}{jsonmap.JSON{"en-US": "OpenAI API credits"}}, expected: "ai-api"},
		{name: "streaming product", values: []interface{}{jsonmap.JSON{"zh-CN": "Netflix 会员"}}, expected: "streaming"},
		{name: "title wins over activation instructions", values: []interface{}{jsonmap.JSON{"en-US": "Apple TV 12 Months Link"}, jsonmap.JSON{"en-US": "Use Gemini during activation"}}, expected: "streaming"},
		{name: "vpn title wins over account wording", values: []interface{}{jsonmap.JSON{"en-US": "Nord VPN account"}, jsonmap.JSON{"en-US": "account delivery"}}, expected: "vpn-proxy"},
		{name: "unknown product", values: []interface{}{jsonmap.JSON{"en-US": "A niche digital product"}}, expected: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyAutoCategory(tt.values...).Slug; got != tt.expected {
				t.Fatalf("classifyAutoCategory() = %q, want %q", got, tt.expected)
			}
		})
	}
}
