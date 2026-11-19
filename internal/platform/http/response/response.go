package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	responseMsgSuccess = "success"
)

// Response 统一响应结构
type Response struct {
	StatusCode int         `json:"status_code"` // 业务状态码
	Msg        string      `json:"msg"`         // 提示消息
	Data       interface{} `json:"data"`        // 数据内容
}

// PageResponse 分页响应结构
type PageResponse struct {
	StatusCode int         `json:"status_code"`
	Msg        string      `json:"msg"`
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// ChannelResponse 渠道 API 响应结构。
type ChannelResponse struct {
	StatusCode int         `json:"status_code"`
	Msg        string      `json:"msg"`
	Data       interface{} `json:"data,omitempty"`
	ErrorCode  string      `json:"error_code,omitempty"`
	RequestID  string      `json:"request_id,omitempty"`
}

// Pagination 分页信息
type Pagination struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int64 `json:"total_page"`
}

// NormalizePage 归一化分页参数，避免非法页码与除零风险。
func NormalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

// BuildPagination 构造分页响应对象。
func BuildPagination(page, pageSize int, total int64) Pagination {
	page, pageSize = NormalizePage(page, pageSize)
	return Pagination{
		Page:      page,
		PageSize:  pageSize,
		Total:     total,
		TotalPage: (total + int64(pageSize) - 1) / int64(pageSize),
	}
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		StatusCode: 0,
		Msg:        responseMsgSuccess,
		Data:       data,
	})
}

// SuccessWithPage 分页成功响应
func SuccessWithPage(c *gin.Context, data interface{}, pagination Pagination) {
	c.JSON(http.StatusOK, PageResponse{
		StatusCode: 0,
		Msg:        responseMsgSuccess,
		Data:       data,
		Pagination: pagination,
	})
}

// Error 错误响应
func Error(c *gin.Context, statusCode int, msg string) {
	c.Set("response_status_code", statusCode)
	c.JSON(http.StatusOK, Response{
		StatusCode: statusCode,
		Msg:        msg,
		Data:       attachRequestID(c, nil),
	})
}

// ErrorWithHTTPStatus 返回真实 HTTP 状态码(非 200),body 仍使用统一 Response 结构。
// 仅基础设施层(recovery、auth 中间件等异常路径)应使用;业务层继续用 Error。
func ErrorWithHTTPStatus(c *gin.Context, httpStatus, statusCode int, msg string) {
	c.Set("response_status_code", statusCode)
	c.AbortWithStatusJSON(httpStatus, Response{
		StatusCode: statusCode,
		Msg:        msg,
		Data:       attachRequestID(c, nil),
	})
}

// Unauthorized 401响应
func Unauthorized(c *gin.Context, msg string) {
	Error(c, CodeUnauthorized, msg)
}

// Forbidden 403响应
func Forbidden(c *gin.Context, msg string) {
	Error(c, CodeForbidden, msg)
}

// ChannelSuccess 渠道 API 成功响应。
func ChannelSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, ChannelResponse{
		StatusCode: CodeOK,
		Msg:        responseMsgSuccess,
		Data:       data,
		RequestID:  currentRequestID(c),
	})
}

// ChannelError 渠道 API 错误响应。
func ChannelError(c *gin.Context, httpStatus, statusCode int, msg, errorCode string) {
	c.JSON(httpStatus, ChannelResponse{
		StatusCode: statusCode,
		Msg:        msg,
		ErrorCode:  errorCode,
		RequestID:  currentRequestID(c),
	})
}

func attachRequestID(c *gin.Context, data interface{}) interface{} {
	requestID := currentRequestID(c)
	if requestID == "" {
		return data
	}
	if data == nil {
		return gin.H{"request_id": requestID}
	}
	switch v := data.(type) {
	case gin.H:
		if _, ok := v["request_id"]; !ok {
			v["request_id"] = requestID
		}
		return v
	case map[string]interface{}:
		if _, ok := v["request_id"]; !ok {
			v["request_id"] = requestID
		}
		return v
	default:
		return gin.H{
			"request_id": requestID,
			"data":       data,
		}
	}
}

func currentRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get("request_id"); ok {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}
