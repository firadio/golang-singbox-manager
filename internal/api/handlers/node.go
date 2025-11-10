package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/network"
	"github.com/firadio/golang-singbox-manager/internal/singbox"
	"github.com/firadio/golang-singbox-manager/internal/storage"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// NodeHandler 节点处理器
type NodeHandler struct {
	sbManager *singbox.Manager
	rtManager *network.RoutingManager
}

// NewNodeHandler 创建节点处理器
func NewNodeHandler(sbManager *singbox.Manager, rtManager *network.RoutingManager) *NodeHandler {
	return &NodeHandler{
		sbManager: sbManager,
		rtManager: rtManager,
	}
}

// GetAllNodes 获取所有节点
func (h *NodeHandler) GetAllNodes(c *gin.Context) {
	nodes, err := storage.GetAllNodes()
	if err != nil {
		log.Errorf("Failed to get nodes: %v", err)
		Error(c, 1001, "Failed to get nodes")
		return
	}

	// 获取所有节点的统计信息
	statsMap, err := storage.GetAllNodeStats()
	if err != nil {
		log.Warnf("Failed to get node stats: %v", err)
		// 即使获取统计失败，也返回节点信息
		Success(c, nodes)
		return
	}

	// 构建包含统计信息的节点数据
	type NodeWithStats struct {
		*storage.ProxyNode
		Latency       int  `json:"latency"`         // TUN延迟
		HTTPLatency   int  `json:"http_latency"`    // HTTP代理延迟
		Socks5Latency int  `json:"socks5_latency"`  // SOCKS5代理延迟
		Available     bool `json:"available"`
	}

	nodesWithStats := make([]NodeWithStats, len(nodes))
	for i, node := range nodes {
		stats, ok := statsMap[node.ID]
		if ok {
			nodesWithStats[i] = NodeWithStats{
				ProxyNode:     node,
				Latency:       stats.Latency,
				HTTPLatency:   stats.HTTPLatency,
				Socks5Latency: stats.Socks5Latency,
				Available:     stats.Available,
			}
		} else {
			nodesWithStats[i] = NodeWithStats{
				ProxyNode:     node,
				Latency:       0,
				HTTPLatency:   0,
				Socks5Latency: 0,
				Available:     false,
			}
		}
	}

	Success(c, nodesWithStats)
}

// GetNode 获取单个节点
func (h *NodeHandler) GetNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	node, err := storage.GetNode(id)
	if err != nil {
		log.Errorf("Failed to get node: %v", err)
		Error(c, 1003, "Node not found")
		return
	}

	Success(c, node)
}

// CreateNode 创建节点
func (h *NodeHandler) CreateNode(c *gin.Context) {
	var node storage.ProxyNode
	if err := c.ShouldBindJSON(&node); err != nil {
		Error(c, 1004, "Invalid request body")
		return
	}

	// 自动分配 TableID
	tableID, err := storage.GetNextAvailableTableID()
	if err != nil {
		log.Errorf("Failed to get next table ID: %v", err)
		Error(c, 1005, "Failed to allocate table ID")
		return
	}
	node.TableID = tableID

	// 设置默认值
	if node.InboundType == "" {
		node.InboundType = "tun"
	}
	if node.InboundListen == "" {
		node.InboundListen = "127.0.0.1"
	}

	// 创建节点
	if err := storage.CreateNode(&node); err != nil {
		log.Errorf("Failed to create node: %v", err)
		Error(c, 1006, "Failed to create node")
		return
	}

	// 自动分配 TunName 和 TunAddress
	node.TunName = storage.GetTunNameByID(node.ID)
	node.TunAddress = storage.GetTunAddressByID(node.ID)
	node.Status = "stopped"

	// 更新节点
	if err := storage.UpdateNode(&node); err != nil {
		log.Errorf("Failed to update node: %v", err)
	}

	log.Infof("Node created: %s (ID: %d)", node.Name, node.ID)
	Success(c, node)
}

