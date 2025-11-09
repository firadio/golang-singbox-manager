# 部署文档

## 系统要求

- **操作系统**: Linux (Debian/Ubuntu 推荐)
- **权限**: root 权限
- **Go 版本**: 1.18+ (已安装在 /usr/local/go)
- **依赖**:
  - sing-box (已安装在 /usr/local/bin/sing-box)
  - gcc (用于编译 SQLite 支持)

## 安装步骤

### 1. 编译程序

```bash
cd /root/github.com/firadio/golang-singbox-manager

# 确保 Go 在 PATH 中
export PATH=$PATH:/usr/local/go/bin

# 编译 (CGO_ENABLED=1 用于 SQLite 支持)
CGO_ENABLED=1 go build -o singbox-manager cmd/manager/main.go
```

### 2. 配置文件

配置文件位于 `configs/config.yaml`，可根据需要修改：

```yaml
server:
  host: 0.0.0.0    # API 服务监听地址
  port: 8080       # API 服务端口

singbox:
  bin_path: /usr/local/bin/sing-box    # sing-box 可执行文件路径
  config_dir: configs/singbox           # sing-box 配置文件目录
  log_dir: logs/singbox                 # sing-box 日志目录

database:
  path: data/manager.db                 # SQLite 数据库路径

logging:
  level: info      # 日志级别: debug/info/warn/error
  format: text     # 日志格式: text/json
```

### 3. 手动运行

```bash
# 以 root 权限运行
sudo ./singbox-manager -config configs/config.yaml
```

### 4. 安装为系统服务

推荐使用 systemd 服务运行，确保系统重启后自动启动。

#### 创建 systemd 服务

```bash
# 复制可执行文件到系统路径
sudo cp singbox-manager /usr/local/bin/

# 创建服务文件
sudo tee /etc/systemd/system/singbox-manager.service > /dev/null <<EOF
[Unit]
Description=Golang Sing-Box Manager
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/github.com/firadio/golang-singbox-manager
ExecStart=/usr/local/bin/singbox-manager -config /root/github.com/firadio/golang-singbox-manager/configs/config.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

# 重载 systemd 配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start singbox-manager

# 设置开机自启
sudo systemctl enable singbox-manager

# 查看状态
sudo systemctl status singbox-manager
```

### 5. 查看日志

```bash
# 查看 Manager 日志
sudo journalctl -u singbox-manager -f

# 查看特定节点的 sing-box 日志
tail -f logs/singbox/node_1.log
```

## 目录结构说明

```
/root/github.com/firadio/golang-singbox-manager/
├── singbox-manager        # 编译后的可执行文件
├── configs/
│   ├── config.yaml        # 主配置文件
│   └── singbox/           # sing-box 运行时配置 (自动生成)
│       ├── node_1.json
│       ├── node_2.json
│       └── ...
├── data/
│   └── manager.db         # SQLite 数据库 (自动生成)
├── logs/
│   └── singbox/           # sing-box 日志 (自动生成)
│       ├── node_1.log
│       └── node_2.log
└── ...
```

## 网络配置

### 防火墙配置

如果需要从其他机器访问 API，需要开放端口：

```bash
# UFW 防火墙
sudo ufw allow 8080/tcp

# iptables 防火墙
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
```

### IP 转发

如果本机作为网关使用，需要开启 IP 转发：

```bash
# 临时开启
sudo sysctl -w net.ipv4.ip_forward=1

# 永久开启
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

## 升级和维护

### 升级程序

```bash
# 1. 拉取最新代码
cd /root/github.com/firadio/golang-singbox-manager
git pull

# 2. 重新编译
export PATH=$PATH:/usr/local/go/bin
CGO_ENABLED=1 go build -o singbox-manager cmd/manager/main.go

# 3. 重启服务
sudo systemctl restart singbox-manager
```

**注意**: 升级 Manager 不会影响正在运行的 sing-box 进程。

### 备份和恢复

#### 备份

```bash
# 备份数据库和配置
sudo tar -czf singbox-manager-backup-$(date +%Y%m%d).tar.gz \
  data/manager.db \
  configs/config.yaml \
  configs/singbox/
```

#### 恢复

```bash
# 解压备份
sudo tar -xzf singbox-manager-backup-20251109.tar.gz

