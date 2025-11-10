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

	// 尝试从 config JSON 中获取 detour 引用
	var currentDetour *storage.ProxyNode
	if detourNode != nil {
		currentDetour = detourNode
	} else if detourTag, ok := nodeOutbound["detour"].(string); ok {
		// 从 "node-X" 格式中提取 ID
		var detourID int
		if _, err := fmt.Sscanf(detourTag, "node-%d", &detourID); err == nil {
			if detourNode, err := storage.GetNode(detourID); err == nil {
				currentDetour = detourNode
			}
		}
	}

	// 递归获取所有中转节点链
	for currentDetour != nil {
		// 防止循环引用
		if visited[currentDetour.ID] {
			return "", fmt.Errorf("circular detour reference detected for node %d", currentDetour.ID)
		}
		visited[currentDetour.ID] = true

		// 解析中转节点配置
		var detourOutbound map[string]interface{}
		if err := json.Unmarshal([]byte(currentDetour.Config), &detourOutbound); err != nil {
			return "", fmt.Errorf("failed to parse detour config: %w", err)
		}
		detourTag := fmt.Sprintf("node-%d", currentDetour.ID)
		detourOutbound["tag"] = detourTag
		outbounds = append(outbounds, detourOutbound)

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

	// 构建路由规则
	routeRules := []RouteRule{
		{Action: "sniff"},
	}
	// 根据 hijack_dns 配置添加 DNS 劫持规则
	if node.HijackDNS {
		routeRules = append(routeRules, RouteRule{Protocol: "dns", Action: "hijack-dns"})
	}

	// 构建入站配置
	var inbounds []Inbound
	switch node.InboundType {
	case "http":
		inbounds = []Inbound{
			{
				Type:       "http",
				Listen:     node.InboundListen,
				ListenPort: *node.InboundPort,
				Tag:        "http-in",
			},
		}
	case "socks5":
		inbounds = []Inbound{
			{
				Type:       "socks",
				Listen:     node.InboundListen,
				ListenPort: *node.InboundPort,
				Tag:        "socks-in",
			},
		}
	default: // tun
		inbounds = []Inbound{
			{
				Type:          "tun",
				InterfaceName: node.TunName,
				Address:       []string{node.TunAddress},
			},
		}
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
