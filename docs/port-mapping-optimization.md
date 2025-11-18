# 端口映射同步优化说明

## 问题背景

在实现"启用路由器代理功能"（Masquerade/SNAT）时，发现以下问题：

### 问题1：字段未保存
**现象**：`enable_masquerade` 字段无法保存到数据库
**原因**：`UpdatePortMapping` 函数中缺少对 `EnableMasquerade` 字段的赋值
**位置**：`internal/api/handlers/port_mapping.go:129`
**修复**：添加 `existing.EnableMasquerade = updateData.EnableMasquerade`

### 问题2：并发同步冲突（核心问题）
**现象**：规则被删完也没加上来，规则状态混乱
**原因**：多个同步操作并发执行时互相干扰

#### 问题详解
当用户快速连续操作（启用/禁用/更新）时，每个操作都会触发同步：

```
时间线示例：
19:56:35 - 操作A开始：获取规则 → 删除规则 → 准备添加规则
19:56:35 - 操作B开始：获取规则 → 删除规则（把操作A刚添加的也删了！）
19:56:36 - 操作A完成：添加规则
19:56:36 - 操作B完成：添加规则
结果：规则被反复删除添加，状态不一致
```

**修复方案**：
1. 添加 `sync.Mutex` 互斥锁，确保同步操作串行执行
2. 拆分全量同步和增量同步：
   - `SyncPortMappings()`: 全量同步所有映射（手动触发或系统启动）
   - `SyncSinglePortMapping()`: 增量同步单个映射（创建/更新/启用时）
   - `DeletePortMappingRules()`: 删除单个映射的规则（禁用/删除时）

## 优化效果

### 性能提升
- **优化前**：每次操作都同步所有映射的规则（例如5个映射 = 15条规则）
- **优化后**：每次操作只同步相关映射的规则（例如1个映射 = 3条规则）
- **效率提升**：80%（5个映射的场景）

### 操作对比

| 操作 | 优化前 | 优化后 |
|------|--------|--------|
| 创建映射 | 删除并重建所有15条规则 | 只添加3条新规则 |
| 更新映射 | 删除并重建所有15条规则 | 只更新3条相关规则 |
| 启用映射 | 删除并重建所有15条规则 | 只添加3条相关规则 |
| 禁用映射 | 删除并重建所有15条规则 | 只删除3条相关规则 |
| 删除映射 | 删除并重建所有15条规则 | 只删除3条相关规则 |
| 手动同步 | 删除并重建所有15条规则 | 删除并重建所有15条规则（不变）|

## 技术细节

### NAT 规则结构
每个端口映射最多包含3条 MikroTik NAT 规则：

1. **Interface Rule** (dstnat)
   - Chain: `dstnat`
   - Action: `dst-nat`
   - Match: `in-interface-list=list-wan`
   - Comment: `web-id-{ID}`

2. **Address Rule** (dstnat)
   - Chain: `dstnat`
   - Action: `dst-nat`
   - Match: `dst-address-list=WAN`
   - Comment: `web-id-{ID}`

3. **Masquerade Rule** (srcnat) - 可选
   - Chain: `srcnat`
   - Action: `masquerade`
   - Match: `dst-address={ToAddress}, dst-port={ToPort}`
   - Comment: `web-id-{ID}-masq`

### 规则识别机制
- 使用 Comment 字段标识系统管理的规则
- Comment 格式：`web-id-{映射ID}` 或 `web-id-{映射ID}-masq`
- 通过解析 Comment 将规则归类到对应的端口映射

### Mutex 保护范围
所有涉及 MikroTik 规则增删改的操作都受互斥锁保护：
- `SyncPortMappings()` - 全量同步
- `SyncSinglePortMapping()` - 单个同步
- `DeletePortMappingRules()` - 删除规则

## 代码位置

### 核心文件
- `internal/mikrotik/client.go` - Client 结构体，包含 syncMux
- `internal/mikrotik/nat.go` - NAT 规则管理核心逻辑
- `internal/api/handlers/port_mapping.go` - API 处理器

### 关键函数
- `GetManagedNATRules()` - 获取系统管理的所有规则
- `AddNATRule()` - 添加 dstnat 转发规则
- `AddMasqueradeRule()` - 添加 srcnat masquerade 规则
- `DeleteNATRule()` - 删除单条规则
- `SyncPortMappings()` - 全量同步
- `SyncSinglePortMapping()` - 增量同步（新增）
- `DeletePortMappingRules()` - 删除映射的所有规则（新增）

## 测试验证

### 测试场景
1. ✅ 创建启用 masquerade 的映射
2. ✅ 更新映射配置
3. ✅ 禁用映射（规则被删除）
4. ✅ 启用映射（规则被添加）
5. ✅ 删除映射（规则被清理）
6. ✅ 快速连续操作（无并发冲突）
7. ✅ 手动全量同步（修复不一致状态）

### 验证方法
```bash
# 查看 MikroTik 规则数量变化
journalctl -u singbox-manager --since "1 minute ago" | grep "Retrieved.*NAT"

# 查看单次操作日志
journalctl -u singbox-manager --since "30 seconds ago" | grep "mapping 1"

# 验证规则返回值
journalctl -u singbox-manager --since "30 seconds ago" | grep "result:"
```

## 总结

通过添加互斥锁和拆分同步函数，成功解决了并发冲突问题，同时大幅提升了操作效率。现在每个操作都只影响相关的规则，不会干扰其他映射，系统更加稳定可靠。
