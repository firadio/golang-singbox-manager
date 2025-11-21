package singbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firadio/golang-singbox-manager/internal/storage"
)

// SingBoxConfig sing-box 配置结构
type SingBoxConfig struct {
	Log       LogConfig       `json:"log"`
	DNS       DNSConfig       `json:"dns"`
	Route     RouteConfig     `json:"route"`
	Inbounds  []Inbound       `json:"inbounds"`
	Outbounds []interface{}   `json:"outbounds"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `json:"level"`
}

// DNSConfig DNS配置
type DNSConfig struct {
	Servers []DNSServer `json:"servers"`
}

// DNSServer DNS服务器
type DNSServer struct {
	Tag    string `json:"tag"`
	Type   string `json:"type"`
	Server string `json:"server"`
	Detour string `json:"detour"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	Rules []RouteRule `json:"rules"`
	Final string      `json:"final"`
}

// RouteRule 路由规则
type RouteRule struct {
	Action   string `json:"action"`
	Protocol string `json:"protocol,omitempty"`
}

// Inbound 入站配置 (支持 TUN/HTTP/SOCKS5)
type Inbound struct {
	Type          string   `json:"type"` // tun/http/socks5
	InterfaceName string   `json:"interface_name,omitempty"` // for tun
	Address       []string `json:"address,omitempty"` // for tun
	Listen        string   `json:"listen,omitempty"` // for http/socks5
	ListenPort    int      `json:"listen_port,omitempty"` // for http/socks5
	Tag           string   `json:"tag,omitempty"` // for identification
}

