package handlers

import (
	"strconv"

	"github.com/firadio/golang-singbox-manager/internal/network"
	"github.com/firadio/golang-singbox-manager/internal/storage"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// RuleHandler 规则处理器
type RuleHandler struct {
	rtManager *network.RoutingManager
}

// NewRuleHandler 创建规则处理器
func NewRuleHandler(rtManager *network.RoutingManager) *RuleHandler {
	return &RuleHandler{
		rtManager: rtManager,
	}
}

// GetAllRules 获取所有规则
func (h *RuleHandler) GetAllRules(c *gin.Context) {
	rules, err := storage.GetAllRules()
	if err != nil {
		log.Errorf("Failed to get rules: %v", err)
		Error(c, 2001, "Failed to get rules")
		return
	}

	Success(c, rules)
}

// GetRule 获取单个规则
func (h *RuleHandler) GetRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2002, "Invalid rule ID")
		return
	}

	rule, err := storage.GetRule(id)
	if err != nil {
		log.Errorf("Failed to get rule: %v", err)
		Error(c, 2003, "Rule not found")
		return
	}

	Success(c, rule)
}

// CreateRule 创建规则
func (h *RuleHandler) CreateRule(c *gin.Context) {
	var rule storage.RoutingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		Error(c, 2004, "Invalid request body")
		return
	}

	// 自动分配优先级
	if rule.Priority == 0 {
		priority, err := storage.GetNextAvailablePriority()
		if err != nil {
			log.Errorf("Failed to get next priority: %v", err)
			Error(c, 2005, "Failed to allocate priority")
			return
		}
		rule.Priority = priority
	}

	// 创建规则
	if err := storage.CreateRule(&rule); err != nil {
		log.Errorf("Failed to create rule: %v", err)
		Error(c, 2006, "Failed to create rule")
		return
	}

	// 如果启用，立即应用路由规则
	if rule.Enabled {
		node, err := storage.GetNode(rule.NodeID)
		if err != nil {
			log.Errorf("Failed to get node for rule: %v", err)
		} else {
			if err := h.rtManager.AddRule(rule.SourceIP, node.TableID, rule.Priority, rule.ID); err != nil {
				log.Errorf("Failed to add routing rule: %v", err)
			}
		}
	}

	log.Infof("Rule created: %s (ID: %d)", rule.Name, rule.ID)
	Success(c, rule)
}

// UpdateRule 更新规则
func (h *RuleHandler) UpdateRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2002, "Invalid rule ID")
		return
	}

	// 获取旧规则
	oldRule, err := storage.GetRule(id)
	if err != nil {
		log.Errorf("Failed to get rule: %v", err)
		Error(c, 2003, "Rule not found")
		return
	}

	var updateData storage.RoutingRule
	if err := c.ShouldBindJSON(&updateData); err != nil {
		Error(c, 2004, "Invalid request body")
		return
	}

	// 保存旧的 SourceIP 和 NodeID，用于删除旧的路由规则
	oldSourceIP := oldRule.SourceIP
	oldNodeID := oldRule.NodeID

	// 如果旧规则启用，先删除旧的路由规则（使用旧值）
	if oldRule.Enabled {
		oldNode, err := storage.GetNode(oldNodeID)
		if err == nil {
			h.rtManager.DeleteRule(oldSourceIP, oldNode.TableID, id)
		}
	}

	// 保留系统字段，只更新用户可编辑的字段
	oldRule.Name = updateData.Name
	oldRule.SourceIP = updateData.SourceIP
	oldRule.SourceCIDR = updateData.SourceCIDR
	oldRule.NodeID = updateData.NodeID
	oldRule.Enabled = updateData.Enabled
	// 如果前端传了 priority，则更新；否则保持不变
	if updateData.Priority > 0 {
		oldRule.Priority = updateData.Priority
	}

	// 更新规则
	if err := storage.UpdateRule(oldRule); err != nil {
		log.Errorf("Failed to update rule: %v", err)
		Error(c, 2007, "Failed to update rule")
		return
	}

	// 如果新规则启用，应用新的路由规则
	if oldRule.Enabled {
		node, err := storage.GetNode(oldRule.NodeID)
		if err != nil {
			log.Errorf("Failed to get node for rule: %v", err)
		} else {
			if err := h.rtManager.AddRule(oldRule.SourceIP, node.TableID, oldRule.Priority, oldRule.ID); err != nil {
				log.Errorf("Failed to add routing rule: %v", err)
			}
		}
	}

	log.Infof("Rule updated: %s (ID: %d)", oldRule.Name, oldRule.ID)
	Success(c, oldRule)
}

// DeleteRule 删除规则
func (h *RuleHandler) DeleteRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2002, "Invalid rule ID")
		return
	}

	// 获取规则
	rule, err := storage.GetRule(id)
	if err != nil {
		log.Errorf("Failed to get rule: %v", err)
		Error(c, 2003, "Rule not found")
		return
	}

	// 如果启用，先删除路由规则
	if rule.Enabled {
		node, err := storage.GetNode(rule.NodeID)
		if err == nil {
			h.rtManager.DeleteRule(rule.SourceIP, node.TableID, rule.ID)
		}
	}

	// 删除规则
	if err := storage.DeleteRule(id); err != nil {
		log.Errorf("Failed to delete rule: %v", err)
		Error(c, 2008, "Failed to delete rule")
		return
	}

	log.Infof("Rule deleted: ID %d", id)
	Success(c, nil)
}

// EnableRule 启用规则
func (h *RuleHandler) EnableRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2002, "Invalid rule ID")
		return
	}

	rule, err := storage.GetRule(id)
	if err != nil {
		log.Errorf("Failed to get rule: %v", err)
		Error(c, 2003, "Rule not found")
		return
	}

	rule.Enabled = true
	if err := storage.UpdateRule(rule); err != nil {
		log.Errorf("Failed to enable rule: %v", err)
		Error(c, 2009, "Failed to enable rule")
		return
	}

	// 应用路由规则
	node, err := storage.GetNode(rule.NodeID)
	if err != nil {
		log.Errorf("Failed to get node for rule: %v", err)
		Error(c, 2010, "Failed to get node")
		return
	}

	if err := h.rtManager.AddRule(rule.SourceIP, node.TableID, rule.Priority, rule.ID); err != nil {
		log.Errorf("Failed to add routing rule: %v", err)
		Error(c, 2011, "Failed to add routing rule")
		return
	}

	log.Infof("Rule enabled: %s (ID: %d)", rule.Name, rule.ID)
	Success(c, rule)
}

// DisableRule 禁用规则
func (h *RuleHandler) DisableRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2002, "Invalid rule ID")
		return
	}

	rule, err := storage.GetRule(id)
	if err != nil {
		log.Errorf("Failed to get rule: %v", err)
		Error(c, 2003, "Rule not found")
		return
	}

	// 删除路由规则
	node, err := storage.GetNode(rule.NodeID)
	if err == nil {
		if err := h.rtManager.DeleteRule(rule.SourceIP, node.TableID, rule.ID); err != nil {
			log.Errorf("Failed to delete routing rule: %v", err)
		}
	}

	rule.Enabled = false
	if err := storage.UpdateRule(rule); err != nil {
		log.Errorf("Failed to disable rule: %v", err)
		Error(c, 2012, "Failed to disable rule")
		return
	}

	log.Infof("Rule disabled: %s (ID: %d)", rule.Name, rule.ID)
	Success(c, rule)
}
