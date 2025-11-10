package handlers

import (
	"fmt"
	"os"
	"strconv"

	"github.com/firadio/golang-singbox-manager/internal/network"
	"github.com/firadio/golang-singbox-manager/internal/singbox"
	"github.com/firadio/golang-singbox-manager/internal/storage"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
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
		Latency   int  `json:"latency"`
		Available bool `json:"available"`
	}

	nodesWithStats := make([]NodeWithStats, len(nodes))
	for i, node := range nodes {
		stats, ok := statsMap[node.ID]
		if ok {
			nodesWithStats[i] = NodeWithStats{
				ProxyNode: node,
				Latency:   stats.Latency,
				Available: stats.Available,
			}
		} else {
			nodesWithStats[i] = NodeWithStats{
				ProxyNode: node,
				Latency:   0,
				Available: false,
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
	// hijack_dns 默认已在数据库设置为 true

	// 创建节点
	if err := storage.CreateNode(&node); err != nil {
		log.Errorf("Failed to create node: %v", err)
		Error(c, 1006, "Failed to create node")
		return
	}

	// 自动分配 TunName 和 TunAddress (即使是 HTTP/SOCKS5，也保留以兼容数据库约束)
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