// GenerateConfig 生成 sing-box 配置文件
func GenerateConfig(node *storage.ProxyNode, detourNode *storage.ProxyNode) (string, error) {
	// 解析节点的 outbound 配置
	var nodeOutbound map[string]interface{}
	if err := json.Unmarshal([]byte(node.Config), &nodeOutbound); err != nil {
		return "", fmt.Errorf("failed to parse node config: %w", err)
	}

	// 设置节点的 tag 为 "node-{id}"
	nodeTag := fmt.Sprintf("node-%d", node.ID)
	nodeOutbound["tag"] = nodeTag

	// 构建 outbounds 列表
	outbounds := []interface{}{nodeOutbound}
	visited := make(map[int]bool) // 防止循环引用
	visited[node.ID] = true

	// 优先从 config JSON 中获取 detour 引用（支持策略组引用）
	// 而不是使用 detour_id 字段（detour_id 可能指向旧的普通节点）
	var currentDetour *storage.ProxyNode
	if detourTag, ok := nodeOutbound["detour"].(string); ok {
		// 从 "node-X" 格式中提取 ID
		var detourID int
		if _, err := fmt.Sscanf(detourTag, "node-%d", &detourID); err == nil {
			if fetchedNode, err := storage.GetNode(detourID); err == nil {
				currentDetour = fetchedNode
			}
		}
	} else if detourNode != nil {
		// 降级：如果 config 中没有 detour，使用传入的 detourNode 参数
		currentDetour = detourNode
	}

	// 递归获取所有中转节点链
	policyVisited := make(map[int]bool) // 策略组循环引用检测
	globalAdded := make(map[int]bool)   // 全局已添加记录
	globalAdded[node.ID] = true

	for currentDetour != nil {
		// 防止循环引用
		if visited[currentDetour.ID] {
			return "", fmt.Errorf("circular detour reference detected for node %d", currentDetour.ID)
		}
		visited[currentDetour.ID] = true

		// 判断中转节点是普通节点还是策略组
		if currentDetour.IsPolicyGroup() {
			// 中转到策略组：递归展开策略组的所有成员
			_, err := processNodeRecursive(currentDetour, policyVisited, globalAdded, &outbounds)
			if err != nil {
				return "", fmt.Errorf("failed to process detour policy group %d: %w", currentDetour.ID, err)
			}
			// 策略组已完整处理，不再继续中转链（策略组自己会处理其成员的中转）
			break
		} else {
			// 中转到普通节点：添加节点配置
			var detourOutbound map[string]interface{}
			if err := json.Unmarshal([]byte(currentDetour.Config), &detourOutbound); err != nil {
				return "", fmt.Errorf("failed to parse detour config: %w", err)
			}
			detourTag := fmt.Sprintf("node-%d", currentDetour.ID)
			detourOutbound["tag"] = detourTag

			if !globalAdded[currentDetour.ID] {
				outbounds = append(outbounds, detourOutbound)
				globalAdded[currentDetour.ID] = true
			}

			// 尝试从当前 detour 的 config 中继续查找下一层
			var nextDetour *storage.ProxyNode
			if currentDetour.DetourID != nil {
				nextDetour, _ = storage.GetNode(*currentDetour.DetourID)
			} else if nextDetourTag, ok := detourOutbound["detour"].(string); ok {
				var detourID int
				if _, err := fmt.Sscanf(nextDetourTag, "node-%d", &detourID); err == nil {
					nextDetour, _ = storage.GetNode(detourID)
				}
			}
			currentDetour = nextDetour
		}
	}

	// 构建路由规则
	routeRules := []RouteRule{
		{Action: "sniff"},
	}
	// 根据 hijack_dns 配置添加 DNS 劫持规则
	if node.HijackDNS {
		routeRules = append(routeRules, RouteRule{Protocol: "dns", Action: "hijack-dns"})
	}

	// 构建入站配置（TUN + HTTP + SOCKS5 同时启用）
	var inbounds []Inbound

	// TUN 入站（始终启用）
	inbounds = append(inbounds, Inbound{
		Type:          "tun",
		InterfaceName: node.TunName,
		Address:       []string{node.TunAddress},
	})

	// HTTP 入站（根据ID自动计算端口：8000 + ID）
	httpPort := 8000 + node.ID
	inbounds = append(inbounds, Inbound{
		Type:       "http",
		Listen:     "::",
		ListenPort: httpPort,
		Tag:        "http-in",
	})

	// SOCKS5 入站（根据ID自动计算端口：5000 + ID）
	socks5Port := 5000 + node.ID
	inbounds = append(inbounds, Inbound{
		Type:       "socks",
		Listen:     "::",
		ListenPort: socks5Port,
		Tag:        "socks-in",
	})

	// 添加 direct 和 block outbounds
	outbounds = append(outbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "block", "tag": "block"},
	)

	// 构建完整配置
	config := &SingBoxConfig{
		Log: LogConfig{
			Level: "info",
		},
		DNS: DNSConfig{
			Servers: []DNSServer{
				{
					Tag:    "dns",
					Type:   "tls",
					Server: "1.0.0.1",
					Detour: nodeTag,
				},
			},
		},
		Route: RouteConfig{
			Rules: routeRules,
			Final: nodeTag,
		},
		Inbounds:  inbounds,
		Outbounds: outbounds,
	}

	// 序列化为 JSON
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	return string(configJSON), nil
}

// SaveConfig 保存配置文件
func SaveConfig(configDir string, nodeID int, configContent string) (string, error) {
	// 确保配置目录存在
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	// 配置文件路径
	configPath := filepath.Join(configDir, fmt.Sprintf("node_%d.json", nodeID))

	// 写入配置文件
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configPath, nil
}

// DeleteConfig 删除配置文件
func DeleteConfig(configDir string, nodeID int) error {
	configPath := filepath.Join(configDir, fmt.Sprintf("node_%d.json", nodeID))
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete config file: %w", err)
	}
	return nil
}

