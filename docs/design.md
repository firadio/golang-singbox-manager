# Golang Sing-Box Manager 设计文档

## 1. 项目概述

### 1.1 项目目标
构建一个基于 Go 语言的 sing-box 代理盒子管理系统，用于网关设备的代理管理。系统通过为每个代理节点创建独立的 sing-box 进程和 TUN 接口，使用 Linux policy routing 实现灵活的流量调度。

### 1.2 核心特性
- **多节点管理**：支持多个代理节点，每个节点独立运行
- **进程隔离**：每个 sing-box 进程只负责一个 TUN 接口，提升稳定性
- **热更新**：修改节点配置时不影响其他线路，避免全局断网
- **策略路由**：基于源 IP 的策略路由，灵活调度流量
- **中转支持**：支持节点间的 detour 中转配置
- **Web 管理**：提供 Web 界面管理节点和路由规则
- **持久化**：配置持久化存储，系统重启后自动恢复

## 2. 系统架构

### 2.1 架构图
```
┌─────────────────────────────────────────────────────────┐
│                     Web Frontend                        │
│            (Node Management / Rule Management)          │
└────────────────────┬────────────────────────────────────┘
                     │ HTTP API
┌────────────────────┼────────────────────────────────────┐
│                    │        Manager Core                │
│  ┌─────────────────▼──────────────────────────────┐    │
│  │              API Server (HTTP)                  │    │
│  └─────────────────┬──────────────────────────────┘    │
│                    │                                     │
│  ┌─────────────────┼──────────────────────────────┐    │
│  │         Business Logic Layer                    │    │
│  │  ┌──────────────┴──────────────┐               │    │
│  │  │   Sing-box Manager          │               │    │
│  │  │  - Process Lifecycle        │               │    │
│  │  │  - Config Generation        │               │    │
│  │  │  - Health Check             │               │    │
│  │  └──────────────┬──────────────┘               │    │
│  │  ┌──────────────┴──────────────┐               │    │
│  │  │   Network Manager            │               │    │
│  │  │  - Policy Routing (netlink)  │               │    │
│  │  │  - TUN Monitor               │               │    │
│  │  └──────────────┬──────────────┘               │    │
│  │  ┌──────────────┴──────────────┐               │    │
│  │  │   Storage Manager            │               │    │
│  │  │  - SQLite DB                 │               │    │
│  │  │  - Config Persistence        │               │    │
│  │  └─────────────────────────────┘               │    │
│  └─────────────────────────────────────────────────┘    │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────┼──────────────────────────────────┐
│                 Linux Kernel                             │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐           │
│  │ sing-box  │  │ sing-box  │  │ sing-box  │  ...      │
│  │ Process 1 │  │ Process 2 │  │ Process 3 │           │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘           │
│        │              │              │                   │
│  ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐           │
│  │   tun1    │  │   tun2    │  │   tun3    │           │
│  └───────────┘  └───────────┘  └───────────┘           │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │         Policy Routing (ip rule + table)         │   │
│  │  - Table 100 -> tun1                             │   │
│  │  - Table 101 -> tun2                             │   │
│  │  - Rule: from 192.168.100.101 table 100 prio 100│   │
│  │  - Rule: from 192.168.100.102 table 101 prio 100│   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

### 2.2 目录结构
```
golang-singbox-manager/
├── cmd/
│   └── manager/
│       └── main.go              # 主程序入口
├── internal/
│   ├── singbox/                 # sing-box 进程管理模块
│   │   ├── manager.go           # 进程生命周期管理
│   │   ├── config.go            # sing-box 配置生成
│   │   ├── health.go            # 健康检查
│   │   └── process.go           # 进程控制
│   ├── network/                 # 网络配置模块
│   │   ├── routing.go           # 策略路由（netlink）
│   │   ├── rule.go              # 路由规则管理
│   │   ├── table.go             # 路由表管理
│   │   └── monitor.go           # TUN 接口监控
│   ├── storage/                 # 数据存储模块
│   │   ├── db.go                # SQLite 数据库
│   │   ├── models.go            # 数据模型
│   │   ├── node.go              # 节点 CRUD
│   │   └── rule.go              # 规则 CRUD
│   ├── api/                     # Web API 模块
│   │   ├── server.go            # HTTP 服务器
│   │   ├── handlers/            # API 处理器
│   │   │   ├── node.go          # 节点管理 API
│   │   │   ├── rule.go          # 规则管理 API
│   │   │   └── system.go        # 系统 API
│   │   └── middleware/          # 中间件
│   │       └── logger.go        # 日志中间件
│   └── config/                  # 配置管理模块
│       └── config.go            # 系统配置
├── web/                         # Web 前端
│   ├── static/                  # 静态资源
│   │   ├── css/
│   │   ├── js/
│   │   └── index.html
│   └── templates/               # 模板文件（如果使用模板）
├── configs/                     # 配置文件
│   ├── config.yaml              # 系统主配置
│   └── singbox/                 # sing-box 运行时配置目录
│       ├── node_1.json
│       ├── node_2.json
│       └── ...
├── data/                        # 数据目录
│   └── manager.db               # SQLite 数据库
├── logs/                        # 日志目录
│   ├── manager.log
│   └── singbox/
│       ├── node_1.log
│       └── node_2.log
├── test/                        # 测试代码
│   ├── singbox_test.go
│   ├── network_test.go
│   └── integration/
├── docs/                        # 文档
│   ├── design.md                # 设计文档（本文档）
│   ├── api.md                   # API 文档
│   ├── deployment.md            # 部署文档
│   └── sing-box-config.json     # sing-box 配置参考
├── scripts/                     # 脚本
│   ├── install.sh               # 安装脚本
│   └── uninstall.sh             # 卸载脚本
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 3. 核心模块设计

