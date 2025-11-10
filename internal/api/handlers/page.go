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

// RenderNodes 渲染节点管理页面
func (h *PageHandler) RenderNodes(c *gin.Context) {
	c.HTML(200, "nodes-page", gin.H{
		"Title":       "节点管理",
		"CurrentPage": "nodes",
		"PageTitle":   "节点管理",
		"PageActions": []PageAction{
			{Text: "+ 创建节点", Class: "btn btn-primary", OnClick: template.JS("showCreateNodeModal()")},
		},
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

// RenderSettings 渲染系统设置页面
func (h *PageHandler) RenderSettings(c *gin.Context) {
	c.HTML(200, "settings-page", gin.H{
		"Title":       "系统设置",
		"CurrentPage": "settings",
		"PageTitle":   "系统设置",
		"PageActions": []PageAction{},
	})
}
