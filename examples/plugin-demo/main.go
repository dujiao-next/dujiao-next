//go:build pluginexample
// +build pluginexample

package main

import (
	"context"
	"strings"

	"github.com/dujiao-next/pluginhost"
)

type demoModule struct {
	host pluginhost.Host
}

// PluginEntrypoint 插件入口符号。
var PluginEntrypoint pluginhost.Entrypoint = func(host pluginhost.Host) (pluginhost.Module, error) {
	return &demoModule{host: host}, nil
}

func (m *demoModule) OnLoad(ctx context.Context) error {
	_ = ctx
	_ = m.host.RegisterRoute(pluginhost.RouteRegistration{
		Scope:   pluginhost.ScopePublic,
		Path:    "/ping",
		Methods: []string{"GET"},
	})
	_ = m.host.RegisterRoute(pluginhost.RouteRegistration{
		Scope:   pluginhost.ScopeAdmin,
		Path:    "/status",
		Methods: []string{"GET"},
	})
	_ = m.host.RegisterPage(pluginhost.PageRegistration{
		Scope:     pluginhost.ScopeAdmin,
		RoutePath: "/demo-feature",
		Title:     "演示插件页面",
		SortOrder: 10,
	})
	_ = m.host.RegisterEventSubscription(pluginhost.EventSubscription{
		EventType: "order.paid",
		Meta: map[string]interface{}{
			"description": "演示订单支付事件订阅",
		},
	})
	m.host.Log("info", "demo_loaded", "演示插件已加载", map[string]interface{}{
		"plugin_id": m.host.GetPluginID(),
		"version":   m.host.GetVersion(),
	})
	return nil
}

func (m *demoModule) HandlePublic(ctx context.Context, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	_ = ctx
	path := strings.TrimSpace(req.Path)
	if path == "" || path == "/" || path == "/ping" {
		config, _ := m.host.ReadConfig()
		greeting, _ := config["greeting"].(string)
		if strings.TrimSpace(greeting) == "" {
			greeting = "Hello from demo plugin"
		}
		return &pluginhost.HTTPResponse{
			StatusCode: 200,
			Data: map[string]interface{}{
				"ok":        true,
				"plugin_id": m.host.GetPluginID(),
				"version":   m.host.GetVersion(),
				"message":   greeting,
			},
		}, nil
	}
	return &pluginhost.HTTPResponse{
		StatusCode: 404,
		Data: map[string]interface{}{
			"ok":      false,
			"message": "not found",
		},
	}, nil
}

func (m *demoModule) HandleAdmin(ctx context.Context, req *pluginhost.HTTPRequest) (*pluginhost.HTTPResponse, error) {
	_ = ctx
	config, _ := m.host.ReadConfig()
	mainDBDriver := ""
	hasMainDB := false
	if dbHost, ok := m.host.(pluginhost.MainDatabaseHost); ok {
		mainDBDriver = dbHost.GetMainDBDriver()
		hasMainDB = dbHost.GetMainDB() != nil
	}
	return &pluginhost.HTTPResponse{
		StatusCode: 200,
		Data: map[string]interface{}{
			"ok":             true,
			"plugin_id":      m.host.GetPluginID(),
			"version":        m.host.GetVersion(),
			"sqlite_path":    m.host.GetSQLitePath(),
			"main_db_driver": mainDBDriver,
			"has_main_db":    hasMainDB,
			"config":         config,
			"path":           req.Path,
		},
	}, nil
}
