package mikrotik

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/firadio/golang-singbox-manager/internal/storage"
	log "github.com/sirupsen/logrus"
)

const (
	// commentPrefix 用于标识由本系统管理的NAT规则
	// 规则命名规则：
	// - dstnat 规则: "web-id-{ID}" (每个映射有2条：interface和address)
	// - srcnat masquerade 规则: "web-id-{ID}-masq" (启用masquerade时才有)
	commentPrefix = "web-id-"
)

// GetManagedNATRules 获取所有由本系统管理的 NAT 规则（包括 dstnat 和 srcnat masquerade）
// 返回值: map[端口映射ID][]规则ID，例如 map[1]["*123", "*124", "*125"] 表示映射1有3条规则
func (c *Client) GetManagedNATRules() (map[int][]string, error) {
	if !c.config.Enabled || c.conn == nil {
		return nil, fmt.Errorf("not connected to MikroTik")
	}

	// 查询所有 NAT 规则
	items, err := c.runCommand("/ip/firewall/nat/print", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query NAT rules: %w", err)
	}

	log.Infof("Retrieved %d NAT rules from MikroTik", len(items))

	// 按 comment 中的 ID 分组
	rulesMap := make(map[int][]string)

	for _, item := range items {
		// 过滤由本系统管理的规则（dstnat 或 srcnat masquerade）
		chain, hasChain := item["chain"]
		action, hasAction := item["action"]

		if !hasChain || !hasAction {
			continue
		}

		// 接受 dstnat/dst-nat 或 srcnat/masquerade 规则
		validRule := (chain == "dstnat" && action == "dst-nat") ||
			(chain == "srcnat" && action == "masquerade")

		if !validRule {
			continue
		}

		comment, hasComment := item["comment"]
		if hasComment && strings.HasPrefix(comment, commentPrefix) {
			// 提取 ID（去除后缀 -masq 如果存在）
			// 例如: "web-id-1" -> "1", "web-id-1-masq" -> "1"
			idStr := strings.TrimPrefix(comment, commentPrefix)
			// 移除 -masq 后缀，这样 dstnat 和 srcnat 规则可以归到同一个映射ID下
			idStr = strings.TrimSuffix(idStr, "-masq")

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

// AddNATRule 添加 NAT 规则（dstnat 端口转发）
// ruleType: "interface" (匹配入接口列表) 或 "address" (匹配目标地址列表)
// 每个端口映射需要添加2条 dstnat 规则，分别匹配不同的网络入口
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

	result, err := c.runCommand("/ip/firewall/nat/add", attrs)
	if err != nil {
		return fmt.Errorf("failed to add NAT rule (%s): %w", ruleType, err)
	}

	log.Infof("Added NAT rule for port mapping %d (%s): %s:%d -> %s:%d, result: %+v",
		mapping.ID, ruleType, mapping.Protocol, mapping.DstPort, mapping.ToAddress, mapping.ToPort, result)

	return nil
}

// AddMasqueradeRule 添加 masquerade 规则（SNAT 源地址转换）
// 用途：为没有设置网关的内网主机启用源地址转换，让返回流量能够正确路由回路由器
// 工作原理：当流量转发到目标主机时，将源地址改为路由器地址，这样目标主机的回应会发回路由器
func (c *Client) AddMasqueradeRule(mapping *storage.PortMapping) error {
	if !c.config.Enabled || c.conn == nil {
		return fmt.Errorf("not connected to MikroTik")
	}

	comment := fmt.Sprintf("%s%d-masq", commentPrefix, mapping.ID)

	attrs := map[string]string{
		"place-before": "0",
		"chain":        "srcnat",
		"action":       "masquerade",
		"protocol":     mapping.Protocol,
		"dst-address":  mapping.ToAddress,
		"dst-port":     strconv.Itoa(mapping.ToPort),
		"comment":      comment,
	}

	result, err := c.runCommand("/ip/firewall/nat/add", attrs)
	if err != nil {
		return fmt.Errorf("failed to add masquerade rule: %w", err)
	}

	log.Infof("Added masquerade rule for port mapping %d: %s:%s:%d, result: %+v",
		mapping.ID, mapping.Protocol, mapping.ToAddress, mapping.ToPort, result)

	return nil
}

// SyncPortMappings 全量同步所有端口映射到 MikroTik
// 使用场景：手动触发全量同步，或系统启动时初始化规则
// 执行逻辑：
// 1. 获取数据库中所有启用的端口映射
// 2. 获取 MikroTik 中所有由本系统管理的规则
// 3. 删除数据库中不存在的规则（清理孤立规则）
// 4. 为每个映射删除旧规则并重新创建（确保配置一致）
func (c *Client) SyncPortMappings() error {
	if !c.config.Enabled || c.conn == nil {
		return fmt.Errorf("not connected to MikroTik")
	}

	// 使用互斥锁防止并发同步冲突
	// 重要：如果不加锁，多个同步操作可能同时进行，导致规则被反复删除添加
	c.syncMux.Lock()
	defer c.syncMux.Unlock()

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
	log.Infof("Found %d managed rule groups in MikroTik", len(mtRulesMap))
	for id, ruleIDs := range mtRulesMap {
		log.Infof("Checking port mapping %d: %d rules in MikroTik", id, len(ruleIDs))
		if _, exists := dbMappingsMap[id]; !exists {
			// 数据库中不存在，删除所有相关规则
			log.Infof("Port mapping %d not found in database, deleting %d rules...", id, len(ruleIDs))
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
			log.Infof("Port mapping %d exists, recreating rules...", id)
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

		// 如果启用了masquerade，添加masquerade规则
		if mapping.EnableMasquerade {
			if err := c.AddMasqueradeRule(mapping); err != nil {
				log.Errorf("Failed to add masquerade rule for mapping %d: %v", id, err)
			}
		}
	}

	log.Info("Port mapping sync completed")
	return nil
}

// SyncSinglePortMapping 同步单个端口映射到 MikroTik（增量同步）
// 使用场景：创建、更新、启用端口映射时调用，只操作相关的规则，效率更高
// 执行逻辑：
// 1. 查找该映射在 MikroTik 中的现有规则
// 2. 删除所有旧规则（如果存在）
// 3. 如果映射是启用状态，添加新规则（2条 dstnat + 可选的1条 masquerade）
// 优势：相比全量同步，只操作单个映射的规则，避免影响其他映射
func (c *Client) SyncSinglePortMapping(mapping *storage.PortMapping) error {
	if !c.config.Enabled || c.conn == nil {
		return fmt.Errorf("not connected to MikroTik")
	}

	// 使用互斥锁防止与其他同步操作冲突
	c.syncMux.Lock()
	defer c.syncMux.Unlock()

	log.Infof("Syncing port mapping %d to MikroTik...", mapping.ID)

	// 1. 获取该映射在 MikroTik 中的现有规则
	mtRulesMap, err := c.GetManagedNATRules()
	if err != nil {
		return fmt.Errorf("failed to get NAT rules from MikroTik: %w", err)
	}

	// 2. 删除该映射的旧规则（如果存在）
	if existingRules, exists := mtRulesMap[mapping.ID]; exists {
		log.Infof("Deleting %d old rules for port mapping %d", len(existingRules), mapping.ID)
		for _, ruleID := range existingRules {
			if err := c.DeleteNATRule(ruleID); err != nil {
				log.Errorf("Failed to delete old rule %s: %v", ruleID, err)
			}
		}
	}

	// 3. 如果映射是启用的，添加新规则
	if mapping.Enabled {
		if err := c.AddNATRule(mapping, "interface"); err != nil {
			return fmt.Errorf("failed to add interface rule: %w", err)
		}

		if err := c.AddNATRule(mapping, "address"); err != nil {
			return fmt.Errorf("failed to add address rule: %w", err)
		}

		if mapping.EnableMasquerade {
			if err := c.AddMasqueradeRule(mapping); err != nil {
				return fmt.Errorf("failed to add masquerade rule: %w", err)
			}
		}
	}

	log.Infof("Port mapping %d synced successfully", mapping.ID)
	return nil
}

// DeletePortMappingRules 删除指定端口映射的所有规则
// 使用场景：禁用或删除端口映射时调用，清理相关的所有 NAT 规则
// 执行逻辑：
// 1. 查找该映射在 MikroTik 中的所有规则（包括 dstnat 和 masquerade）
// 2. 逐个删除所有找到的规则
// 3. 如果没有找到规则，仅记录日志不报错
func (c *Client) DeletePortMappingRules(mappingID int) error {
	if !c.config.Enabled || c.conn == nil {
		return fmt.Errorf("not connected to MikroTik")
	}

	// 使用互斥锁防止与其他同步操作冲突
	c.syncMux.Lock()
	defer c.syncMux.Unlock()

	log.Infof("Deleting rules for port mapping %d from MikroTik...", mappingID)

	// 获取该映射在 MikroTik 中的现有规则
	mtRulesMap, err := c.GetManagedNATRules()
	if err != nil {
		return fmt.Errorf("failed to get NAT rules from MikroTik: %w", err)
	}

	// 删除该映射的所有规则
	if existingRules, exists := mtRulesMap[mappingID]; exists {
		log.Infof("Found %d rules for port mapping %d, deleting...", len(existingRules), mappingID)
		for _, ruleID := range existingRules {
			if err := c.DeleteNATRule(ruleID); err != nil {
				log.Errorf("Failed to delete rule %s: %v", ruleID, err)
			}
		}
	} else {
		log.Infof("No rules found for port mapping %d", mappingID)
	}

	return nil
}