// UpdateNode 更新节点
func (h *NodeHandler) UpdateNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	// 获取原有节点信息
	existingNode, err := storage.GetNode(id)
	if err != nil {
		log.Errorf("Failed to get existing node: %v", err)
		Error(c, 1003, "Node not found")
		return
	}

	var updateData storage.ProxyNode
	if err := c.ShouldBindJSON(&updateData); err != nil {
		Error(c, 1004, "Invalid request body")
		return
	}

	// 保留系统字段，只更新用户可编辑的字段
	existingNode.Name = updateData.Name
	existingNode.Type = updateData.Type
	existingNode.Config = updateData.Config
	existingNode.InboundType = updateData.InboundType
	existingNode.InboundListen = updateData.InboundListen
	existingNode.InboundPort = updateData.InboundPort
	existingNode.HijackDNS = updateData.HijackDNS
	// HTTP和SOCKS5端口（保留原有设置，不允许修改）
	// existingNode.HTTPPort 和 existingNode.Socks5Port 不修改
	if updateData.DetourID != nil {
		existingNode.DetourID = updateData.DetourID
	}
	if updateData.Enabled {
		existingNode.Enabled = updateData.Enabled
	}

	if err := storage.UpdateNode(existingNode); err != nil {
		log.Errorf("Failed to update node: %v", err)
		Error(c, 1007, "Failed to update node")
		return
	}

	log.Infof("Node updated: %s (ID: %d)", existingNode.Name, existingNode.ID)

	// 提示用户重启节点以应用新配置
	message := "节点已更新"
	if existingNode.Status == "running" {
		message = "节点已更新，请重启节点以应用新配置"
	}

	Success(c, gin.H{
		"node": existingNode,
		"message": message,
		"need_restart": existingNode.Status == "running",
	})
}

// DeleteNode 删除节点
func (h *NodeHandler) DeleteNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	// 先停止节点
	if err := h.sbManager.StopNode(id); err != nil {
		log.Warnf("Failed to stop node before delete: %v", err)
	}

	// 删除节点
	if err := storage.DeleteNode(id); err != nil {
		log.Errorf("Failed to delete node: %v", err)
		Error(c, 1008, "Failed to delete node")
		return
	}

	log.Infof("Node deleted: ID %d", id)
	Success(c, nil)
}

// StartNode 启动节点
func (h *NodeHandler) StartNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	node, err := storage.GetNode(id)
	if err != nil {
		log.Errorf("Failed to get node: %v", err)
		Error(c, 1003, "Node not found")
		return
	}

	// 启动 sing-box 进程
	if err := h.sbManager.StartNode(node); err != nil {
		log.Errorf("Failed to start node: %v", err)
		Error(c, 1009, "Failed to start node")
		return
	}

	// 重新获取节点信息
	node, _ = storage.GetNode(id)

	// 添加路由表默认路由
	if err := h.rtManager.AddTableRoute(node.TableID, node.TunName); err != nil {
		log.Errorf("Failed to add table route: %v", err)
		Error(c, 1010, "Failed to add route")
		return
	}

	log.Infof("Node started: %s (ID: %d)", node.Name, node.ID)
	Success(c, node)
}

// StopNode 停止节点
func (h *NodeHandler) StopNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	if err := h.sbManager.StopNode(id); err != nil {
		log.Errorf("Failed to stop node: %v", err)
		Error(c, 1011, "Failed to stop node")
		return
	}

	node, _ := storage.GetNode(id)
	log.Infof("Node stopped: %s (ID: %d)", node.Name, node.ID)
	Success(c, node)
}

// RestartNode 重启节点
func (h *NodeHandler) RestartNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	if err := h.sbManager.RestartNode(id); err != nil {
		log.Errorf("Failed to restart node: %v", err)
		Error(c, 1012, "Failed to restart node")
		return
	}

	// 重新添加路由
	node, _ := storage.GetNode(id)
	if err := h.rtManager.AddTableRoute(node.TableID, node.TunName); err != nil {
		log.Errorf("Failed to add table route: %v", err)
	}

	log.Infof("Node restarted: %s (ID: %d)", node.Name, node.ID)
	Success(c, node)
}

