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

	// 如果启用，同步到 MikroTik
	if mapping.Enabled && h.mtClient != nil {
		if err := h.mtClient.SyncPortMappings(); err != nil {
			log.Errorf("Failed to sync port mappings to MikroTik: %v", err)
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

	// 同步到 MikroTik
	if h.mtClient != nil {
		if err := h.mtClient.SyncPortMappings(); err != nil {
			log.Errorf("Failed to sync port mappings to MikroTik: %v", err)
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

	// 删除
	if err := storage.DeletePortMapping(id); err != nil {
		log.Errorf("Failed to delete port mapping: %v", err)
		Error(c, 3010, "Failed to delete port mapping")
		return
	}

	// 同步到 MikroTik（删除相关规则）
	if h.mtClient != nil {
		if err := h.mtClient.SyncPortMappings(); err != nil {
			log.Errorf("Failed to sync port mappings to MikroTik: %v", err)
		}
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

	// 同步到 MikroTik
	if h.mtClient != nil {
		if err := h.mtClient.SyncPortMappings(); err != nil {
			log.Errorf("Failed to sync port mappings to MikroTik: %v", err)
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

	mapping.Enabled = false
	if err := storage.UpdatePortMapping(mapping); err != nil {
		log.Errorf("Failed to disable port mapping: %v", err)
		Error(c, 3012, "Failed to disable port mapping")
		return
	}

	// 同步到 MikroTik（删除规则）
	if h.mtClient != nil {
		if err := h.mtClient.SyncPortMappings(); err != nil {
			log.Errorf("Failed to sync port mappings to MikroTik: %v", err)
		}
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
