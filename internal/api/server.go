package api

import (
	"fmt"

	"github.com/firadio/golang-singbox-manager/internal/api/handlers"
	"github.com/firadio/golang-singbox-manager/internal/api/middleware"
	"github.com/firadio/golang-singbox-manager/internal/config"
	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	"github.com/firadio/golang-singbox-manager/internal/network"
	"github.com/firadio/golang-singbox-manager/internal/singbox"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Server API 服务器
type Server struct {
	router          *gin.Engine
	nodeHandler     *handlers.NodeHandler
	ruleHandler     *handlers.RuleHandler
	settingsHandler *handlers.SettingsHandler
	config          *config.Config
	host            string
	port            int
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, configPath string, sbManager *singbox.Manager, rtManager *network.RoutingManager, mtClient *mikrotik.Client) *Server {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	// 自定义日志中间件
	router.Use(func(c *gin.Context) {
		c.Next()
		log.Infof("%s %s %d", c.Request.Method, c.Request.URL.Path, c.Writer.Status())
	})

	server := &Server{
		router:          router,
		nodeHandler:     handlers.NewNodeHandler(sbManager, rtManager),
		ruleHandler:     handlers.NewRuleHandler(rtManager),
		settingsHandler: handlers.NewSettingsHandler(cfg, configPath, mtClient),
		config:          cfg,
		host:            cfg.Server.Host,
		port:            cfg.Server.Port,
	}

	server.setupRoutes()
	return server
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 公开路由（不需要认证）
	public := s.router.Group("/api")
	{
		// 登录
		public.POST("/auth/login", s.settingsHandler.Login)

		// 健康检查
		public.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}

	// 需要认证的API路由
	api := s.router.Group("/api")
	api.Use(middleware.AuthMiddleware(s.config))

	// 节点管理 API
	nodes := api.Group("/nodes")
	{
		nodes.GET("", s.nodeHandler.GetAllNodes)
		nodes.GET("/:id", s.nodeHandler.GetNode)
		nodes.POST("", s.nodeHandler.CreateNode)
		nodes.PUT("/:id", s.nodeHandler.UpdateNode)
		nodes.DELETE("/:id", s.nodeHandler.DeleteNode)
		nodes.POST("/:id/start", s.nodeHandler.StartNode)
		nodes.POST("/:id/stop", s.nodeHandler.StopNode)
		nodes.POST("/:id/restart", s.nodeHandler.RestartNode)
		nodes.GET("/:id/status", s.nodeHandler.GetNodeStatus)
	}

	// 路由规则 API
	rules := api.Group("/rules")
	{
		rules.GET("", s.ruleHandler.GetAllRules)
		rules.GET("/:id", s.ruleHandler.GetRule)
		rules.POST("", s.ruleHandler.CreateRule)
		rules.PUT("/:id", s.ruleHandler.UpdateRule)
		rules.DELETE("/:id", s.ruleHandler.DeleteRule)
		rules.POST("/:id/enable", s.ruleHandler.EnableRule)
		rules.POST("/:id/disable", s.ruleHandler.DisableRule)
	}

	// 系统设置 API
	settings := api.Group("/settings")
	{
		settings.GET("", s.settingsHandler.GetSettings)
		settings.PUT("", s.settingsHandler.UpdateSettings)
		settings.POST("/test-mikrotik", s.settingsHandler.TestMikrotikConnection)
	}

	// 系统控制 API
	system := api.Group("/system")
	{
		system.POST("/restart", s.settingsHandler.RestartService)
	}

	// 系统信息 API
	api.GET("/system/info", func(c *gin.Context) {
		handlers.Success(c, gin.H{
			"version": "0.0.1",
			"name":    "Golang Sing-Box Manager",
		})
	})

	// 静态文件服务 (Web UI) - 不需要认证
	s.router.Static("/static", "./web/static")
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/static/index.html")
	})
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Infof("Starting API server on %s", addr)
	return s.router.Run(addr)
}
