package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
	upstreamhandlers "github.com/dujiao-next/internal/http/handlers/upstream"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

type callbackRouteSettingRepo struct {
	setting *models.Setting
}

func (r *callbackRouteSettingRepo) GetByKey(key string) (*models.Setting, error) {
	if r.setting != nil && r.setting.Key == key {
		return r.setting, nil
	}
	return nil, nil
}

func (r *callbackRouteSettingRepo) Upsert(key string, value models.JSON) (*models.Setting, error) {
	r.setting = &models.Setting{Key: key, ValueJSON: value}
	return r.setting, nil
}

func TestCallbackRouteMiddlewareGenericWebhookCustomPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settingService := service.NewSettingService(&callbackRouteSettingRepo{setting: &models.Setting{
		Key: constants.SettingKeyCallbackRoutesConfig,
		ValueJSON: models.JSON{
			constants.SettingFieldGenericWebhookCallback: "/api/custom/generic-hook",
		},
	}})
	settingService.InvalidateCallbackRoutesCache()
	t.Cleanup(settingService.InvalidateCallbackRoutesCache)

	upstreamHandler := upstreamhandlers.New(&provider.Container{}, nil)
	router := gin.New()
	router.Use(CallbackRouteMiddleware(settingService, nil, upstreamHandler))
	router.NoRoute(func(c *gin.Context) { c.Status(http.StatusTeapot) })

	custom := httptest.NewRecorder()
	router.ServeHTTP(custom, httptest.NewRequest(http.MethodPost, "/api/custom/generic-hook", nil))
	if custom.Code != http.StatusUnauthorized {
		t.Fatalf("custom generic webhook path was not dispatched: status=%d", custom.Code)
	}

	defaultPath := httptest.NewRecorder()
	router.ServeHTTP(defaultPath, httptest.NewRequest(http.MethodPost, constants.DefaultWebhookCallbackPath, nil))
	if defaultPath.Code != http.StatusNotFound {
		t.Fatalf("default generic webhook path should be hidden: status=%d", defaultPath.Code)
	}
}
