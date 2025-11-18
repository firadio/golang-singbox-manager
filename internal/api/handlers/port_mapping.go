package handlers

import (
	"strconv"

	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	"github.com/firadio/golang-singbox-manager/internal/storage"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// PortMappingHandler 端口映射处理器
type PortMappingHandler struct {
	mtClient *mikrotik.Client
}

// NewPortMappingHandler 创建端口映射处理器
func NewPortMappingHandler(mtClient *mikrotik.Client) *PortMappingHandler {
	return &PortMappingHandler{
		mtClient: mtClient,
	}
}

// GetAllPortMappings 获取所有端口映射
func (h *PortMappingHandler) GetAllPortMappings(c *gin.Context) {
	mappings, err := storage.GetAllPortMappings()
	if err != nil {
		log.Errorf("Failed to get port mappings: %v", err)
		Error(c, 3001, "Failed to get port mappings")
		return
	}

	Success(c, mappings)
}

// GetPortMapping 获取单个端口映射
func (h *PortMappingHandler) GetPortMapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 3002, "Invalid port mapping ID")
		return
	}

	mapping, err := storage.GetPortMapping(id)
	if err != nil {
		log.Errorf("Failed to get port mapping: %v", err)
		Error(c, 3003, "Port mapping not found")
		return
	}

	Success(c, mapping)
}

// CreatePortMapping 创建端口映射
func (h *PortMappingHandler) CreatePortMapping(c *gin.Context) {
	var mapping storage.PortMapping
	if err := c.ShouldBindJSON(&mapping); err != nil {
		Error(c, 3004, "Invalid request body")
		return
	}

	// 验证必填字段
	if mapping.Name == "" || mapping.Protocol == "" || mapping.DstPort == 0 ||
		mapping.ToAddress == "" || mapping.ToPort == 0 {
		Error(c, 3005, "Missing required fields")
		return
	}

	// 验证协议
	if mapping.Protocol != "tcp" && mapping.Protocol != "udp" {
		Error(c, 3006, "Invalid protocol (must be tcp or udp)")
		return
	}

	// 验证端口范围
	if mapping.DstPort < 1 || mapping.DstPort > 65535 ||
		mapping.ToPort < 1 || mapping.ToPort > 65535 {
		Error(c, 3007, "Invalid port number (must be 1-65535)")
		return
	}

	// 创建端口映射
	if err := storage.CreatePortMapping(&mapping); err != nil {
		log.Errorf("Failed to create port mapping: %v", err)
		Error(c, 3008, err.Error())
		return
	}

	// 如果启用，同步到 MikroTik（使用增量同步，只操作这一条映射的规则）
	// 优化说明：使用 SyncSinglePortMapping 而不是 SyncPortMappings
	// - 效率更高：只操作这一条映射，不影响其他映射
	// - 避免冲突：减少与其他操作的锁竞争时间
	if mapping.Enabled && h.mtClient != nil {
		if err := h.mtClient.SyncSinglePortMapping(&mapping); err != nil {
			log.Errorf("Failed to sync port mapping to MikroTik: %v", err)
			// 不返回错误，因为数据库已创建成功
		}
	}

	log.Infof("Port mapping created: %s (ID: %d, Port: %d)", mapping.Name, mapping.ID, mapping.DstPort)
	Success(c, mapping)
}

// UpdatePortMapping 更新端口映射
func (h *PortMappingHandler) UpdatePortMapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 3002, "Invalid port mapping ID")
		return
	}

	// 获取原有记录
	existing, err := storage.GetPortMapping(id)
	if err != nil {
		log.Errorf("Failed to get existing port mapping: %v", err)
		Error(c, 3003, "Port mapping not found")
		return
	}

	var updateData storage.PortMapping
	if err := c.ShouldBindJSON(&updateData); err != nil {
		Error(c, 3004, "Invalid request body")
		return
	}

	// 保留 ID 和时间戳
	existing.Name = updateData.Name
	existing.Protocol = updateData.Protocol
	existing.DstPort = updateData.DstPort
	existing.ToAddress = updateData.ToAddress
	existing.ToPort = updateData.ToPort
	existing.EnableMasquerade = updateData.EnableMasquerade
	existing.Enabled = updateData.Enabled

	// 验证字段
	if existing.Name == "" || existing.Protocol == "" || existing.DstPort == 0 ||
		existing.ToAddress == "" || existing.ToPort == 0 {
		Error(c, 3005, "Missing required fields")
		return
	}

	if existing.Protocol != "tcp" && existing.Protocol != "udp" {
		Error(c, 3006, "Invalid protocol (must be tcp or udp)")
		return
	}

	if existing.DstPort < 1 || existing.DstPort > 65535 ||
		existing.ToPort < 1 || existing.ToPort > 65535 {
		Error(c, 3007, "Invalid port number (must be 1-65535)")
		return
	}

	// 更新
	if err := storage.UpdatePortMapping(existing); err != nil {
		log.Errorf("Failed to update port mapping: %v", err)
		Error(c, 3009, err.Error())
		return
	}

	// 同步到 MikroTik（使用增量同步，只更新这一条映射）
	// 执行：删除旧规则 → 添加新规则（配置可能已修改）
	if h.mtClient != nil {
		if err := h.mtClient.SyncSinglePortMapping(existing); err != nil {
			log.Errorf("Failed to sync port mapping to MikroTik: %v", err)
		}
	}

	log.Infof("Port mapping updated: %s (ID: %d)", existing.Name, existing.ID)
	Success(c, existing)
}

