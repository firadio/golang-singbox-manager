package api

import (
	"fmt"
	"path/filepath"

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
	router             *gin.Engine
	nodeHandler        *handlers.NodeHandler
	ruleHandler        *handlers.RuleHandler
	settingsHandler    *handlers.SettingsHandler
	logHandler         *handlers.LogHandler
	portMappingHandler *handlers.PortMappingHandler
	pageHandler        *handlers.PageHandler
	config             *config.Config
	configPath         string
	host               string
	port               int
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, configPath string, sbManager *singbox.Manager, rtManager *network.RoutingManager, mtClient *mikrotik.Client) *Server {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	// 加载模板（每次请求都重新加载，方便开发）
	router.LoadHTMLGlob("web/templates/**/*")

	// 禁用静态文件缓存，方便开发
	router.Use(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 7 && c.Request.URL.Path[:7] == "/static" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	})

	// 自定义日志中间件
	router.Use(func(c *gin.Context) {
		c.Next()
		log.Infof("%s %s %d", c.Request.Method, c.Request.URL.Path, c.Writer.Status())
	})

	server := &Server{
		router:             router,
		nodeHandler:        handlers.NewNodeHandler(sbManager, rtManager),
		ruleHandler:        handlers.NewRuleHandler(rtManager),
		settingsHandler:    handlers.NewSettingsHandler(cfg, configPath, mtClient),
		logHandler:         handlers.NewLogHandler(cfg.SingBox.LogDir),
		portMappingHandler: handlers.NewPortMappingHandler(mtClient),
		pageHandler:        handlers.NewPageHandler(),
		config:             cfg,
		configPath:         configPath,
		host:               cfg.Server.Host,
		port:               cfg.Server.Port,
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

		// 获取认证配置（用于登录页面）
		public.GET("/auth/config", s.settingsHandler.GetAuthConfig)

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
		nodes.POST("", s.nodeHandler.CreateNode)
		nodes.GET("/:id/config-file", s.nodeHandler.GetNodeConfigFile)
		nodes.GET("/:id/status", s.nodeHandler.GetNodeStatus)
		nodes.GET("/:id/logs", s.logHandler.GetNodeLogs)
		nodes.GET("/:id/logs/search", s.logHandler.SearchNodeLogs)
		nodes.POST("/:id/logs/clear", s.logHandler.ClearNodeLogs)
		nodes.POST("/:id/start", s.nodeHandler.StartNode)
		nodes.POST("/:id/stop", s.nodeHandler.StopNode)
		nodes.POST("/:id/restart", s.nodeHandler.RestartNode)
		nodes.POST("/:id/check", s.nodeHandler.CheckNodeLatency)
		nodes.GET("/:id/stream-test", s.nodeHandler.StreamLatencyTest) // 流式延迟测试
		nodes.GET("/:id", s.nodeHandler.GetNode)
		nodes.PUT("/:id", s.nodeHandler.UpdateNode)
		nodes.DELETE("/:id", s.nodeHandler.DeleteNode)
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

	// 端口映射 API
	portMappings := api.Group("/port-mappings")
	{
		portMappings.GET("", s.portMappingHandler.GetAllPortMappings)
		portMappings.GET("/:id", s.portMappingHandler.GetPortMapping)
		portMappings.POST("", s.portMappingHandler.CreatePortMapping)
		portMappings.PUT("/:id", s.portMappingHandler.UpdatePortMapping)
		portMappings.DELETE("/:id", s.portMappingHandler.DeletePortMapping)
		portMappings.POST("/:id/enable", s.portMappingHandler.EnablePortMapping)
		portMappings.POST("/:id/disable", s.portMappingHandler.DisablePortMapping)
		portMappings.POST("/sync", s.portMappingHandler.SyncPortMappings)
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

	// 静态文件服务 (CSS, JS, 图片等)
	s.router.Static("/static", "./web/static")

	// 页面路由 - 认证在前端 JavaScript 中处理
	s.router.GET("/nodes", s.pageHandler.RenderNodes)
	s.router.GET("/rules", s.pageHandler.RenderRules)
	s.router.GET("/port-mappings", s.pageHandler.RenderPortMappings)
	s.router.GET("/settings", s.pageHandler.RenderSettings)

	// 根路径重定向
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/nodes")
	})

	// 登录页面
	s.router.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login", gin.H{
			"Title": "登录",
		})
	})
}

// Start 启动服务器
func (s *Server) Start() error {
	// 启动 HTTP 服务器
	httpAddr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Infof("Starting API server on %s", httpAddr)

	// 如果启用HTTPS，同时启动HTTPS服务器
	if s.config.Server.HTTPSEnable {
		httpsAddr := fmt.Sprintf("%s:%d", s.host, s.config.Server.HTTPSPort)

		// 获取证书文件路径（使用与settings.go相同的计算方式）
		certFile := s.config.Server.CertFile
		keyFile := s.config.Server.KeyFile
		if !filepath.IsAbs(certFile) {
			certFile = filepath.Join(filepath.Dir(s.configPath), certFile)
		}
		if !filepath.IsAbs(keyFile) {
			keyFile = filepath.Join(filepath.Dir(s.configPath), keyFile)
		}

		log.Infof("Starting HTTPS server on %s", httpsAddr)
		log.Infof("Using certificate file: %s", certFile)
		log.Infof("Using key file: %s", keyFile)

		// 在goroutine中启动HTTPS服务器
		go func() {
			if err := s.router.RunTLS(httpsAddr, certFile, keyFile); err != nil {
				log.Errorf("HTTPS server error: %v", err)
			}
		}()
	}

	// 启动HTTP服务器（主线程）
	return s.router.Run(httpAddr)
}