# 重启服务
sudo systemctl restart singbox-manager
```

### 清理日志

```bash
# 清理旧日志 (保留最近 7 天)
find logs/singbox/ -name "*.log" -mtime +7 -delete
```

## 故障排查

### 1. Manager 无法启动

```bash
# 检查日志
sudo journalctl -u singbox-manager -n 50

# 常见问题:
# - 权限不足: 必须以 root 运行
# - 端口被占用: 修改 config.yaml 中的端口
# - 数据库损坏: 删除 data/manager.db 重新初始化
```

### 2. sing-box 进程启动失败

```bash
# 检查 sing-box 是否安装
which sing-box
/usr/local/bin/sing-box --version

# 检查 sing-box 日志
tail -f logs/singbox/node_1.log

# 手动测试配置
sing-box run -c configs/singbox/node_1.json
```

### 3. TUN 接口未创建

```bash
# 检查 TUN 接口
ip link show

# 检查内核模块
lsmod | grep tun

# 加载 TUN 模块 (如果未加载)
sudo modprobe tun
```

### 4. 路由规则未生效

```bash
# 查看路由规则
ip rule list

# 查看路由表
ip route show table 100

# 检查 TUN 接口状态
ip link show tun1
```

### 5. Manager 重启后节点未恢复

```bash
# 查看数据库中的节点
sqlite3 data/manager.db "SELECT * FROM proxy_nodes;"

# 查看进程是否存在
ps aux | grep sing-box

# 检查恢复日志
sudo journalctl -u singbox-manager | grep "Recovering"
```

## 性能优化

### 1. 数据库优化

如果节点和规则数量很大，可以优化数据库：

```bash
# 定期优化数据库
sqlite3 data/manager.db "VACUUM;"
```

### 2. 日志轮转

配置 logrotate 自动轮转日志：

```bash
sudo tee /etc/logrotate.d/singbox-manager > /dev/null <<EOF
/root/github.com/firadio/golang-singbox-manager/logs/singbox/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
}
EOF
```

## 安全建议

1. **限制 API 访问**: 默认监听 0.0.0.0，建议改为 127.0.0.1 或配置防火墙
2. **使用 HTTPS**: 在生产环境中，建议在前面加 Nginx 反向代理并启用 HTTPS
3. **定期备份**: 定期备份数据库和配置文件
4. **监控**: 建议配置监控系统（如 Prometheus）监控节点状态

## 监控和告警

### 使用 Prometheus 监控

可以扩展 Manager 添加 Prometheus metrics 端点：

```go
// 在 API server 中添加
router.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

### 健康检查

使用健康检查端点配置外部监控：

```bash
# 检查 Manager 是否存活
curl http://localhost:8080/health

# 检查节点状态
curl http://localhost:8080/api/nodes/1/status
```

## 卸载

```bash
# 停止并禁用服务
sudo systemctl stop singbox-manager
sudo systemctl disable singbox-manager

# 删除服务文件
sudo rm /etc/systemd/system/singbox-manager.service
sudo systemctl daemon-reload

# 删除可执行文件
sudo rm /usr/local/bin/singbox-manager

# 删除数据 (可选)
rm -rf /root/github.com/firadio/golang-singbox-manager/data
rm -rf /root/github.com/firadio/golang-singbox-manager/logs
rm -rf /root/github.com/firadio/golang-singbox-manager/configs/singbox
```

## 常见使用场景

### 场景 1: 家庭网关

作为家庭网关，为不同设备提供不同的代理线路：

1. 配置多个代理节点（美国、香港、日本等）
2. 为每个设备的 IP 配置路由规则
3. 设备通过网关上网时，自动使用对应的代理

### 场景 2: 负载均衡

为同一个 IP 配置多个规则（不同优先级），实现简单的负载均衡。

### 场景 3: 自动故障切换

配置主节点和备用节点，当主节点故障时，手动或通过脚本切换规则到备用节点。

## 版本说明

- **v0.0.x**: 开发版本，直接部署到生产环境
- **v1.0.x**: 稳定版本（计划），将包含测试模式和更多功能

## 技术支持

- 文档: `/docs`
- 问题反馈: GitHub Issues
