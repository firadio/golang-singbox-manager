# Mikrotik RouterOS API 实现踩坑记录

## 概述

本文档记录在实现 Mikrotik RouterOS API 客户端过程中遇到的问题和解决方案，供后续维护参考。

## 1. 认证方式问题

### 问题描述
RouterOS 在 6.43 版本后改变了 API 认证方式，导致使用旧版 MD5 challenge-response 认证失败。

### 错误表现
```
failed to login: invalid user name or password (6)
```

### 根本原因
- **RouterOS < 6.43**: 使用 MD5 challenge-response 认证
  - 服务器发送 hex 编码的 challenge
  - 客户端计算: `MD5(0x00 + password + decode_hex(challenge))`
  - 发送: `00` + `hex(hash)`

- **RouterOS >= 6.43**: 使用明文密码认证
  - 直接发送 `name` 和 `password` 参数

### 解决方案
实现双版本支持，先尝试新版认证，失败时回退到旧版：

```go
// 先尝试新版认证
attrs := map[string]string{
    "name":     username,
    "password": password,
}
c.writeCommand("/login", attrs)

// 如果返回 challenge，使用旧版认证
if challengeReceived {
    return c.loginOldStyle(challenge)
}
```

**关键点**：Challenge 是 hex 编码的字符串，必须先 `hex.DecodeString()` 后再参与 MD5 计算。

---

## 2. Challenge 解码问题

### 问题描述
使用正确的密码，但一直提示密码错误。

### 错误代码
```go
// ❌ 错误：直接使用 hex 字符串
h.Write([]byte(challenge))

// ✅ 正确：先解码再使用
challengeBytes, _ := hex.DecodeString(challenge)
h.Write(challengeBytes)
```

### 解决方案
Mikrotik 返回的 challenge 是 hex 编码的字符串（如 `"7a9d6cabd7d62bbb..."`），必须解码为字节数组后才能用于 MD5 计算。

**文件位置**: `internal/mikrotik/client.go:98-108`

---

## 3. 查询参数语法问题

### 问题描述
尝试使用查询参数过滤规则时，一直返回 `unknown parameter ?comment` 错误。

### 尝试过的错误方案

#### 方案 1: 使用 `?key=value` 格式
```go
// ❌ 不支持
items, _ := c.runCommand("/routing/rule/print", map[string]string{
    "?comment": "sing-box-rule-id-1",
})
// 返回: unknown parameter ?comment
```

#### 方案 2: 使用 `?=key=value` 格式
```go
// ❌ 不支持
items, _ := c.runCommand("/routing/rule/print", map[string]string{
    "?=comment": "sing-box-rule-id-1",
})
// 返回: unknown parameter ?comment
```

### 根本原因
Mikrotik API 的查询语法与预期不同，或者该版本不支持直接在 API 中使用查询参数。

### 最终解决方案
**放弃使用查询参数，改为获取所有规则后在本地过滤：**

```go
// ✅ 正确方案
// 1. 获取所有规则
items, _ := c.runCommand("/routing/rule/print", nil)

// 2. 在本地遍历查找
for _, item := range items {
    if itemComment, ok := item["comment"]; ok && itemComment == targetComment {
        // 找到匹配的规则
    }
}
```

**优点**:
- 避免了查询语法兼容性问题
- 代码更简单可靠
- 适用于所有 RouterOS 版本

**缺点**:
- 规则数量大时性能略差（但对于实际使用场景可以接受）

**文件位置**: `internal/mikrotik/client.go:405-418, 447-463`

---

## 4. 错误响应处理问题

### 问题描述
当 API 调用出错时，Mikrotik 会返回包含 `message` 字段的错误响应，但这个响应会被误当作正常数据。

### 错误表现
```go
items, _ := c.runCommand("/routing/rule/print", badParams)
// items = [{"message": "unknown parameter xxx"}]
// len(items) = 1, 误判为有1条规则
```

### 解决方案
在处理返回结果时，过滤掉包含 `message` 字段的错误响应：

```go
// 过滤错误消息，只处理真正的规则
var validItems []map[string]string
for _, item := range items {
    if _, hasMsg := item["message"]; !hasMsg {
        validItems = append(validItems, item)
    }
}
```

---

## 5. 删除规则的正确方式

### 问题描述
最初尝试使用 `[find ...]` 语法一步删除规则失败。

### 错误方案
```go
// ❌ API 不支持这种语法
findExpr := fmt.Sprintf("[find comment=\"%s\"]", comment)
c.runCommand("/routing/rule/remove", map[string]string{
    "numbers": findExpr,
})
```

### 正确方案
分两步操作：先查找获取 ID，再删除：

```go
// 1. 获取所有规则
items, _ := c.runCommand("/routing/rule/print", nil)

// 2. 查找匹配的规则
for _, item := range items {
    if item["comment"] == targetComment {
        id := item[".id"]

        // 3. 使用 .id 删除
        c.runCommand("/routing/rule/remove", map[string]string{
            ".id": id,
        })
    }
}
```

**关键点**: 删除操作必须使用 `.id` 参数，而不能使用 find 表达式。

---

## 6. 规则添加的命令路径

### 问题描述
RouterOS 7.x 改变了命令路径结构。

### 正确的命令路径
- **旧版本**: `/ip/route/rule/add`
- **新版本**: `/routing/rule/add`

建议使用新版路径，兼容性更好。

**文件位置**: `internal/mikrotik/client.go:413-426`

---

## 总结

### 核心经验教训

1. **认证方式**: 优先使用新版明文认证，保留旧版 MD5 认证作为后备
2. **数据解码**: Hex 编码的数据必须先解码再使用
3. **查询过滤**: 避免使用 API 查询参数，改用本地过滤
4. **错误处理**: 始终检查并过滤包含 `message` 字段的错误响应
5. **删除操作**: 必须使用 `.id` 参数，不能使用 find 表达式
6. **命令路径**: 使用新版本命令路径 `/routing/...`

### 调试建议

遇到 Mikrotik API 问题时的调试步骤：

1. 启用调试日志查看 API 返回的原始数据
2. 检查返回数据中是否有 `message` 字段（表示错误）
3. 验证命令路径是否正确（新旧版本差异）
4. 确认参数格式符合 API 规范
5. 必要时使用 Mikrotik CLI 测试相同的操作

### 参考资料

- Mikrotik Wiki: https://wiki.mikrotik.com/wiki/Manual:API
- RouterOS API Protocol: https://help.mikrotik.com/docs/display/ROS/API

---

**最后更新**: 2025-11-09
**维护者**: AI Assistant
