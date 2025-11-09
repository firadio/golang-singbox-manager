# Golang Sing-Box Manager

基于 Go 语言的 sing-box 代理盒子管理系统，用于网关设备的代理管理。

## 🌐 快速访问

**Web 管理界面**: `http://localhost:8080`（或使用服务器 IP）

![Status](https://img.shields.io/badge/status-active-success)
![Version](https://img.shields.io/badge/version-0.0.1-blue)

## 特性

- **多节点管理**：支持多个代理节点，每个节点独立运行
- **进程隔离**：每个 sing-box 进程只负责一个 TUN 接口，提升稳定性
- **热更新**：修改节点配置时不影响其他线路，避免全局断网
- **策略路由**：基于源 IP 的策略路由，灵活调度流量
- **中转支持**：支持节点间的 detour 中转配置
- **Web 管理**：提供 RESTful API 管理节点和路由规则
- **持久化**：配置持久化存储，系统重启后自动恢复
- **进程独立**：Manager 程序终止不影响正在运行的 sing-box 进程

## 系统要求

- Linux 系统（需要 root 权限）
- Go 1.18+ （已安装在 /usr/local/go）
- sing-box （已安装）

## 快速开始

### 1. 编译

```bash
make build
```

### 2. 运行

```bash
sudo make run
```

### 3. 安装为系统服务

```bash
sudo make install
sudo systemctl start singbox-manager
sudo systemctl enable singbox-manager
```

### 4. 查看状态

```bash
make status
```

### 5. 查看日志

```bash
make logs
```

## 配置

配置文件位于 `configs/config.yaml`：

```yaml
server:
  host: 0.0.0.0
  port: 8080

singbox:
  bin_path: /usr/local/bin/sing-box
  config_dir: configs/singbox
  log_dir: logs/singbox

database:
  path: data/manager.db

logging:
  level: info
  format: text
```

## API 文档

### 节点管理

- `GET /api/nodes` - 获取所有节点
- `GET /api/nodes/:id` - 获取单个节点
- `POST /api/nodes` - 创建节点
- `PUT /api/nodes/:id` - 更新节点
- `DELETE /api/nodes/:id` - 删除节点
- `POST /api/nodes/:id/start` - 启动节点
- `POST /api/nodes/:id/stop` - 停止节点
- `POST /api/nodes/:id/restart` - 重启节点
- `GET /api/nodes/:id/status` - 获取节点状态

### 路由规则管理

- `GET /api/rules` - 获取所有规则
- `GET /api/rules/:id` - 获取单个规则
- `POST /api/rules` - 创建规则
- `PUT /api/rules/:id` - 更新规则
- `DELETE /api/rules/:id` - 删除规则
- `POST /api/rules/:id/enable` - 启用规则
- `POST /api/rules/:id/disable` - 禁用规则

### 系统

- `GET /api/system/info` - 获取系统信息
- `GET /health` - 健康检查

## 使用示例

### 创建节点

```bash
curl -X POST http://localhost:8080/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "us-node",
    "type": "socks",
    "config": "{\"type\":\"socks\",\"server\":\"107.151.209.156\",\"server_port\":65480,\"version\":\"5\",\"username\":\"65480\",\"password\":\"65480\"}",
    "enabled": true
  }'
```

### 启动节点

```bash
curl -X POST http://localhost:8080/api/nodes/1/start
```

### 创建路由规则

```bash
curl -X POST http://localhost:8080/api/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "route-192.168.100.101",
    "source_ip": "192.168.100.101",
    "node_id": 1,
    "enabled": true
  }'
```

## 架构设计

详细的架构设计请参考 [docs/design.md](docs/design.md)

## 版本

当前版本：v0.0.1

## 开发

### 项目结构

```
golang-singbox-manager/
├── cmd/manager/        # 主程序
├── internal/           # 内部包
│   ├── api/           # API 服务
│   ├── config/        # 配置管理
│   ├── network/       # 网络路由管理
│   ├── singbox/       # sing-box 进程管理
│   └── storage/       # 数据存储
├── configs/           # 配置文件
├── data/              # 数据目录
├── logs/              # 日志目录
├── docs/              # 文档
└── test/              # 测试代码
```

### 运行测试

```bash
make test
```

### 格式化代码

```bash
make fmt
```

## 许可证

MIT License

## 作者

Generated with Claude Code
