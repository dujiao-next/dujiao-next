package application

import (
	"context"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestNormalizePostLayout(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":        "article",
		"article": "article",
		"blocks":  "blocks",
		"unknown": "article",
	}
	for input, want := range cases {
		if got := normalizePostLayout(input); got != want {
			t.Fatalf("normalizePostLayout(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCreatePageStoresBlocks(t *testing.T) {
	t.Parallel()

	store := &postStoreStub{}
	service := NewPostService(store, store, nil, fixedClock{})

	blocks := map[string]interface{}{
		"version": float64(1),
		"blocks": []interface{}{
			map[string]interface{}{"id": "b1", "type": "hero", "props": map[string]interface{}{}},
		},
	}

	post, err := service.Create(context.Background(), CreatePostInput{
		Slug:          "home",
		Type:          constants.PostTypePage,
		Layout:        "blocks",
		ContentBlocks: blocks,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if post.Layout != "blocks" {
		t.Fatalf("Layout = %q, want blocks", post.Layout)
	}
	if len(post.ContentBlocks) == 0 {
		t.Fatalf("ContentBlocks is empty")
	}
}

func TestCreatePostWithoutLayoutDefaultsToArticle(t *testing.T) {
	t.Parallel()

	store := &postStoreStub{}
	service := NewPostService(store, store, nil, fixedClock{})

	post, err := service.Create(context.Background(), CreatePostInput{
		Slug: "plain-post",
		Type: constants.PostTypeBlog,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if post.Layout != "article" {
		t.Fatalf("Layout = %q, want article", post.Layout)
	}
}

func TestListPublicExcludesPagesByDefault(t *testing.T) {
	t.Parallel()

	store := &postStoreStub{}
	service := NewPostService(store, store, nil, fixedClock{})

	if _, _, err := service.ListPublic(context.Background(), PublicPostQuery{}); err != nil {
		t.Fatalf("ListPublic() error = %v", err)
	}
	if len(store.lastQuery.ExcludeTypes) != 1 || store.lastQuery.ExcludeTypes[0] != constants.PostTypePage {
		t.Fatalf("ExcludeTypes = %v, want [page]", store.lastQuery.ExcludeTypes)
	}
}

func TestListPublicKeepsExplicitTypeWithoutExclusion(t *testing.T) {
	t.Parallel()

	store := &postStoreStub{}
	service := NewPostService(store, store, nil, fixedClock{})

	if _, _, err := service.ListPublic(context.Background(), PublicPostQuery{Type: constants.PostTypeBlog}); err != nil {
		t.Fatalf("ListPublic() error = %v", err)
	}
	if len(store.lastQuery.ExcludeTypes) != 0 {
		t.Fatalf("ExcludeTypes = %v, want empty", store.lastQuery.ExcludeTypes)
	}
}
