package mikrotik

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/firadio/golang-singbox-manager/internal/storage"
	log "github.com/sirupsen/logrus"
)

const (
	commentPrefix = "web-id-"
)

// GetManagedNATRules 获取所有由本系统管理的 NAT 规则
func (c *Client) GetManagedNATRules() (map[int][]string, error) {
	if !c.config.Enabled || c.conn == nil {
		return nil, fmt.Errorf("not connected to MikroTik")
	}

	// 查询所有 dstnat 规则
	items, err := c.runCommand("/ip/firewall/nat/print", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query NAT rules: %w", err)
	}

	// 按 comment 中的 ID 分组
	rulesMap := make(map[int][]string)

	for _, item := range items {
		// 过滤 chain 和 action
		chain, hasChain := item["chain"]
		action, hasAction := item["action"]
		if !hasChain || chain != "dstnat" || !hasAction || action != "dst-nat" {
			continue
		}

		comment, hasComment := item["comment"]
		if hasComment && strings.HasPrefix(comment, commentPrefix) {
			// 提取 ID
			idStr := strings.TrimPrefix(comment, commentPrefix)
			id, err := strconv.Atoi(idStr)
			if err != nil {
				log.Warnf("Invalid comment format: %s", comment)
				continue
			}

			// 获取规则 .id
			ruleID, hasID := item[".id"]
			if hasID {
				rulesMap[id] = append(rulesMap[id], ruleID)
			}
		}
	}

	return rulesMap, nil
}

// DeleteNATRule 删除指定的 NAT 规则
func (c *Client) DeleteNATRule(ruleID string) error {
	if !c.config.Enabled || c.conn == nil {
		return fmt.Errorf("not connected to MikroTik")
	}

	attrs := map[string]string{
		".id": ruleID,
	}

	_, err := c.runCommand("/ip/firewall/nat/remove", attrs)
	if err != nil {
		return fmt.Errorf("failed to delete NAT rule %s: %w", ruleID, err)
	}

	log.Debugf("Deleted NAT rule: %s", ruleID)
	return nil
}

// AddNATRule 添加 NAT 规则
func (c *Client) AddNATRule(mapping *storage.PortMapping, ruleType string) error {
	if !c.config.Enabled || c.conn == nil {
		return fmt.Errorf("not connected to MikroTik")
	}

	comment := fmt.Sprintf("%s%d", commentPrefix, mapping.ID)

	attrs := map[string]string{
		"place-before":  "0",
		"chain":         "dstnat",
		"action":        "dst-nat",
		"protocol":      mapping.Protocol,
		"dst-port":      strconv.Itoa(mapping.DstPort),
		"to-addresses":  mapping.ToAddress,
		"to-ports":      strconv.Itoa(mapping.ToPort),
		"comment":       comment,
	}

	// 根据类型添加不同的匹配条件
	if ruleType == "interface" {
		attrs["in-interface-list"] = "list-wan"
	} else if ruleType == "address" {
		attrs["dst-address-list"] = "WAN"
	}

	_, err := c.runCommand("/ip/firewall/nat/add", attrs)
	if err != nil {
		return fmt.Errorf("failed to add NAT rule (%s): %w", ruleType, err)
	}

	log.Infof("Added NAT rule for port mapping %d (%s): %s:%d -> %s:%d",
		mapping.ID, ruleType, mapping.Protocol, mapping.DstPort, mapping.ToAddress, mapping.ToPort)

	return nil
}

// SyncPortMappings 同步所有端口映射到 MikroTik
func (c *Client) SyncPortMappings() error {
	if !c.config.Enabled || c.conn == nil {
		return fmt.Errorf("not connected to MikroTik")
	}

	log.Info("Starting port mapping sync to MikroTik...")

	// 1. 获取数据库中的所有启用的端口映射
	dbMappings, err := storage.GetEnabledPortMappings()
	if err != nil {
		return fmt.Errorf("failed to get port mappings from database: %w", err)
	}

	// 转换为 map 便于查找
	dbMappingsMap := make(map[int]*storage.PortMapping)
	for _, m := range dbMappings {
		dbMappingsMap[m.ID] = m
	}

	// 2. 获取 MikroTik 中的所有由本系统管理的 NAT 规则
	mtRulesMap, err := c.GetManagedNATRules()
	if err != nil {
		return fmt.Errorf("failed to get NAT rules from MikroTik: %w", err)
	}

	// 3. 删除数据库中不存在或已禁用的规则
	for id, ruleIDs := range mtRulesMap {
		if _, exists := dbMappingsMap[id]; !exists {
			// 数据库中不存在，删除所有相关规则
			log.Infof("Port mapping %d not found in database, deleting rules...", id)
			for _, ruleID := range ruleIDs {
				if err := c.DeleteNATRule(ruleID); err != nil {
					log.Errorf("Failed to delete rule %s: %v", ruleID, err)
				}
			}
		}
	}

	// 4. 添加或更新规则
	for id, mapping := range dbMappingsMap {
		existingRules := mtRulesMap[id]

		// 如果已存在规则，先删除再重新创建（简单的更新策略）
		if len(existingRules) > 0 {
			log.Debugf("Port mapping %d exists, recreating rules...", id)
			for _, ruleID := range existingRules {
				if err := c.DeleteNATRule(ruleID); err != nil {
					log.Errorf("Failed to delete old rule %s: %v", ruleID, err)
				}
			}
		}

		// 添加新规则（两条：interface 和 address）
		if err := c.AddNATRule(mapping, "interface"); err != nil {
			log.Errorf("Failed to add interface rule for mapping %d: %v", id, err)
		}

		if err := c.AddNATRule(mapping, "address"); err != nil {
			log.Errorf("Failed to add address rule for mapping %d: %v", id, err)
		}
	}

	log.Info("Port mapping sync completed")
	return nil
}