// DeletePortMapping 删除端口映射
func (h *PortMappingHandler) DeletePortMapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 3002, "Invalid port mapping ID")
		return
	}

	// 先删除 MikroTik 上的规则（删除数据库记录之前）
	// 顺序重要：先删规则再删数据库，确保即使数据库删除失败，规则也不会成为孤立规则
	if h.mtClient != nil {
		if err := h.mtClient.DeletePortMappingRules(id); err != nil {
			log.Errorf("Failed to delete port mapping rules from MikroTik: %v", err)
		}
	}

	// 删除数据库记录
	if err := storage.DeletePortMapping(id); err != nil {
		log.Errorf("Failed to delete port mapping: %v", err)
		Error(c, 3010, "Failed to delete port mapping")
		return
	}

	log.Infof("Port mapping deleted: ID %d", id)
	Success(c, nil)
}

// EnablePortMapping 启用端口映射
func (h *PortMappingHandler) EnablePortMapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 3002, "Invalid port mapping ID")
		return
	}

	mapping, err := storage.GetPortMapping(id)
	if err != nil {
		log.Errorf("Failed to get port mapping: %v", err)
		Error(c, 3003, "Port mapping not found")
		return
	}

	mapping.Enabled = true
	if err := storage.UpdatePortMapping(mapping); err != nil {
		log.Errorf("Failed to enable port mapping: %v", err)
		Error(c, 3011, "Failed to enable port mapping")
		return
	}

	// 同步到 MikroTik（启用映射：添加 NAT 规则）
	// 执行：添加2条 dstnat 规则 + 可选的1条 masquerade 规则
	if h.mtClient != nil {
		if err := h.mtClient.SyncSinglePortMapping(mapping); err != nil {
			log.Errorf("Failed to sync port mapping to MikroTik: %v", err)
		}
	}

	log.Infof("Port mapping enabled: ID %d", id)
	Success(c, mapping)
}

// DisablePortMapping 禁用端口映射
func (h *PortMappingHandler) DisablePortMapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 3002, "Invalid port mapping ID")
		return
	}

	mapping, err := storage.GetPortMapping(id)
	if err != nil {
		log.Errorf("Failed to get port mapping: %v", err)
		Error(c, 3003, "Port mapping not found")
		return
	}

	// 先删除 MikroTik 上的规则（禁用映射：删除所有 NAT 规则）
	// 执行：删除该映射的所有规则（2条 dstnat + 可选的1条 masquerade）
	if h.mtClient != nil {
		if err := h.mtClient.DeletePortMappingRules(id); err != nil {
			log.Errorf("Failed to delete port mapping rules from MikroTik: %v", err)
		}
	}

	// 更新数据库状态为禁用
	mapping.Enabled = false
	if err := storage.UpdatePortMapping(mapping); err != nil {
		log.Errorf("Failed to disable port mapping: %v", err)
		Error(c, 3012, "Failed to disable port mapping")
		return
	}

	log.Infof("Port mapping disabled: ID %d", id)
	Success(c, mapping)
}

// SyncPortMappings 手动同步所有端口映射到 MikroTik
func (h *PortMappingHandler) SyncPortMappings(c *gin.Context) {
	if h.mtClient == nil {
		Error(c, 3013, "MikroTik client not configured")
		return
	}

	if err := h.mtClient.SyncPortMappings(); err != nil {
		log.Errorf("Failed to sync port mappings: %v", err)
		Error(c, 3014, "Failed to sync port mappings: "+err.Error())
		return
	}

	log.Info("Port mappings synced to MikroTik")
	Success(c, gin.H{"message": "Port mappings synced successfully"})
}
