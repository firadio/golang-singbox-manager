# DDNS 动态域名功能

## 功能概述

DDNS (Dynamic DNS) 动态域名功能允许您自动将域名解析更新到变化的 IP 地址。本系统支持以下特性：

- 支持 Cloudflare 作为 DNS 服务商
- 支持两种 IP 来源：公网 IP 和 Mikrotik 接口 IP
- 自动检测 IP 变化并更新 DNS 记录
- 支持 A 记录（IPv4）和 AAAA 记录（IPv6）
- Web 界面管理，操作简单直观

## 架构设计

### 模块组成

1. **Cloudflare API 客户端** (`internal/cloudflare/client.go`)
   - API Token 验证
   - 域名（Zone）管理
   - DNS 记录的增删改查
   - 自动创建或更新记录

2. **IP 检测服务** (`internal/ipdetect/ipdetect.go`)
   - 公网 IP 检测（通过 ip.sb）
   - Mikrotik 接口 IP 获取
   - 统一的 IP 获取接口

3. **数据存储层** (`internal/storage/ddns.go`)
   - DDNS 记录的 CRUD 操作
   - 记录启用/禁用管理
   - IP 更新记录

4. **API 处理器** (`internal/api/handlers/ddns.go`)
   - RESTful API 端点
   - 业务逻辑处理
   - 错误处理和验证

5. **前端页面** (`web/templates/pages/ddns.html`)
   - 用户界面
   - 交互逻辑
   - 实时状态显示

## 数据库设计

### ddns_records 表

```sql
CREATE TABLE IF NOT EXISTS ddns_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,                      -- 记录名称
    provider TEXT NOT NULL DEFAULT 'cloudflare',  -- 服务商
    api_token TEXT NOT NULL,                 -- API Token
    zone_id TEXT NOT NULL,                   -- Cloudflare Zone ID
    zone_name TEXT NOT NULL,                 -- 域名
    record_name TEXT NOT NULL,               -- 完整域名
    record_type TEXT NOT NULL DEFAULT 'A',   -- 记录类型（A/AAAA）
    ip_source TEXT NOT NULL DEFAULT 'public', -- IP 来源（public/interface）
    mikrotik_interface TEXT,                 -- Mikrotik 接口名称
    current_ip TEXT,                         -- 当前 IP
    last_ip TEXT,                            -- 上次 IP
    last_update DATETIME,                    -- 最后更新时间
    enabled BOOLEAN DEFAULT 1,               -- 是否启用
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(zone_id, record_name, record_type)
)
```

## API 端点

### DDNS 记录管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ddns` | 获取所有 DDNS 记录 |
| GET | `/api/ddns/:id` | 获取指定 DDNS 记录 |
| POST | `/api/ddns` | 创建 DDNS 记录 |
| PUT | `/api/ddns/:id` | 更新 DDNS 记录 |
| DELETE | `/api/ddns/:id` | 删除 DDNS 记录 |
| POST | `/api/ddns/:id/enable` | 启用 DDNS 记录 |
| POST | `/api/ddns/:id/disable` | 禁用 DDNS 记录 |
| POST | `/api/ddns/:id/update` | 立即更新 DDNS 记录 |

### Cloudflare 集成

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/ddns/test-cloudflare` | 测试 Cloudflare API Token |
| GET | `/api/ddns/cloudflare/zones` | 获取域名列表 |
| GET | `/api/ddns/cloudflare/records` | 获取 DNS 记录列表 |

### Mikrotik 集成

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ddns/mikrotik/interfaces` | 获取动态接口列表 |

## 使用指南

### 1. 准备工作

#### 获取 Cloudflare API Token

1. 登录 Cloudflare 控制台
2. 进入 "My Profile" → "API Tokens"
3. 点击 "Create Token"
4. 选择 "Edit zone DNS" 模板
5. 配置权限：
   - Zone: DNS: Edit
   - Zone: Zone: Read
6. 选择要管理的域名
7. 创建并保存 Token

#### 配置 Mikrotik（可选）

如果需要使用 Mikrotik 接口 IP，需要在系统设置中配置 Mikrotik 连接信息。

### 2. 创建 DDNS 记录

1. 访问 Web 界面，进入"动态域名"页面
2. 点击"+ 创建 DDNS"按钮
3. 填写基本信息：
   - **记录名称**：用于标识此 DDNS 记录
   - **服务商**：选择 Cloudflare
   - **API Token**：输入 Cloudflare API Token

4. 测试连接：
   - 点击"测试连接"按钮验证 Token 是否有效
   - 点击"加载域名列表"获取可管理的域名

5. 配置域名：
   - **主域名**：从列表中选择域名
   - **完整域名**：输入要更新的完整域名（如 home.example.com）
   - **记录类型**：选择 A（IPv4）或 AAAA（IPv6）

