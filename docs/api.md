# API 文档

Golang Sing-Box Manager RESTful API 文档

## 基础信息

- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json`
- **响应格式**:

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

- `code`: 0 表示成功，非 0 表示错误
- `message`: 提示信息
- `data`: 响应数据

## 节点管理 API

### 1. 获取所有节点

**请求**:
```
GET /api/nodes
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "name": "us-node",
      "type": "socks",
      "config": "{\"type\":\"socks\",\"server\":\"107.151.209.156\",\"server_port\":65480}",
      "tun_name": "tun1",
      "tun_address": "172.18.0.1/30",
      "table_id": 100,
      "enabled": true,
      "status": "running",
      "pid": 12345,
      "created_at": "2025-11-09T10:00:00Z",
      "updated_at": "2025-11-09T10:00:00Z"
    }
  ]
}
```

### 2. 获取单个节点

**请求**:
```
GET /api/nodes/:id
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "us-node",
    "type": "socks",
    "config": "{\"type\":\"socks\",\"server\":\"107.151.209.156\",\"server_port\":65480}",
    "tun_name": "tun1",
    "tun_address": "172.18.0.1/30",
    "table_id": 100,
    "enabled": true,
    "status": "running",
    "pid": 12345
  }
}
```

### 3. 创建节点

**请求**:
```
POST /api/nodes
Content-Type: application/json

{
  "name": "us-node",
  "type": "socks",
  "config": "{\"type\":\"socks\",\"server\":\"107.151.209.156\",\"server_port\":65480,\"version\":\"5\",\"username\":\"user\",\"password\":\"pass\"}",
  "enabled": true
}
```

**配置说明**:

`config` 字段是 sing-box outbound 配置的 JSON 字符串，支持以下类型：

#### SOCKS5 节点
```json
{
  "type": "socks",
  "server": "107.151.209.156",
  "server_port": 65480,
  "version": "5",
  "username": "user",
  "password": "pass"
}
```

#### VLESS 节点
```json
{
  "type": "vless",
  "server": "example.com",
  "server_port": 443,
  "uuid": "uuid-here",
  "tls": {
    "enabled": true,
    "server_name": "www.example.com"
  }
}
```

#### VMess 节点
```json
{
  "type": "vmess",
  "server": "example.com",
  "server_port": 443,
  "uuid": "uuid-here",
  "security": "auto",
  "alter_id": 0
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "us-node",
    "type": "socks",
    "tun_name": "tun1",
    "tun_address": "172.18.0.1/30",
    "table_id": 100,
    "status": "stopped"
  }
}
```

### 4. 更新节点

**请求**:
```
PUT /api/nodes/:id
Content-Type: application/json

{
  "name": "us-node-updated",
  "type": "socks",
  "config": "...",
  "enabled": true
}
```

### 5. 删除节点

**请求**:
```
DELETE /api/nodes/:id
```

**响应**:
```json
{
  "code": 0,
  "message": "success"
}
```

### 6. 启动节点

**请求**:
```
POST /api/nodes/:id/start
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "us-node",
    "status": "running",
    "pid": 12345
  }
}
```

### 7. 停止节点

**请求**:
```
POST /api/nodes/:id/stop
```

### 8. 重启节点

**请求**:
```
POST /api/nodes/:id/restart
```

### 9. 获取节点状态

**请求**:
```
GET /api/nodes/:id/status
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "running"
  }
}
```

**状态说明**:
- `stopped`: 已停止
- `starting`: 启动中
- `running`: 运行中
- `error`: 错误

## 路由规则 API

### 1. 获取所有规则

**请求**:
```
GET /api/rules
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "name": "route-192.168.100.101",
      "source_ip": "192.168.100.101",
      "node_id": 1,
      "priority": 100,
      "enabled": true,
      "created_at": "2025-11-09T10:00:00Z",
      "updated_at": "2025-11-09T10:00:00Z"
    }
  ]
}
```

### 2. 获取单个规则

**请求**:
```
GET /api/rules/:id
```

### 3. 创建规则

**请求**:
```
POST /api/rules
Content-Type: application/json

{
  "name": "route-192.168.100.101",
  "source_ip": "192.168.100.101",
  "node_id": 1,
  "enabled": true
}
```

**字段说明**:
- `name`: 规则名称
- `source_ip`: 源 IP 地址
- `node_id`: 关联的节点 ID
- `priority`: 优先级（可选，自动分配）
- `enabled`: 是否启用

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "route-192.168.100.101",
    "source_ip": "192.168.100.101",
    "node_id": 1,
    "priority": 100,
    "enabled": true
  }
}
```

### 4. 更新规则

**请求**:
```
PUT /api/rules/:id
Content-Type: application/json

{
  "name": "route-192.168.100.101-updated",
  "source_ip": "192.168.100.101",
  "node_id": 2,
  "priority": 100,
  "enabled": true
}
```

### 5. 删除规则

**请求**:
```
DELETE /api/rules/:id
```

