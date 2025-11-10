package storage

import "time"

// ProxyNode 代理节点模型
type ProxyNode struct {
	ID         int       `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	Type       string    `json:"type" db:"type"` // socks/vless/vmess/trojan
	Config     string    `json:"config" db:"config"`
	DetourID   *int      `json:"detour_id,omitempty" db:"detour_id"`
	TunName    string    `json:"tun_name" db:"tun_name"`
	TunAddress string    `json:"tun_address" db:"tun_address"`
	TableID    int       `json:"table_id" db:"table_id"`
	Enabled    bool      `json:"enabled" db:"enabled"`
	Status     string    `json:"status" db:"status"` // stopped/starting/running/error
	Pid        int       `json:"pid,omitempty" db:"pid"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
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
	ID        int       `json:"id" db:"id"`
	NodeID    int       `json:"node_id" db:"node_id"`
	Latency   int       `json:"latency" db:"latency"`
	TxBytes   int64     `json:"tx_bytes" db:"tx_bytes"`
	RxBytes   int64     `json:"rx_bytes" db:"rx_bytes"`
	LastCheck time.Time `json:"last_check" db:"last_check"`
	Available bool      `json:"available" db:"available"`
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
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Protocol  string    `json:"protocol" db:"protocol"` // tcp/udp
	DstPort   int       `json:"dst_port" db:"dst_port"`
	ToAddress string    `json:"to_address" db:"to_address"`
	ToPort    int       `json:"to_port" db:"to_port"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