6. 选择 IP 来源：
   - **公网 IP**：自动从 ip.sb 获取公网 IP
   - **Mikrotik 接口**：从 Mikrotik 指定接口获取动态 IP
     - 点击"加载接口列表"获取可用接口
     - 选择具有动态 IP 的接口

7. 点击"创建"保存配置

### 3. 管理 DDNS 记录

#### 查看记录

DDNS 记录列表显示以下信息：
- ID
- 名称
- 域名
- 记录类型
- IP 来源
- 当前 IP
- 最后更新时间
- 状态（启用/禁用）

#### 手动更新

点击"立即更新"按钮可以手动触发 DNS 更新，系统会：
1. 获取当前 IP 地址
2. 检查 IP 是否变化
3. 如果 IP 有变化，更新 Cloudflare DNS 记录
4. 更新数据库中的 IP 信息和时间戳

#### 启用/禁用

- **启用**：开启自动更新功能
- **禁用**：停止自动更新

#### 编辑记录

点击"编辑"按钮可以修改 DDNS 记录配置，包括：
- 记录名称
- API Token
- 域名配置
- IP 来源

#### 删除记录

点击"删除"按钮可以删除 DDNS 记录。注意：删除操作不会删除 Cloudflare 上的 DNS 记录。

## 工作原理

### IP 检测流程

#### 公网 IP 检测

```go
// 通过 ip.sb API 获取公网 IP
func GetPublicIP() (string, error) {
    resp, err := http.Get("https://api.ip.sb/ip")
    // 处理响应，返回 IP 地址
    return ip, nil
}
```

#### Mikrotik 接口 IP 获取

```go
// 从 Mikrotik 获取指定接口的动态 IP
func GetMikrotikInterfaceIP(client *mikrotik.Client, interfaceName string) (string, error) {
    // 执行 /ip/address/print where dynamic
    addresses, err := client.GetAddresses()
    // 筛选指定接口的动态地址
    return ip, nil
}
```

### DNS 更新流程

1. **获取当前 IP**
   ```go
   currentIP, err := ipdetect.GetIP(record.IPSource, mtClient, record.MikrotikInterface)
   ```

2. **检查 IP 是否变化**
   ```go
   if record.CurrentIP == currentIP {
       // IP 未变化，无需更新
       return
   }
   ```

3. **更新 Cloudflare DNS**
   ```go
   cfClient := cloudflare.NewClient(record.APIToken)
   cfClient.UpdateOrCreateDNSRecord(
       record.ZoneID,
       record.RecordType,
       record.RecordName,
       currentIP,
       300,  // TTL: 5分钟
       false,
   )
   ```

4. **更新数据库记录**
   ```go
   storage.UpdateDDNSRecordIP(id, currentIP)
   ```

## Cloudflare API 集成

### API Token 权限要求

- Zone: DNS: Edit（必需）
- Zone: Zone: Read（推荐）

### API 端点使用

#### 验证 Token

```go
func (c *Client) VerifyToken() error {
    resp, err := c.doRequest("GET", "/user/tokens/verify", nil)
    return err
}
```

#### 列出域名

```go
func (c *Client) ListZones() ([]Zone, error) {
    resp, err := c.doRequest("GET", "/zones", nil)
    return zones, nil
}
```

#### 更新或创建 DNS 记录

```go
func (c *Client) UpdateOrCreateDNSRecord(zoneID, recordType, name, content string, ttl int, proxied bool) (*DNSRecord, error) {
    // 尝试获取现有记录
    existingRecord, err := c.GetDNSRecord(zoneID, name, recordType)
    if err == nil {
        // 记录存在，更新它
        if existingRecord.Content == content {
            return existingRecord, nil  // 无需更新
        }
        return c.UpdateDNSRecord(...)
    }
    // 记录不存在，创建它
    return c.CreateDNSRecord(...)
}
```

## Mikrotik 集成扩展

### 新增方法

为 Mikrotik 客户端添加了 `GetAddresses()` 方法：

```go
// Address IP地址信息
type Address struct {
    Interface string
    Address   string
    Dynamic   bool
}

// GetAddresses 获取所有IP地址
func (c *Client) GetAddresses() ([]Address, error) {
    items, err := c.runCommand("/ip/address/print", map[string]string{
        "?dynamic": "true",
    })
    // 解析并返回动态地址列表
    return addresses, nil
}
```

### 使用场景

- DDNS 功能获取接口 IP
- 获取 WAN 口动态公网 IP
- 获取 PPPoE 拨号接口 IP
- 获取其他动态分配的接口 IP

## 前端界面

### 页面结构

- **记录列表**：展示所有 DDNS 记录的表格
- **创建/编辑模态框**：表单式操作界面
- **操作按钮**：立即更新、启用/禁用、编辑、删除

### 交互流程

