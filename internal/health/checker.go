package health

import (
	"context"
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
	// 使用 DNS 查询测试节点可用性
	// 通过节点的 TUN 接口查询 1.1.1.1 的 DNS 服务
	start := time.Now()

	// 创建 DNS resolver，使用 1.1.1.1:53 作为 DNS 服务器
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// 使用 TUN 接口的地址作为源地址
			// 从 TUN 地址中提取 IP（去掉 /30）
			tunAddr := node.TunAddress
			if idx := len(tunAddr) - 3; idx > 0 && tunAddr[idx:] == "/30" {
				tunAddr = tunAddr[:idx]
			}

			localAddr, err := net.ResolveTCPAddr(network, tunAddr+":0")
			if err != nil {
				return nil, fmt.Errorf("failed to resolve local address: %w", err)
			}

			dialer := &net.Dialer{
				LocalAddr: localAddr,
				Timeout:   5 * time.Second,
			}

			// 连接到 1.1.1.1:53
			return dialer.DialContext(ctx, network, "1.1.1.1:53")
		},
	}

	// 查询 cloudflare.com 的 A 记录
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := resolver.LookupHost(ctx, "cloudflare.com")
	if err != nil {
		log.Debugf("Node %d (%s) DNS query failed: %v", node.ID, node.Name, err)
		return 0, false
	}

	latency := int(time.Since(start).Milliseconds())

	return latency, true
}