// processNodeRecursive 递归处理节点（普通节点或策略组），返回该节点的tag列表
// policyVisited: 策略组访问记录，用于检测策略组循环引用
// globalAdded: 全局已添加的节点，避免重复添加
// outbounds: 存储所有 outbound 配置
func processNodeRecursive(node *storage.ProxyNode, policyVisited map[int]bool, globalAdded map[int]bool, outbounds *[]interface{}) ([]string, error) {
	nodeTag := fmt.Sprintf("node-%d", node.ID)

	// 如果是普通节点
	if node.IsNormalNode() {
		// 解析节点配置
		var nodeOutbound map[string]interface{}
		if err := json.Unmarshal([]byte(node.Config), &nodeOutbound); err != nil {
			return nil, fmt.Errorf("failed to parse node %d config: %w", node.ID, err)
		}
		nodeOutbound["tag"] = nodeTag

		// 添加节点到 outbounds（如果还未添加）
		if !globalAdded[node.ID] {
			*outbounds = append(*outbounds, nodeOutbound)
			globalAdded[node.ID] = true
		}

		// 处理中转链
		chainVisited := make(map[int]bool)
		chainVisited[node.ID] = true

		// 优先从 config JSON 中获取 detour（支持策略组引用）
		currentDetour := (*storage.ProxyNode)(nil)
		if detourTag, ok := nodeOutbound["detour"].(string); ok {
			var detourID int
			if _, err := fmt.Sscanf(detourTag, "node-%d", &detourID); err == nil {
				currentDetour, _ = storage.GetNode(detourID)
			}
		} else if node.DetourID != nil {
			currentDetour, _ = storage.GetNode(*node.DetourID)
		}

		// 递归获取中转链
		for currentDetour != nil {
			if chainVisited[currentDetour.ID] {
				return nil, fmt.Errorf("circular detour reference detected in chain for node %d", node.ID)
			}
			chainVisited[currentDetour.ID] = true

			// 检查当前 detour 是否为策略组
			if currentDetour.IsPolicyGroup() {
				// 如果 detour 指向策略组，递归展开该策略组
				_, err := processNodeRecursive(currentDetour, policyVisited, globalAdded, outbounds)
				if err != nil {
					return nil, fmt.Errorf("failed to process detour policy group %d: %w", currentDetour.ID, err)
				}
				// 策略组已完整处理，终止中转链
				break
			}

			// 普通节点：添加配置
			var detourOutbound map[string]interface{}
			if err := json.Unmarshal([]byte(currentDetour.Config), &detourOutbound); err != nil {
				return nil, fmt.Errorf("failed to parse detour config: %w", err)
			}
			detourTag := fmt.Sprintf("node-%d", currentDetour.ID)
			detourOutbound["tag"] = detourTag

			if !globalAdded[currentDetour.ID] {
				*outbounds = append(*outbounds, detourOutbound)
				globalAdded[currentDetour.ID] = true
			}

			// 查找下一层中转（优先使用 config.detour）
			var nextDetour *storage.ProxyNode
			if nextDetourTag, ok := detourOutbound["detour"].(string); ok {
				var detourID int
				if _, err := fmt.Sscanf(nextDetourTag, "node-%d", &detourID); err == nil {
					nextDetour, _ = storage.GetNode(detourID)
				}
			} else if currentDetour.DetourID != nil {
				nextDetour, _ = storage.GetNode(*currentDetour.DetourID)
			}
			currentDetour = nextDetour
		}

		return []string{nodeTag}, nil
	}

	// 如果是策略组
	if node.IsPolicyGroup() {
		// 检测策略组循环引用
		if policyVisited[node.ID] {
			return nil, fmt.Errorf("circular policy group reference detected for policy group %d", node.ID)
		}
		policyVisited[node.ID] = true
		defer delete(policyVisited, node.ID) // 回溯时移除，允许不同分支重用

		// 解析策略组配置
		policyConfig, err := node.GetPolicyGroupConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get policy group %d config: %w", node.ID, err)
		}

		if len(policyConfig.Nodes) == 0 {
			return nil, fmt.Errorf("policy group %d has no member nodes", node.ID)
		}

		// 递归处理所有成员节点
		allMemberTags := make([]string, 0)
		for _, memberID := range policyConfig.Nodes {
			memberNode, err := storage.GetNode(memberID)
			if err != nil {
				return nil, fmt.Errorf("failed to get member node %d: %w", memberID, err)
			}

			memberTags, err := processNodeRecursive(memberNode, policyVisited, globalAdded, outbounds)
			if err != nil {
				return nil, err
			}
			allMemberTags = append(allMemberTags, memberTags...)
		}

		// 构建策略组 outbound
		policyOutbound := make(map[string]interface{})
		policyOutbound["tag"] = nodeTag
		policyOutbound["type"] = node.Type
		policyOutbound["outbounds"] = allMemberTags

		// 根据策略类型添加配置
		switch node.Type {
		case "urltest":
			if policyConfig.URL != "" {
				policyOutbound["url"] = policyConfig.URL
			} else {
				policyOutbound["url"] = "https://www.gstatic.com/generate_204"
			}
			if policyConfig.Interval != "" {
				policyOutbound["interval"] = policyConfig.Interval
			} else {
				policyOutbound["interval"] = "3m"
			}
			if policyConfig.Tolerance > 0 {
				policyOutbound["tolerance"] = policyConfig.Tolerance
			}
			if policyConfig.InterruptExistConnections {
				policyOutbound["interrupt_exist_connections"] = true
			}

		case "selector":
			if policyConfig.Default > 0 {
				defaultTag := fmt.Sprintf("node-%d", policyConfig.Default)
				// 验证默认节点是否在成员列表中
				validDefault := false
				for _, tag := range allMemberTags {
					if tag == defaultTag {
						validDefault = true
						break
					}
				}
				if validDefault {
					policyOutbound["default"] = defaultTag
				}
			}
			if policyConfig.InterruptExistConnections {
				policyOutbound["interrupt_exist_connections"] = true
			}

		case "fallback":
			if policyConfig.URL != "" {
				policyOutbound["url"] = policyConfig.URL
			} else {
				policyOutbound["url"] = "https://www.gstatic.com/generate_204"
			}
			if policyConfig.Interval != "" {
				policyOutbound["interval"] = policyConfig.Interval
			} else {
				policyOutbound["interval"] = "3m"
			}
		}

		// 添加策略组 outbound（如果还未添加）
		if !globalAdded[node.ID] {
			*outbounds = append(*outbounds, policyOutbound)
			globalAdded[node.ID] = true
		}

		return []string{nodeTag}, nil
	}

	return nil, fmt.Errorf("unknown node type for node %d", node.ID)
}

