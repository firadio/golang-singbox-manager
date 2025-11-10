package health

import (
	"context"
	"os/exec"
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
	// 使用 DNS 查询测试节点可用性
	// 通过节点的路由表进行 DNS 查询，流量会经过代理节点
	start := time.Now()

	// 从 TUN 地址中提取 IP（去掉 /30）
	tunAddr := node.TunAddress
	if idx := len(tunAddr) - 3; idx > 0 && tunAddr[idx:] == "/30" {
		tunAddr = tunAddr[:idx]
	}

	// 使用 dig 命令通过路由表查询 DNS
	// -b 指定源地址，这样流量会通过对应的路由表和代理节点
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dig", "@1.1.1.1", "cloudflare.com", "+short", "+time=5", "-b", tunAddr)
	err := cmd.Run()

	if err != nil {
		log.Debugf("Node %d (%s) DNS query failed: %v", node.ID, node.Name, err)
		return 0, false
	}

	latency := int(time.Since(start).Milliseconds())

	return latency, true
}