### 6. 启用规则

**请求**:
```
POST /api/rules/:id/enable
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "enabled": true
  }
}
```

### 7. 禁用规则

**请求**:
```
POST /api/rules/:id/disable
```

## 系统 API

### 1. 获取系统信息

**请求**:
```
GET /api/system/info
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "version": "0.0.1",
    "name": "Golang Sing-Box Manager"
  }
}
```

### 2. 健康检查

**请求**:
```
GET /health
```

**响应示例**:
```json
{
  "status": "ok"
}
```

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1001 | 获取节点失败 |
| 1002 | 无效的节点 ID |
| 1003 | 节点未找到 |
| 1004 | 无效的请求体 |
| 1005 | 分配 Table ID 失败 |
| 1006 | 创建节点失败 |
| 1007 | 更新节点失败 |
| 1008 | 删除节点失败 |
| 1009 | 启动节点失败 |
| 1010 | 添加路由失败 |
| 1011 | 停止节点失败 |
| 1012 | 重启节点失败 |
| 1013 | 获取节点状态失败 |
| 2001 | 获取规则失败 |
| 2002 | 无效的规则 ID |
| 2003 | 规则未找到 |
| 2004 | 无效的请求体 |
| 2005 | 分配优先级失败 |
| 2006 | 创建规则失败 |
| 2007 | 更新规则失败 |
| 2008 | 删除规则失败 |
| 2009 | 启用规则失败 |
| 2010 | 获取节点失败 |
| 2011 | 添加路由规则失败 |
| 2012 | 禁用规则失败 |

## 使用示例

### 示例 1: 创建并启动一个 SOCKS 节点

```bash
# 1. 创建节点
curl -X POST http://localhost:8080/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "us-node",
    "type": "socks",
    "config": "{\"type\":\"socks\",\"server\":\"107.151.209.156\",\"server_port\":65480,\"version\":\"5\",\"username\":\"user\",\"password\":\"pass\"}",
    "enabled": true
  }'

# 2. 启动节点
curl -X POST http://localhost:8080/api/nodes/1/start

# 3. 检查状态
curl http://localhost:8080/api/nodes/1/status
```

### 示例 2: 为特定 IP 添加路由规则

```bash
# 1. 创建规则 (将 192.168.100.101 的流量路由到节点 1)
curl -X POST http://localhost:8080/api/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "route-client-1",
    "source_ip": "192.168.100.101",
    "node_id": 1,
    "enabled": true
  }'

# 2. 查看所有规则
curl http://localhost:8080/api/rules
```

### 示例 3: 创建带中转的节点

```bash
# 1. 先创建中转节点
curl -X POST http://localhost:8080/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hk-transit",
    "type": "vless",
    "config": "{\"type\":\"vless\",\"server\":\"hk.example.com\",\"server_port\":443,\"uuid\":\"xxx\"}",
    "enabled": true
  }'

# 2. 创建使用中转的节点
curl -X POST http://localhost:8080/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "us-via-hk",
    "type": "socks",
    "config": "{\"type\":\"socks\",\"server\":\"us.example.com\",\"server_port\":1080}",
    "detour_id": 1,
    "enabled": true
  }'
```

### 示例 4: 使用 Python 脚本管理

```python
import requests
import json

BASE_URL = "http://localhost:8080"

# 创建节点
def create_node(name, node_type, config):
    response = requests.post(f"{BASE_URL}/api/nodes", json={
        "name": name,
        "type": node_type,
        "config": json.dumps(config),
        "enabled": True
    })
    return response.json()

# 启动节点
def start_node(node_id):
    response = requests.post(f"{BASE_URL}/api/nodes/{node_id}/start")
    return response.json()

# 创建规则
def create_rule(name, source_ip, node_id):
    response = requests.post(f"{BASE_URL}/api/rules", json={
        "name": name,
        "source_ip": source_ip,
        "node_id": node_id,
        "enabled": True
    })
    return response.json()

# 使用示例
if __name__ == "__main__":
    # 创建 SOCKS 节点
    node_config = {
        "type": "socks",
        "server": "107.151.209.156",
        "server_port": 65480,
        "version": "5",
        "username": "user",
        "password": "pass"
    }

    result = create_node("my-proxy", "socks", node_config)
    node_id = result["data"]["id"]
    print(f"Created node ID: {node_id}")

    # 启动节点
    start_node(node_id)
    print(f"Started node {node_id}")

    # 添加路由规则
    create_rule("route-client-1", "192.168.100.101", node_id)
    print("Created routing rule")
```

## 注意事项

1. **权限要求**: Manager 需要 root 权限运行
2. **TUN 接口**: 启动节点会自动创建 TUN 接口（tun1, tun2...）
3. **路由表**: 每个节点自动分配独立的路由表（100, 101, 102...）
4. **进程独立**: Manager 终止不影响正在运行的 sing-box 进程
5. **自动恢复**: Manager 重启后会自动恢复之前的节点和规则