// GetNodeStatus 获取节点状态
func (h *NodeHandler) GetNodeStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	status, err := h.sbManager.GetNodeStatus(id)
	if err != nil {
		log.Errorf("Failed to get node status: %v", err)
		Error(c, 1013, "Failed to get node status")
		return
	}

	Success(c, gin.H{"status": status})
}

// GetNodeConfigFile 获取节点的真实配置文件内容
func (h *NodeHandler) GetNodeConfigFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	// 读取配置文件
	configPath := fmt.Sprintf("configs/singbox/node_%d.json", id)
	content, err := os.ReadFile(configPath)
	if err != nil {
		log.Errorf("Failed to read config file: %v", err)
		Error(c, 1014, "Failed to read config file")
		return
	}

	Success(c, gin.H{
		"path":    configPath,
		"content": string(content),
	})
}

// CheckNodeLatency 手动检测节点延迟
func (h *NodeHandler) CheckNodeLatency(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	node, err := storage.GetNode(id)
	if err != nil {
		log.Errorf("Failed to get node: %v", err)
		Error(c, 1003, "Node not found")
		return
	}

	// 只检测运行中的节点
	if node.Status != "running" {
		Error(c, 1015, "Node is not running")
		return
	}

	// 检测三种入站
	tunLatency, tunAvailable := checkNodeTUN(node)
	httpLatency, httpAvailable := checkNodeHTTP(node)
	socks5Latency, socks5Available := checkNodeSOCKS5(node)

	// 更新统计信息
	stats := &storage.NodeStats{
		NodeID:        node.ID,
		Latency:       tunLatency,
		HTTPLatency:   httpLatency,
		Socks5Latency: socks5Latency,
		Available:     tunAvailable,
		LastCheck:     time.Now(),
	}

	if err := storage.SaveNodeStats(stats); err != nil {
		log.Errorf("Failed to save node stats: %v", err)
	}

	Success(c, gin.H{
		"node_id":          node.ID,
		"tun_latency":      tunLatency,
		"tun_available":    tunAvailable,
		"http_latency":     httpLatency,
		"http_available":   httpAvailable,
		"socks5_latency":   socks5Latency,
		"socks5_available": socks5Available,
		"last_check":       stats.LastCheck,
	})
}

