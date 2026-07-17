package admin

import (
	"errors"
	"strconv"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// GetPostCategories 获取文章分类列表
func (h *Handler) GetPostCategories(c *gin.Context) {
	if c.Query("tree") == "1" {
		cats, err := h.PostCategoryService.ListTree()
		if err != nil {
			shared.RespondError(c, response.CodeInternal, "error.post_category_fetch_failed", err)
			return
		}
		response.Success(c, cats)
		return
	}

	var parentID *uint
	if pidStr := c.Query("parent_id"); pidStr != "" {
		pid, err := strconv.ParseUint(pidStr, 10, 64)
		if err == nil {
			n := uint(pid)
			parentID = &n
		}
	}
	cats, err := h.PostCategoryService.ListAll(parentID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.post_category_fetch_failed", err)
		return
	}
	response.Success(c, cats)
}

// CreatePostCategory 创建文章分类
func (h *Handler) CreatePostCategory(c *gin.Context) {
	var body struct {
		NameJSON  models.JSON `json:"name" binding:"required"`
		Slug      string      `json:"slug" binding:"required"`
		ParentID  *uint       `json:"parent_id"`
		SortOrder int         `json:"sort_order"`
		Icon      string      `json:"icon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	cat, err := h.PostCategoryService.Create(service.CreatePostCategoryInput{
		NameJSON:  body.NameJSON,
		Slug:      body.Slug,
		ParentID:  body.ParentID,
		SortOrder: body.SortOrder,
		Icon:      body.Icon,
	})
	if err != nil {
		if errors.Is(err, service.ErrSlugExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
			return
		}
		if errors.Is(err, service.ErrCategoryParentInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.category_parent_invalid", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_category_create_failed", err)
		return
	}
	response.Success(c, cat)
}

// UpdatePostCategory 修改文章分类
func (h *Handler) UpdatePostCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		shared.RespondBindError(c, err)
		return
	}

	var body struct {
		NameJSON  models.JSON `json:"name"`
		Slug      string      `json:"slug"`
		ParentID  *uint       `json:"parent_id"`
		SortOrder int         `json:"sort_order"`
		Icon      string      `json:"icon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	cat, err := h.PostCategoryService.Update(uint(id), service.CreatePostCategoryInput{
		NameJSON:  body.NameJSON,
		Slug:      body.Slug,
		ParentID:  body.ParentID,
		SortOrder: body.SortOrder,
		Icon:      body.Icon,
	})
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.post_category_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrSlugExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.slug_used", nil)
			return
		}
		if errors.Is(err, service.ErrCategoryParentInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.category_parent_invalid", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_category_update_failed", err)
		return
	}
	response.Success(c, cat)
}

// DeletePostCategory 删除文章分类
func (h *Handler) DeletePostCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		shared.RespondBindError(c, err)
		return
	}

	if err := h.PostCategoryService.Delete(uint(id)); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.post_category_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrCategoryInUse) {
			shared.RespondError(c, response.CodeBadRequest, "error.post_category_in_use", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_category_delete_failed", err)
		return
	}
	response.Success(c, nil)
}

// PatchPostCategoryStatus 启用/禁用文章分类
func (h *Handler) PatchPostCategoryStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		shared.RespondBindError(c, err)
		return
	}

	var body struct {
		IsActive *bool `json:"is_active" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	cat, err := h.PostCategoryService.SetActive(uint(id), *body.IsActive)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.post_category_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_category_update_failed", err)
		return
	}
	response.Success(c, cat)
}