// GeneratePolicyGroupConfig 生成策略组的 sing-box 配置文件
func GeneratePolicyGroupConfig(node *storage.ProxyNode) (string, error) {
	// 验证是否为策略组
	if !node.IsPolicyGroup() {
		return "", fmt.Errorf("node %d is not a policy group", node.ID)
	}

	// 解析策略组配置
	policyConfig, err := node.GetPolicyGroupConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get policy group config: %w", err)
	}

	// 验证成员节点列表不为空
	if len(policyConfig.Nodes) == 0 {
		return "", fmt.Errorf("policy group %d has no member nodes", node.ID)
	}

	// 使用递归函数处理所有节点
	outbounds := make([]interface{}, 0)
	policyVisited := make(map[int]bool)
	globalAdded := make(map[int]bool)
	policyVisited[node.ID] = true // 防止自我引用

	memberTags := make([]string, 0)
	for _, memberID := range policyConfig.Nodes {
		memberNode, err := storage.GetNode(memberID)
		if err != nil {
			return "", fmt.Errorf("failed to get member node %d: %w", memberID, err)
		}

		tags, err := processNodeRecursive(memberNode, policyVisited, globalAdded, &outbounds)
		if err != nil {
			return "", err
		}
		memberTags = append(memberTags, tags...)
	}

	// 构建策略组 outbound
	policyTag := fmt.Sprintf("policy-%d", node.ID)
	policyOutbound := make(map[string]interface{})
	policyOutbound["tag"] = policyTag
	policyOutbound["type"] = node.Type // urltest/selector/fallback
	policyOutbound["outbounds"] = memberTags

	// 根据策略类型添加特定配置
	switch node.Type {
	case "urltest":
		// URL测试配置
		if policyConfig.URL != "" {
			policyOutbound["url"] = policyConfig.URL
		} else {
			policyOutbound["url"] = "https://www.gstatic.com/generate_204"
		}
		if policyConfig.Interval != "" {
			policyOutbound["interval"] = policyConfig.Interval
		} else {
			policyOutbound["interval"] = "3m"
		}
		if policyConfig.Tolerance > 0 {
			policyOutbound["tolerance"] = policyConfig.Tolerance
		}
		if policyConfig.InterruptExistConnections {
			policyOutbound["interrupt_exist_connections"] = true
		}

	case "selector":
		// 手动选择配置
		if policyConfig.Default > 0 {
			// 验证默认节点是否在成员列表中
			defaultTag := fmt.Sprintf("node-%d", policyConfig.Default)
			validDefault := false
			for _, tag := range memberTags {
				if tag == defaultTag {
					validDefault = true
					break
				}
			}
			if validDefault {
				policyOutbound["default"] = defaultTag
			}
		}
		if policyConfig.InterruptExistConnections {
			policyOutbound["interrupt_exist_connections"] = true
		}

	case "fallback":
		// 故障转移配置
		if policyConfig.URL != "" {
			policyOutbound["url"] = policyConfig.URL
		} else {
			policyOutbound["url"] = "https://www.gstatic.com/generate_204"
		}
		if policyConfig.Interval != "" {
			policyOutbound["interval"] = policyConfig.Interval
		} else {
			policyOutbound["interval"] = "3m"
		}
	}

	// 将策略 outbound 添加到列表首位
	outbounds = append([]interface{}{policyOutbound}, outbounds...)

	// 添加 direct 和 block outbounds
	outbounds = append(outbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "block", "tag": "block"},
	)

	// 构建入站配置（TUN + HTTP + SOCKS5）
	var inbounds []Inbound

	// TUN 入站
	inbounds = append(inbounds, Inbound{
		Type:          "tun",
		InterfaceName: node.TunName,
		Address:       []string{node.TunAddress},
	})

	// HTTP 入站（端口 = 8000 + ID）
	httpPort := 8000 + node.ID
	inbounds = append(inbounds, Inbound{
		Type:       "http",
		Listen:     "::",
		ListenPort: httpPort,
		Tag:        "http-in",
	})

	// SOCKS5 入站（端口 = 5000 + ID）
	socks5Port := 5000 + node.ID
	inbounds = append(inbounds, Inbound{
		Type:       "socks",
		Listen:     "::",
		ListenPort: socks5Port,
		Tag:        "socks-in",
	})

	// 构建路由规则
	routeRules := []RouteRule{
		{Action: "sniff"},
	}
	if node.HijackDNS {
		routeRules = append(routeRules, RouteRule{Protocol: "dns", Action: "hijack-dns"})
	}

	// 构建完整配置
	config := &SingBoxConfig{
		Log: LogConfig{
			Level: "info",
		},
		DNS: DNSConfig{
			Servers: []DNSServer{
				{
					Tag:    "dns",
					Type:   "tls",
					Server: "1.0.0.1",
					Detour: policyTag, // DNS 通过策略组路由
				},
			},
		},
		Route: RouteConfig{
			Rules: routeRules,
			Final: policyTag, // 所有流量最终路由到策略组
		},
		Inbounds:  inbounds,
		Outbounds: outbounds,
	}

	// 序列化为 JSON
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	return string(configJSON), nil
}
