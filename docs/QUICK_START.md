# 快速开始指南

## 访问地址

### Web 管理界面
```
http://localhost:8080
```

或者使用服务器 IP 地址：
```
http://<服务器IP>:8080
```

### API 接口
```
http://localhost:8080/api
```

## 系统服务管理

### 查看服务状态
```bash
systemctl status singbox-manager
```

### 启动/停止/重启服务
```bash
systemctl start singbox-manager   # 启动
systemctl stop singbox-manager    # 停止
systemctl restart singbox-manager # 重启
```

### 查看日志
```bash
# 实时查看 Manager 日志
journalctl -u singbox-manager -f

# 查看最近 50 行日志
journalctl -u singbox-manager -n 50

# 或使用脚本
./scripts/logs.sh manager
./scripts/logs.sh follow
```

### 禁用/启用开机自启
```bash
systemctl disable singbox-manager  # 禁用自启
systemctl enable singbox-manager   # 启用自启
```

## 管理脚本

项目提供了便捷的管理脚本（位于 `scripts/` 目录）：

### 1. 查看系统状态
```bash
./scripts/status.sh
```
显示：
- 服务状态
- API 健康检查
- 所有节点列表
- 所有路由规则
- 运行中的 sing-box 进程

### 2. 查看日志
```bash
./scripts/logs.sh manager    # 查看最近 50 行 Manager 日志
./scripts/logs.sh follow     # 实时跟踪 Manager 日志
./scripts/logs.sh node 1     # 查看节点 1 的 sing-box 日志
```

### 3. 备份数据
```bash
./scripts/backup.sh
```
自动备份：
- 数据库（data/manager.db）
- 配置文件（configs/config.yaml）
- Sing-box 配置（configs/singbox/）

备份文件保存在 `/root/singbox-manager-backups/`

## 使用示例

### 示例 1：创建并启动一个节点

#### 通过 Web UI：
1. 访问 http://localhost:8080
2. 点击"节点管理"标签
3. 点击"+ 创建节点"按钮
4. 填写节点信息：
   - 名称：us-node
   - 类型：SOCKS5
   - 配置：
     ```json
     {
       "type": "socks",
       "server": "107.151.209.156",
       "server_port": 65480,
       "version": "5",
       "username": "65480",
       "password": "65480"
     }
     ```
5. 点击"创建"
6. 在节点列表中点击"启动"按钮

#### 通过 API：
```bash
# 1. 创建节点
curl -X POST http://localhost:8080/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "us-node",
    "type": "socks",
    "config": "{\"type\":\"socks\",\"server\":\"107.151.209.156\",\"server_port\":65480,\"version\":\"5\",\"username\":\"65480\",\"password\":\"65480\"}",
    "enabled": true
  }'

# 2. 启动节点（假设节点 ID 为 1）
curl -X POST http://localhost:8080/api/nodes/1/start
```

### 示例 2：添加路由规则

#### 通过 Web UI：
1. 访问 http://localhost:8080
2. 点击"路由规则"标签
3. 点击"+ 创建规则"按钮
4. 填写规则信息：
   - 名称：route-client-1
   - 源 IP：192.168.100.101
   - 目标节点：选择已创建的节点
5. 点击"创建"

#### 通过 API：
```bash
curl -X POST http://localhost:8080/api/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "route-client-1",
    "source_ip": "192.168.100.101",
    "node_id": 1,
    "enabled": true
  }'
```

### 示例 3：验证配置

```bash
# 查看节点状态
curl http://localhost:8080/api/nodes/1/status

# 查看所有节点
curl http://localhost:8080/api/nodes | python3 -m json.tool

# 查看所有规则
curl http://localhost:8080/api/rules | python3 -m json.tool

# 检查 TUN 接口
ip link show tun1

# 检查路由规则
ip rule list | grep "lookup 100"

# 检查路由表
ip route show table 100
```

## 常见问题

### Q: Web UI 无法访问？
A: 检查服务是否运行：
```bash
systemctl status singbox-manager
curl http://localhost:8080/health
```

### Q: 节点启动失败？
A: 查看日志：
```bash
journalctl -u singbox-manager -n 50
tail -f logs/singbox/node_1.log
```

### Q: 如何修改 API 监听端口？
A: 编辑配置文件 `configs/config.yaml`：
```yaml
server:
  port: 8888  # 改为其他端口
```
然后重启服务：
```bash
systemctl restart singbox-manager
```

### Q: Manager 重启后节点会断网吗？
A: **不会**。sing-box 进程独立运行，Manager 重启不影响代理服务。

### Q: 系统重启后会自动启动吗？
A: 会。服务已设置为开机自启（enabled）。

## 文件位置

- **可执行文件**：`/usr/local/bin/singbox-manager`
- **工作目录**：`/root/github.com/firadio/golang-singbox-manager`
- **配置文件**：`/root/github.com/firadio/golang-singbox-manager/configs/config.yaml`
- **数据库**：`/root/github.com/firadio/golang-singbox-manager/data/manager.db`
- **日志目录**：`/root/github.com/firadio/golang-singbox-manager/logs/singbox/`
- **服务文件**：`/etc/systemd/system/singbox-manager.service`
- **日志轮转**：`/etc/logrotate.d/singbox-manager`

## 下一步

- 阅读完整 [API 文档](api.md)
- 查看 [部署文档](deployment.md)
- 了解 [架构设计](design.md)

## 技术支持

如有问题，请查看：
- 服务日志：`journalctl -u singbox-manager -f`
- Sing-box 日志：`logs/singbox/node_*.log`
- 或运行状态检查脚本：`./scripts/status.sh`