### 3.1 数据模型

#### 3.1.1 ProxyNode（代理节点）
```go
type ProxyNode struct {
    ID          int       `json:"id" db:"id"`
    Name        string    `json:"name" db:"name"`                    // 节点名称
    Type        string    `json:"type" db:"type"`                    // 节点类型：socks/vless/vmess/trojan
    Config      string    `json:"config" db:"config"`                // sing-box outbound 配置 JSON
    DetourID    *int      `json:"detour_id,omitempty" db:"detour_id"` // 中转节点 ID（可选）
    TunName     string    `json:"tun_name" db:"tun_name"`            // TUN 接口名：tun1, tun2...
    TunAddress  string    `json:"tun_address" db:"tun_address"`      // TUN 地址：172.18.x.x/30
    TableID     int       `json:"table_id" db:"table_id"`            // 路由表 ID：100, 101...
    Enabled     bool      `json:"enabled" db:"enabled"`              // 是否启用
    Status      string    `json:"status" db:"status"`                // 状态：stopped/starting/running/error
    Pid         int       `json:"pid,omitempty" db:"pid"`            // sing-box 进程 PID
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
```

#### 3.1.2 RoutingRule（路由规则）
```go
type RoutingRule struct {
    ID          int       `json:"id" db:"id"`
    Name        string    `json:"name" db:"name"`                    // 规则名称
    SourceIP    string    `json:"source_ip" db:"source_ip"`          // 源 IP：192.168.100.101
    SourceCIDR  string    `json:"source_cidr,omitempty" db:"source_cidr"` // 源网段（可选）
    NodeID      int       `json:"node_id" db:"node_id"`              // 关联的代理节点 ID
    Priority    int       `json:"priority" db:"priority"`            // 优先级：100-999
    Enabled     bool      `json:"enabled" db:"enabled"`              // 是否启用
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
```

#### 3.1.3 NodeStats（节点统计）
```go
type NodeStats struct {
    NodeID      int       `json:"node_id" db:"node_id"`
    Latency     int       `json:"latency" db:"latency"`              // 延迟（ms）
    TxBytes     int64     `json:"tx_bytes" db:"tx_bytes"`            // 发送字节数
    RxBytes     int64     `json:"rx_bytes" db:"rx_bytes"`            // 接收字节数
    LastCheck   time.Time `json:"last_check" db:"last_check"`        // 最后检查时间
    Available   bool      `json:"available" db:"available"`          // 是否可用
}
```

### 3.2 Sing-box 进程管理模块

#### 3.2.1 Manager 接口
```go
type Manager interface {
    // 启动节点
    StartNode(node *storage.ProxyNode) error

    // 停止节点
    StopNode(nodeID int) error

    // 重启节点
    RestartNode(nodeID int) error

    // 获取节点状态
    GetNodeStatus(nodeID int) (string, error)

    // 健康检查
    HealthCheck(nodeID int) (*HealthStatus, error)

    // 生成 sing-box 配置
    GenerateConfig(node *storage.ProxyNode) (string, error)
}
```