// StreamLatencyTest 流式延迟测试（支持指定协议和测试次数）
func (h *NodeHandler) StreamLatencyTest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 1002, "Invalid node ID")
		return
	}

	// 获取查询参数
	protocol := c.Query("protocol") // tun, http, socks5
	countStr := c.DefaultQuery("count", "5")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 20 {
		Error(c, 1002, "Invalid count parameter (1-20)")
		return
	}

	node, err := storage.GetNode(id)
	if err != nil {
		log.Errorf("Failed to get node: %v", err)
		Error(c, 1003, "Node not found")
		return
	}

	// 只检测运行中的节点
	if node.Status != "running" {
		Error(c, 1015, "Node is not running")
		return
	}

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 创建flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		Error(c, 1016, "Streaming not supported")
		return
	}

	// 根据协议选择测试函数
	var testFunc func(*storage.ProxyNode) (int, bool)
	switch protocol {
	case "tun":
		testFunc = checkNodeTUN
	case "http":
		testFunc = checkNodeHTTP
	case "socks5":
		testFunc = checkNodeSOCKS5
	default:
		Error(c, 1002, "Invalid protocol (tun/http/socks5)")
		return
	}

	// 执行测试并流式返回结果
	for i := 1; i <= count; i++ {
		latency, available := testFunc(node)

		// 发送SSE数据
		data := gin.H{
			"index":     i,
			"total":     count,
			"latency":   latency,
			"available": available,
			"protocol":  protocol,
		}

		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: %s\n\n", jsonData)
		flusher.Flush()

		// 如果不是最后一次测试，等待一小段时间
		if i < count {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 发送完成信号
	fmt.Fprintf(c.Writer, "data: {\"done\": true}\n\n")
	flusher.Flush()

	// 更新统计信息（使用最后一次测试结果）
	finalLatency, finalAvailable := testFunc(node)
	stats, _ := storage.GetNodeStats(node.ID)
	if stats == nil {
		stats = &storage.NodeStats{NodeID: node.ID}
	}

	switch protocol {
	case "tun":
		stats.Latency = finalLatency
		stats.Available = finalAvailable
	case "http":
		stats.HTTPLatency = finalLatency
	case "socks5":
		stats.Socks5Latency = finalLatency
	}
	stats.LastCheck = time.Now()

	storage.SaveNodeStats(stats)
}

// checkNodeTUN 通过TUN入站检查节点 (使用 curl --interface tun1)
func checkNodeTUN(node *storage.ProxyNode) (int, bool) {
	start := time.Now()

	// 使用 curl 命令通过 --interface 绑定到 TUN 设备（类似 curl --interface tun1 http://1.1.1.1）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "curl",
		"--interface", node.TunName, // 绑定到 TUN 设备
		"-m", "5", // 5秒超时
		"-s", // 静默模式
		"-o", "/dev/null", // 不输出内容
		"-w", "%{http_code}", // 只输出HTTP状态码
		"http://1.1.1.1") // 测试目标

	output, err := cmd.Output()
	if err != nil {
		log.Debugf("Node %d (%s) TUN test failed: %v", node.ID, node.Name, err)
		return 0, false
	}

	// 检查HTTP状态码（200表示成功）
	statusCode := string(output)
	if statusCode != "200" && statusCode != "301" && statusCode != "302" {
		log.Debugf("Node %d (%s) TUN test got HTTP %s", node.ID, node.Name, statusCode)
		return 0, false
	}

	latency := int(time.Since(start).Milliseconds())
	return latency, true
}

// checkNodeHTTP 通过HTTP代理检查节点
func checkNodeHTTP(node *storage.ProxyNode) (int, bool) {
	start := time.Now()

	// HTTP代理端口 = 8000 + ID
	httpPort := 8000 + node.ID
	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	if err != nil {
		log.Debugf("Node %d (%s) HTTP proxy URL parse failed: %v", node.ID, node.Name, err)
		return 0, false
	}

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 5 * time.Second,
	}

	// 请求一个简单的网站
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://www.gstatic.com/generate_204", nil)
	if err != nil {
		return 0, false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Debugf("Node %d (%s) HTTP proxy request failed: %v", node.ID, node.Name, err)
		return 0, false
	}
	defer resp.Body.Close()

	latency := int(time.Since(start).Milliseconds())
	return latency, resp.StatusCode == 204
}

// checkNodeSOCKS5 通过SOCKS5代理检查节点
func checkNodeSOCKS5(node *storage.ProxyNode) (int, bool) {
	start := time.Now()

	// SOCKS5代理端口 = 5000 + ID
	socks5Port := 5000 + node.ID
	proxyAddr := fmt.Sprintf("127.0.0.1:%d", socks5Port)

	// 创建SOCKS5拨号器
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		log.Debugf("Node %d (%s) SOCKS5 dialer creation failed: %v", node.ID, node.Name, err)
		return 0, false
	}

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
		Timeout: 5 * time.Second,
	}

	// 请求一个简单的网站
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://www.gstatic.com/generate_204", nil)
	if err != nil {
		return 0, false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Debugf("Node %d (%s) SOCKS5 proxy request failed: %v", node.ID, node.Name, err)
		return 0, false
	}
	defer resp.Body.Close()

	latency := int(time.Since(start).Milliseconds())
	return latency, resp.StatusCode == 204
}
