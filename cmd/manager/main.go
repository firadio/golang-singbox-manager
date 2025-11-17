package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/api"
	"github.com/firadio/golang-singbox-manager/internal/config"
	"github.com/firadio/golang-singbox-manager/internal/ddns"
	"github.com/firadio/golang-singbox-manager/internal/health"
	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	"github.com/firadio/golang-singbox-manager/internal/network"
	"github.com/firadio/golang-singbox-manager/internal/singbox"
	"github.com/firadio/golang-singbox-manager/internal/storage"
	log "github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "Path to config file")
	version    = "0.0.1"
)

func main() {
	flag.Parse()

	// 打印版本信息
	fmt.Printf("Golang Sing-Box Manager v%s\n", version)
	fmt.Println("Starting...")

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	initLogger(cfg)

	log.Info("=== Golang Sing-Box Manager ===")
	log.Infof("Version: %s", version)
	log.Infof("Config: %s", *configPath)

	// 检查是否以 root 权限运行
	if os.Geteuid() != 0 {
		log.Fatal("This program must be run as root")
	}

	// 初始化数据库
	log.Info("Initializing database...")
	if err := storage.InitDB(cfg.Database.Path); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer storage.CloseDB()

	// 初始化 Mikrotik 客户端
	var mtClient *mikrotik.Client
	if cfg.Mikrotik.Enabled {
		log.Info("Initializing Mikrotik client...")
		mtClient = mikrotik.NewClient(&cfg.Mikrotik)
		if err := mtClient.Connect(); err != nil {
			log.Errorf("Failed to connect to Mikrotik: %v", err)
			log.Warn("Mikrotik integration disabled")
			mtClient = nil
		} else {
			defer mtClient.Close()
		}
	}

	// 初始化网络路由管理器
	log.Info("Initializing routing manager...")
	rtManager := network.NewRoutingManager()
	rtManager.SetMikrotikClient(mtClient)

	// 初始化系统路由（设置兜底规则，防止跳本地）
	log.Info("Initializing system routing...")
	if err := rtManager.InitSystemRouting(); err != nil {
		log.Fatalf("Failed to initialize system routing: %v", err)
	}

	// 初始化 sing-box 管理器
	log.Info("Initializing sing-box manager...")
	sbManager := singbox.NewManager(
		cfg.SingBox.BinPath,
		cfg.SingBox.ConfigDir,
		cfg.SingBox.LogDir,
	)

	// 恢复节点（Manager 重启后重新接管 sing-box 进程）
	log.Info("Recovering nodes...")
	if err := recoverNodes(sbManager, rtManager); err != nil {
		log.Errorf("Failed to recover nodes: %v", err)
	}

	// 恢复路由规则
	log.Info("Recovering routing rules...")
	if err := recoverRoutingRules(rtManager); err != nil {
		log.Errorf("Failed to recover routing rules: %v", err)
	}

	// 启动健康检查器
	log.Info("Starting health checker...")
	healthChecker := health.NewChecker(30 * time.Second)
	healthChecker.Start()
	defer healthChecker.Stop()

	// 启动 DDNS 自动更新器
	log.Info("Starting DDNS updater...")
	ddnsUpdater := ddns.NewUpdater(mtClient, 30*time.Second)
	ddnsUpdater.Start()
	defer ddnsUpdater.Stop()

	// 启动 API 服务器
	log.Info("Starting API server...")
	server := api.NewServer(cfg, *configPath, sbManager, rtManager, mtClient)

	// 在 goroutine 中启动服务器
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	log.Infof("API server started on %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info("Manager is ready!")

	// 等待退出信号
	waitForExit()

	log.Info("Manager is shutting down...")
	log.Info("Note: sing-box processes will continue running independently")
}

// initLogger 初始化日志
func initLogger(cfg *config.Config) {
	// 设置日志级别
	level, err := log.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	// 设置日志格式
	if cfg.Logging.Format == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	}

	log.SetOutput(os.Stdout)
}

// recoverNodes 恢复节点
func recoverNodes(sbManager *singbox.Manager, rtManager *network.RoutingManager) error {
	// 使用 sing-box Manager 的恢复功能
	if err := sbManager.RecoverNodes(); err != nil {
		return err
	}

	// 为每个运行中的节点添加路由表默认路由
	nodes, err := storage.GetEnabledNodes()
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if node.Status == "running" {
			// 添加路由表默认路由
			if err := rtManager.AddTableRoute(node.TableID, node.TunName); err != nil {
				log.Errorf("Failed to add route for node %s: %v", node.Name, err)
			}
		}
	}

	return nil
}

// recoverRoutingRules 恢复路由规则
func recoverRoutingRules(rtManager *network.RoutingManager) error {
	rules, err := storage.GetEnabledRules()
	if err != nil {
		return err
	}

	for _, rule := range rules {
		// 获取节点信息
		node, err := storage.GetNode(rule.NodeID)
		if err != nil {
			log.Errorf("Failed to get node for rule %s: %v", rule.Name, err)
			continue
		}

		// 只为运行中的节点添加路由规则
		if node.Status != "running" {
			log.Warnf("Skipping rule %s: node %s is not running", rule.Name, node.Name)
			continue
		}

		// 添加路由规则
		if err := rtManager.AddRule(rule.SourceIP, node.TableID, rule.Priority, rule.ID); err != nil {
			log.Errorf("Failed to add rule %s: %v", rule.Name, err)
		} else {
			log.Infof("Recovered rule: %s (%s -> table %d)", rule.Name, rule.SourceIP, node.TableID)
		}
	}

	return nil
}

// waitForExit 等待退出信号
func waitForExit() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
