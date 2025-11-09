package health

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/storage"
	log "github.com/sirupsen/logrus"
)

// Checker 健康检查器
type Checker struct {
	interval time.Duration
	stopCh   chan struct{}
}

// NewChecker 创建健康检查器
func NewChecker(interval time.Duration) *Checker {
	return &Checker{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动健康检查
func (c *Checker) Start() {
	log.Info("Starting health checker...")
	go c.checkLoop()
}

// Stop 停止健康检查
func (c *Checker) Stop() {
	close(c.stopCh)
}

// checkLoop 检查循环
func (c *Checker) checkLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// 立即执行一次
	c.checkAllNodes()

	for {
		select {
		case <-ticker.C:
			c.checkAllNodes()
		case <-c.stopCh:
			log.Info("Health checker stopped")
			return
		}
	}
}

// checkAllNodes 检查所有节点
func (c *Checker) checkAllNodes() {
	nodes, err := storage.GetAllNodes()
	if err != nil {
		log.Errorf("Failed to get nodes for health check: %v", err)
		return
	}

	log.Debugf("Health check: checking %d nodes", len(nodes))

	for _, node := range nodes {
		// 只检查运行中的节点
		if node.Status != "running" {
			continue
		}

		latency, available := c.checkNode(node)

		// 更新统计信息
		stats := &storage.NodeStats{
			NodeID:    node.ID,
			Latency:   latency,
			Available: available,
			LastCheck: time.Now(),
		}

		if err := storage.SaveNodeStats(stats); err != nil {
			log.Errorf("Failed to save node stats for node %d: %v", node.ID, err)
		}

		log.Debugf("Node %d (%s) - latency: %dms, available: %v", node.ID, node.Name, latency, available)
	}
}

// checkNode 检查单个节点
func (c *Checker) checkNode(node *storage.ProxyNode) (int, bool) {
	// 解析节点配置获取服务器地址和端口
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(node.Config), &config); err != nil {
		log.Errorf("Failed to parse node config: %v", err)
		return 0, false
	}

	server, ok := config["server"].(string)
	if !ok || server == "" {
		log.Warnf("Node %d has no server address", node.ID)
		return 0, false
	}

	port, ok := config["server_port"]
	if !ok {
		log.Warnf("Node %d has no server port", node.ID)
		return 0, false
	}

	// 转换端口为整数
	var serverPort int
	switch v := port.(type) {
	case float64:
		serverPort = int(v)
	case int:
		serverPort = v
	default:
		log.Warnf("Node %d has invalid port type", node.ID)
		return 0, false
	}

	// TCP连接测试
	address := fmt.Sprintf("%s:%d", server, serverPort)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		log.Debugf("Node %d (%s) connection failed: %v", node.ID, node.Name, err)
		return 0, false
	}
	defer conn.Close()

	latency := int(time.Since(start).Milliseconds())

	return latency, true
}
