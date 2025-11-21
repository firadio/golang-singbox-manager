package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// ProxyNode 代理节点模型
type ProxyNode struct {
	ID            int       `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Type          string    `json:"type" db:"type"` // socks/vless/vmess/trojan
	Config        string    `json:"config" db:"config"`
	DetourID      *int      `json:"detour_id,omitempty" db:"detour_id"`
	TunName       string    `json:"tun_name" db:"tun_name"`
	TunAddress    string    `json:"tun_address" db:"tun_address"`
	TableID       int       `json:"table_id" db:"table_id"`
	InboundType   string    `json:"inbound_type" db:"inbound_type"`       // tun/http/socks5 (deprecated, use multiple inbounds)
	InboundListen string    `json:"inbound_listen" db:"inbound_listen"`   // listen address for http/socks5
	InboundPort   *int      `json:"inbound_port,omitempty" db:"inbound_port"` // listen port for http/socks5
	HijackDNS     bool      `json:"hijack_dns" db:"hijack_dns"`           // enable DNS hijacking
	// HTTP和SOCKS5端口根据ID自动计算：HTTP=8000+ID, SOCKS5=5000+ID
	Enabled        bool   `json:"enabled" db:"enabled"`
	Status         string `json:"status" db:"status"` // stopped/starting/running/error
	Pid            int    `json:"pid,omitempty" db:"pid"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// RoutingRule 路由规则模型
type RoutingRule struct {
	ID         int       `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	SourceIP   string    `json:"source_ip" db:"source_ip"`
	SourceCIDR string    `json:"source_cidr,omitempty" db:"source_cidr"`
	NodeID     int       `json:"node_id" db:"node_id"`
	Priority   int       `json:"priority" db:"priority"`
	Enabled    bool      `json:"enabled" db:"enabled"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// NodeStats 节点统计模型
type NodeStats struct {
	ID            int       `json:"id" db:"id"`
	NodeID        int       `json:"node_id" db:"node_id"`
	Latency       int       `json:"latency" db:"latency"`               // TUN延迟
	HTTPLatency   int       `json:"http_latency" db:"http_latency"`     // HTTP代理延迟
	Socks5Latency int       `json:"socks5_latency" db:"socks5_latency"` // SOCKS5代理延迟
	TxBytes       int64     `json:"tx_bytes" db:"tx_bytes"`
	RxBytes       int64     `json:"rx_bytes" db:"rx_bytes"`
	LastCheck     time.Time `json:"last_check" db:"last_check"`
	Available     bool      `json:"available" db:"available"`
}

// OperationLog 操作日志模型
type OperationLog struct {
	ID         int       `json:"id" db:"id"`
	Operation  string    `json:"operation" db:"operation"`
	TargetType string    `json:"target_type,omitempty" db:"target_type"`
	TargetID   int       `json:"target_id,omitempty" db:"target_id"`
	Details    string    `json:"details,omitempty" db:"details"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// PortMapping 端口映射模型
type PortMapping struct {
	ID               int       `json:"id" db:"id"`
	Name             string    `json:"name" db:"name"`
	Protocol         string    `json:"protocol" db:"protocol"` // tcp/udp
	DstPort          int       `json:"dst_port" db:"dst_port"`
	ToAddress        string    `json:"to_address" db:"to_address"`
	ToPort           int       `json:"to_port" db:"to_port"`
	EnableMasquerade bool      `json:"enable_masquerade" db:"enable_masquerade"` // 启用路由器代理（masquerade）
	Enabled          bool      `json:"enabled" db:"enabled"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// PolicyGroupConfig 策略组配置
// 用于存储在 ProxyNode.Config 字段中（JSON格式）
type PolicyGroupConfig struct {
	Type                      string `json:"type"`                        // urltest/selector/fallback
	Nodes                     []int  `json:"nodes"`                       // 成员节点ID列表
	URL                       string `json:"url,omitempty"`               // 测试URL（urltest/fallback使用）
	Interval                  string `json:"interval,omitempty"`          // 测试间隔（urltest/fallback使用）
	Tolerance                 int    `json:"tolerance,omitempty"`         // 容差值ms（urltest使用）
	Default                   int    `json:"default,omitempty"`           // 默认节点ID（selector使用）
	InterruptExistConnections bool   `json:"interrupt_exist_connections"` // 切换时是否中断现有连接
}

// IsPolicyGroup 判断节点是否为策略组
func (n *ProxyNode) IsPolicyGroup() bool {
	return n.Type == "urltest" || n.Type == "selector" || n.Type == "fallback"
}

// IsNormalNode 判断节点是否为普通节点
func (n *ProxyNode) IsNormalNode() bool {
	return !n.IsPolicyGroup()
}

// GetPolicyGroupConfig 获取策略组配置
// 如果节点不是策略组或配置解析失败，返回错误
func (n *ProxyNode) GetPolicyGroupConfig() (*PolicyGroupConfig, error) {
	if !n.IsPolicyGroup() {
		return nil, fmt.Errorf("node %d is not a policy group", n.ID)
	}

	var config PolicyGroupConfig
	if err := json.Unmarshal([]byte(n.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse policy group config: %w", err)
	}

	return &config, nil
}

// SetPolicyGroupConfig 设置策略组配置
// 将 PolicyGroupConfig 序列化为 JSON 并存储到 Config 字段
func (n *ProxyNode) SetPolicyGroupConfig(config *PolicyGroupConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal policy group config: %w", err)
	}

	n.Config = string(configJSON)
	return nil
}