#### 3.2.2 配置生成逻辑
基于节点信息生成 sing-box 配置文件：
```go
func GenerateConfig(node *storage.ProxyNode) (string, error) {
    config := &SingBoxConfig{
        Log: LogConfig{Level: "info"},
        DNS: DNSConfig{
            Servers: []DNSServer{
                {Tag: "dns", Type: "tls", Server: "1.0.0.1", Detour: node.Name},
            },
        },
        Route: RouteConfig{
            Rules: []RouteRule{
                {Action: "sniff"},
                {Protocol: "dns", Action: "hijack-dns"},
            },
            Final: node.Name,
        },
        Inbounds: []Inbound{
            {
                Type:          "tun",
                InterfaceName: node.TunName,
                Address:       []string{node.TunAddress},
            },
        },
        Outbounds: parseOutbounds(node),
    }
    return json.MarshalIndent(config, "", "  ")
}
```

#### 3.2.3 进程生命周期管理
```go
// 启动 sing-box 进程
func (m *manager) StartNode(node *storage.ProxyNode) error {
    // 1. 生成配置文件
    configPath := fmt.Sprintf("configs/singbox/node_%d.json", node.ID)

    // 2. 启动进程
    cmd := exec.Command("sing-box", "run", "-c", configPath)
    cmd.Stdout = logFile
    cmd.Stderr = logFile

    // 3. 记录 PID
    // 4. 监控进程状态
}
```

### 3.3 网络管理模块（Policy Routing）

#### 3.3.1 路由管理接口
```go
type RoutingManager interface {
    // 添加路由规则
    AddRule(rule *storage.RoutingRule) error

    // 删除路由规则
    DeleteRule(ruleID int) error

    // 添加路由表默认路由
    AddTableRoute(tableID int, tunName string) error

    // 删除路由表
    DeleteTable(tableID int) error

    // 初始化系统路由
    InitSystemRouting() error
}
```

#### 3.3.2 使用 netlink 实现策略路由
```go
import "github.com/vishvananda/netlink"

// 添加 ip rule
func AddRule(sourceIP string, tableID int, priority int) error {
    rule := netlink.NewRule()
    rule.Src = &net.IPNet{
        IP:   net.ParseIP(sourceIP),
        Mask: net.CIDRMask(32, 32),
    }
    rule.Table = tableID
    rule.Priority = priority

    return netlink.RuleAdd(rule)
}

// 添加路由表默认路由
func AddTableRoute(tableID int, tunName string) error {
    link, err := netlink.LinkByName(tunName)
    if err != nil {
        return err
    }

    route := &netlink.Route{
        LinkIndex: link.Attrs().Index,
        Dst:       nil, // default route
        Table:     tableID,
    }

    return netlink.RouteAdd(route)
}
```

### 3.4 存储模块

#### 3.4.1 数据库设计（SQLite）
```sql
-- 节点表
CREATE TABLE proxy_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    config TEXT NOT NULL,
    detour_id INTEGER,
    tun_name TEXT NOT NULL UNIQUE,
    tun_address TEXT NOT NULL,
    table_id INTEGER NOT NULL UNIQUE,
    enabled BOOLEAN DEFAULT 1,
    status TEXT DEFAULT 'stopped',
    pid INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (detour_id) REFERENCES proxy_nodes(id) ON DELETE SET NULL
);

-- 路由规则表
CREATE TABLE routing_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    source_ip TEXT NOT NULL,
    source_cidr TEXT,
    node_id INTEGER NOT NULL,
    priority INTEGER NOT NULL,
    enabled BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES proxy_nodes(id) ON DELETE CASCADE,
    UNIQUE(source_ip, node_id)
);

-- 节点统计表
CREATE TABLE node_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id INTEGER NOT NULL,
    latency INTEGER DEFAULT 0,
    tx_bytes INTEGER DEFAULT 0,
    rx_bytes INTEGER DEFAULT 0,
    last_check DATETIME,
    available BOOLEAN DEFAULT 0,
    FOREIGN KEY (node_id) REFERENCES proxy_nodes(id) ON DELETE CASCADE
);

-- 操作日志表
CREATE TABLE operation_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    operation TEXT NOT NULL,
    target_type TEXT,
    target_id INTEGER,
    details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX idx_routing_rules_node_id ON routing_rules(node_id);
CREATE INDEX idx_routing_rules_enabled ON routing_rules(enabled);
CREATE INDEX idx_node_stats_node_id ON node_stats(node_id);
```

### 3.5 API 模块

#### 3.5.1 RESTful API 设计

