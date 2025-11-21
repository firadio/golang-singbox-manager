package handlers

import (
	"github.com/gin-gonic/gin"
	"html/template"
)

// PageAction 页面操作按钮
type PageAction struct {
	Text    string
	Class   string
	OnClick template.JS
}

// PageHandler 页面渲染处理器
type PageHandler struct{}

// NewPageHandler 创建页面处理器
func NewPageHandler() *PageHandler {
	return &PageHandler{}
}

// RenderPortMappings 渲染端口映射页面
func (h *PageHandler) RenderPortMappings(c *gin.Context) {
	c.HTML(200, "port-mappings-page", gin.H{
		"Title":       "端口映射",
		"CurrentPage": "port-mappings",
		"PageTitle":   "端口映射管理",
		"PageActions": []PageAction{
			{Text: "+ 创建映射", Class: "btn btn-primary", OnClick: template.JS("showCreatePortMappingModal()")},
			{Text: "同步到 MikroTik", Class: "btn btn-success", OnClick: template.JS("syncPortMappings()")},
		},
	})
}

// RenderNodes 渲染代理节点页面
func (h *PageHandler) RenderNodes(c *gin.Context) {
	c.HTML(200, "nodes-page", gin.H{
		"Title":       "代理节点",
		"CurrentPage": "nodes",
		"PageTitle":   "代理节点管理",
		// PageActions 已移除，按钮现在在 Tab 区域根据当前 Tab 动态显示
	})
}

// RenderRules 渲染路由规则页面
func (h *PageHandler) RenderRules(c *gin.Context) {
	c.HTML(200, "rules-page", gin.H{
		"Title":       "路由规则",
		"CurrentPage": "rules",
		"PageTitle":   "路由规则管理",
		"PageActions": []PageAction{
			{Text: "+ 创建规则", Class: "btn btn-primary", OnClick: template.JS("showCreateRuleModal()")},
		},
	})
}

// RenderDDNS 渲染 DDNS 页面
func (h *PageHandler) RenderDDNS(c *gin.Context) {
	c.HTML(200, "ddns-page", gin.H{
		"Title":       "动态域名",
		"CurrentPage": "ddns",
		"PageTitle":   "动态域名管理",
		"PageActions": []PageAction{
			{Text: "+ 创建 DDNS", Class: "btn btn-primary", OnClick: template.JS("showCreateDDNSModal()")},
		},
	})
}

// RenderSettings 渲染系统设置页面
func (h *PageHandler) RenderSettings(c *gin.Context) {
	c.HTML(200, "settings-page", gin.H{
		"Title":       "系统设置",
		"CurrentPage": "settings",
		"PageTitle":   "系统设置",
		"PageActions": []PageAction{},
	})
}