1. 用户输入 API Token
2. 测试 Token 有效性
3. 加载域名列表
4. 选择或输入域名信息
5. 选择 IP 来源
6. 如果选择 Mikrotik 接口，加载接口列表
7. 提交创建/更新

### JavaScript 功能

- `loadDDNSRecords()` - 加载记录列表
- `showCreateDDNSModal()` - 显示创建对话框
- `testCloudflareToken()` - 测试 API Token
- `loadCloudflareZones()` - 加载域名列表
- `loadMikrotikInterfaces()` - 加载接口列表
- `saveDDNS()` - 保存记录
- `updateDDNSNow()` - 立即更新
- `enableDDNS()` / `disableDDNS()` - 启用/禁用
- `editDDNS()` - 编辑记录
- `deleteDDNS()` - 删除记录

## 安全考虑

### API Token 保护

- API Token 在前端显示时会部分隐藏（只显示前6位和后4位）
- Token 存储在数据库中
- 建议定期更换 API Token

### 权限控制

- 所有 DDNS API 端点需要认证
- 使用中间件进行权限验证

### 错误处理

- API 调用失败时有详细的错误信息
- 网络超时设置为 30 秒
- Token 验证失败时提示用户检查权限

## 未来扩展

### 计划中的功能

1. **自动定时更新**
   - 添加后台定时任务
   - 定期检查 IP 变化并自动更新
   - 可配置更新间隔

2. **多服务商支持**
   - DNSPod
   - 阿里云 DNS
   - 腾讯云 DNS
   - AWS Route53

3. **通知功能**
   - IP 变化时发送通知
   - 更新失败时告警
   - 支持邮件、Webhook 等通知方式

4. **历史记录**
   - 记录 IP 变化历史
   - 更新操作日志
   - 故障排查辅助

5. **批量操作**
   - 批量启用/禁用
   - 批量更新
   - 批量删除

## 故障排查

### 常见问题

#### 1. API Token 验证失败

**原因**：
- Token 无效或已过期
- Token 权限不足
- 网络连接问题

**解决方法**：
- 检查 Token 是否正确
- 在 Cloudflare 控制台确认 Token 权限
- 测试网络连接到 api.cloudflare.com

#### 2. 无法获取域名列表

**原因**：
- API Token 缺少 Zone:Read 权限
- Token 未授权访问该域名

**解决方法**：
- 重新创建 Token 并添加正确权限
- 确认 Token 的域名授权范围

#### 3. Mikrotik 接口列表为空

**原因**：
- Mikrotik 未配置或连接失败
- 接口没有动态 IP

**解决方法**：
- 在系统设置中配置 Mikrotik 连接
- 测试 Mikrotik 连接
- 检查接口是否配置为动态 IP

#### 4. DNS 更新失败

**原因**：
- Cloudflare API 限流
- DNS 记录被锁定
- 网络超时

**解决方法**：
- 等待后重试
- 检查 Cloudflare 域名设置
- 查看系统日志获取详细错误信息

### 日志查看

```bash
# 查看系统日志
journalctl -u singbox-manager -f

# 查看最近的 DDNS 相关日志
journalctl -u singbox-manager | grep -i ddns

# 查看 Cloudflare API 调用日志
journalctl -u singbox-manager | grep -i cloudflare
```

## 性能优化

### 缓存策略

- Cloudflare 域名列表可以缓存
- IP 检测结果短期缓存
- 减少不必要的 API 调用

### 并发控制

- 使用连接池管理 HTTP 客户端
- 限制同时进行的 DNS 更新数量
- 避免 API 限流

### 数据库优化

- 为常用查询字段添加索引
- 定期清理过期的历史记录
- 使用事务保证数据一致性

## 开发指南

### 添加新的 DNS 服务商

1. 在 `internal/` 下创建新的服务商包
2. 实现以下接口：
   ```go
   type DNSProvider interface {
       VerifyToken() error
       ListZones() ([]Zone, error)
       ListDNSRecords(zoneID string) ([]DNSRecord, error)
       UpdateOrCreateDNSRecord(...) (*DNSRecord, error)
   }
   ```
3. 在 `internal/api/handlers/ddns.go` 中添加相应的处理逻辑
4. 更新前端页面支持新服务商

### 测试

```bash
# 运行单元测试
go test ./internal/cloudflare/...
go test ./internal/ipdetect/...
go test ./internal/storage/...

# 测试 Cloudflare API
go test ./internal/cloudflare/ -v -run TestClient

# 测试 API 端点
curl -X POST http://localhost/api/ddns/test-cloudflare \
  -H "Content-Type: application/json" \
  -d '{"api_token":"your-token"}'
```

## 参考资料

- [Cloudflare API 文档](https://developers.cloudflare.com/api/)
- [ip.sb API](https://ip.sb/)
- [Mikrotik API 文档](https://help.mikrotik.com/docs/display/ROS/API)