**节点管理 API**
```
GET    /api/nodes              # 获取所有节点
GET    /api/nodes/:id          # 获取单个节点
POST   /api/nodes              # 创建节点
PUT    /api/nodes/:id          # 更新节点
DELETE /api/nodes/:id          # 删除节点
POST   /api/nodes/:id/start    # 启动节点
POST   /api/nodes/:id/stop     # 停止节点
POST   /api/nodes/:id/restart  # 重启节点
GET    /api/nodes/:id/status   # 获取节点状态
GET    /api/nodes/:id/stats    # 获取节点统计
POST   /api/nodes/:id/test     # 测试节点延迟
```

**路由规则 API**
```
GET    /api/rules              # 获取所有规则
GET    /api/rules/:id          # 获取单个规则
POST   /api/rules              # 创建规则
PUT    /api/rules/:id          # 更新规则
DELETE /api/rules/:id          # 删除规则
POST   /api/rules/:id/enable   # 启用规则
POST   /api/rules/:id/disable  # 禁用规则
```

**系统 API**
```
GET    /api/system/info        # 获取系统信息
GET    /api/system/logs        # 获取操作日志
POST   /api/system/reload      # 重载所有配置
GET    /api/system/export      # 导出配置
POST   /api/system/import      # 导入配置
```

#### 3.5.2 API 响应格式
```go
type Response struct {
    Code    int         `json:"code"`    // 0: 成功, 非0: 错误码
    Message string      `json:"message"` // 提示信息
    Data    interface{} `json:"data,omitempty"`
}

// 成功响应
{
    "code": 0,
    "message": "success",
    "data": {...}
}

// 错误响应
{
    "code": 1001,
    "message": "node not found"
}
```

## 4. 关键技术方案

### 4.1 路由表和优先级分配策略

#### 4.1.1 路由表编号
- **系统保留**：0-99（main=254, default=253, local=255）
- **节点路由表**：100-999
  - 算法：`tableID = 100 + nodeID`
  - 最多支持 900 个节点

#### 4.1.2 路由规则优先级
- **系统保留**：0-99, 32766-32767
- **用户规则**：100-999
  - 默认优先级：`priority = 100 + ruleID`
  - 优先级越小越优先

### 4.2 防止重启跳本地机制

#### 4.2.1 启动顺序
```
1. 初始化数据库连接
2. 设置兜底路由规则（防止流量泄露）
   - ip rule add from all lookup main prio 32766
3. 按顺序启动各 sing-box 进程
4. 等待 TUN 接口就绪（最多等待 10 秒）
5. 为每个 TUN 添加路由表默认路由
6. 加载用户自定义路由规则
7. 启动 Web 服务
```

#### 4.2.2 TUN 接口就绪检测
```go
func WaitForTun(tunName string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        link, err := netlink.LinkByName(tunName)
        if err == nil && link.Attrs().Flags&net.FlagUp != 0 {
            return nil
        }
        time.Sleep(500 * time.Millisecond)
    }
    return fmt.Errorf("timeout waiting for tun %s", tunName)
}
```

### 4.3 健康检查机制

#### 4.3.1 检查方式
- **进程检查**：检查 sing-box 进程是否存在
- **TUN 检查**：检查 TUN 接口是否 UP
- **连通性检查**：通过节点 ping 外部地址（如 8.8.8.8）
- **延迟测试**：HTTP 请求测速

#### 4.3.2 检查频率
- **快速检查**：每 30 秒检查进程和 TUN 状态
- **慢速检查**：每 5 分钟检查连通性和延迟

#### 4.3.3 故障处理
- 进程崩溃：自动重启（最多重试 3 次）
- TUN 消失：重启 sing-box 进程
- 连通性失败：标记节点不可用，记录日志

### 4.4 配置持久化和恢复

#### 4.4.1 持久化内容
- 节点配置（数据库 + sing-box 配置文件）
- 路由规则（数据库）
- 系统设置（config.yaml）

#### 4.4.2 启动时恢复流程
```go
func RestoreOnStartup() error {
    // 1. 从数据库加载所有启用的节点
    nodes, err := storage.GetEnabledNodes()

    // 2. 为每个节点启动 sing-box
    for _, node := range nodes {
        singbox.StartNode(node)
    }

    // 3. 从数据库加载所有启用的路由规则
    rules, err := storage.GetEnabledRules()

    // 4. 应用路由规则
    for _, rule := range rules {
        network.AddRule(rule)
    }

    return nil
}
```

### 4.5 TUN 地址和接口名分配

#### 4.5.1 TUN 接口命名
- 格式：`tun{nodeID}`
- 示例：`tun1`, `tun2`, `tun3`

#### 4.5.2 TUN 地址分配
- 使用 172.18.0.0/16 网段
- 每个 TUN 分配 /30 网段（4 个 IP，实际可用 2 个）
- 算法：
  ```go
  func GetTunAddress(nodeID int) string {
      base := 172*256*256*256 + 18*256*256 // 172.18.0.0
      offset := (nodeID - 1) * 4
      ip := base + offset + 1
      return fmt.Sprintf("%d.%d.%d.%d/30",
          (ip>>24)&0xFF, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
  }
  ```
- 示例：
  - Node 1: 172.18.0.1/30
  - Node 2: 172.18.0.5/30
  - Node 3: 172.18.0.9/30

## 5. 开发计划

### Phase 1: 核心功能（2-3 天）
- [x] 项目初始化，目录结构搭建
- [ ] 数据库模型和 CRUD 操作
- [ ] Sing-box 进程管理
- [ ] Netlink 路由管理
- [ ] 配置生成和持久化
- [ ] 启动恢复机制

### Phase 2: Web 管理（2-3 天）
- [ ] RESTful API 实现
- [ ] 节点管理 API（CRUD + 启停）
- [ ] 规则管理 API（CRUD）
- [ ] 简单的 Web UI
  - 节点列表和管理
  - 规则列表和管理
  - 系统状态展示

### Phase 3: 增强功能（1-2 天）
- [ ] 健康检查和监控
- [ ] 延迟测试
- [ ] 流量统计
- [ ] 操作日志
- [ ] 配置导入导出

### Phase 4: 测试和优化（1-2 天）
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能优化
- [ ] 文档完善

## 6. 技术选型

### 6.1 核心依赖
- **Web 框架**：`github.com/gin-gonic/gin`（轻量高性能）
- **数据库**：SQLite + `github.com/mattn/go-sqlite3`
- **网络管理**：`github.com/vishvananda/netlink`
- **配置管理**：`gopkg.in/yaml.v3`
- **日志**：`github.com/sirupsen/logrus`
- **进程管理**：标准库 `os/exec`

### 6.2 前端技术
- **基础**：HTML5 + CSS3 + JavaScript（Vanilla JS 或 Vue.js）
- **UI 框架**：Bootstrap 5 或 Tailwind CSS
- **HTTP 客户端**：Fetch API 或 Axios

## 7. 安全考虑

### 7.1 权限要求
- 程序需要 root 权限（管理网络和启动 sing-box）
- 建议使用 systemd 服务运行

### 7.2 配置安全
- 敏感信息（密码、UUID）存储在数据库中
- API 可选添加认证（JWT 或 Basic Auth）

### 7.3 防止流量泄露
- 启动时先设置兜底规则
- 节点停止时保留路由规则指向其他节点
- 提供"全局直连"紧急开关

## 8. 运维和监控

### 8.1 日志管理
- **Manager 日志**：`logs/manager.log`
- **Sing-box 日志**：`logs/singbox/node_{id}.log`
- **日志轮转**：每日轮转，保留 7 天

### 8.2 系统服务
```ini
[Unit]
Description=Golang Sing-Box Manager
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/singbox-manager
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

### 8.3 监控指标
- 节点状态（running/stopped/error）
- TUN 接口状态
- 流量统计（TX/RX bytes）
- 节点延迟
- 进程 CPU 和内存使用

## 9. 未来扩展

### 9.1 功能扩展
- [ ] 节点自动测速和切换
- [ ] 负载均衡（多个源 IP 分配到多个节点）
- [ ] 基于域名/IP 的路由规则
- [ ] 订阅链接导入
- [ ] 图表和可视化

### 9.2 性能优化
- [ ] 缓存节点状态
- [ ] 批量路由规则更新
- [ ] 使用 eBPF 加速路由

### 9.3 高可用
- [ ] 节点故障自动切换
- [ ] 配置热备份
- [ ] 多管理节点（主从）

## 10. 参考资料

- [Sing-box 官方文档](https://sing-box.sagernet.org/)
- [Netlink 库文档](https://github.com/vishvananda/netlink)
- [Linux Policy Routing](https://www.kernel.org/doc/Documentation/networking/policy-routing.txt)
- [Gin Web Framework](https://gin-gonic.com/)

---

**文档版本**：v1.0
**创建日期**：2025-11-09
**最后更新**：2025-11-09
