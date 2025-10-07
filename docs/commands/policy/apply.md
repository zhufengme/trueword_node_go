# policy apply - 应用策略

## 概述

`policy apply` 命令将策略组的路由规则应用到系统内核。支持应用单个策略组或所有策略组。

## 语法

```bash
# 应用单个策略组
sudo twnode policy apply <策略组名>

# 应用所有策略组
sudo twnode policy apply
```

## 工作流程

```
1. 同步保护路由（自动执行）
   └─ policy sync-protection

2. 检查出口接口状态
   ├─ 检查接口是否存在
   └─ 检查接口是否 UP

3. 应用路由规则（每个 CIDR）
   ├─ 先添加新规则
   ├─ 检测并清理重复规则
   └─ 验证规则存在

4. 添加保护路由（如果需要）
   └─ 确保隧道底层连接不走策略路由
```

## 示例

### 示例1: 应用单个策略组

```bash
$ sudo twnode policy apply vpn_traffic

同步保护路由...
✓ 保护路由同步完成

应用策略组: vpn_traffic

【策略组信息】
名称: vpn_traffic
出口接口: tunnel_hk
优先级: 100
CIDR 数量: 2

检查出口接口...
✓ 接口 tunnel_hk 存在
✓ 接口 tunnel_hk 已启动

应用路由规则...
✓ 192.168.100.0/24 → tunnel_hk (pref 100)
✓ 192.168.101.0/24 → tunnel_hk (pref 100)

✓ 策略组 vpn_traffic 已应用
```

### 示例2: 应用所有策略组

```bash
$ sudo twnode policy apply

同步保护路由...
✓ 保护路由同步完成

应用所有策略组...

【策略组 1: asia_traffic】
优先级: 100
出口: tunnel_hk
✓ 已应用 3 个 CIDR

【策略组 2: us_traffic】
优先级: 200
出口: tunnel_us
✓ 已应用 2 个 CIDR

【策略组 3: backup_route】
优先级: 800
出口: tunnel_backup
✓ 已应用 1 个 CIDR

共应用 3 个策略组，6 条路由规则
✓ 所有策略组已应用
```

### 示例3: 出口接口未启动

```bash
$ sudo twnode policy apply vpn_traffic

同步保护路由...
✓ 保护路由同步完成

应用策略组: vpn_traffic

检查出口接口...
✓ 接口 tunnel_hk 存在
❌ 错误: 接口 tunnel_hk 未启动

请先启动接口: sudo twnode line start tunnel_hk
```

### 示例4: 策略组无 CIDR

```bash
$ sudo twnode policy apply empty_group

同步保护路由...
✓ 保护路由同步完成

应用策略组: empty_group

⚠️ 警告: 策略组 empty_group 没有 CIDR 规则

请先添加 CIDR: sudo twnode policy add-cidr empty_group <CIDR>
```

## 无缝切换机制

### 先添加后清理

```go
// 1. 添加新规则（可能导致重复）
ip rule add to 192.168.100.0/24 lookup 50 pref 100

// 2. 循环清理重复规则
for {
    count := 规则数量(pref 100)
    if count <= 1 {
        break
    }
    ip rule del pref 100
}

// 3. 验证规则存在
if 规则数量(pref 100) == 0 {
    // 被意外删除，重新添加
    ip rule add to 192.168.100.0/24 lookup 50 pref 100
}
```

**优势**:
- ✅ 始终至少有一个规则存在
- ✅ 不会中断网络
- ✅ 有验证和恢复机制

## 验证应用结果

### 检查路由规则

```bash
# 查看所有规则
ip rule show

# 查看特定优先级的规则
ip rule show pref 100

# 应该显示:
# 100: from all to 192.168.100.0/24 lookup 50
# 100: from all to 192.168.101.0/24 lookup 50
```

### 查看路由表 50

```bash
ip route show table 50

# 应该显示出口接口的路由
# 192.168.100.0/24 dev tunnel_hk scope link
# 192.168.101.0/24 dev tunnel_hk scope link
```

### 测试路由

```bash
# 测试流量是否走指定接口
ip route get 192.168.100.5

# 应该显示:
# 192.168.100.5 dev tunnel_hk src 10.0.0.1
```

### 实际连通性测试

```bash
# Ping 目标地址
ping -c 3 192.168.100.5

# 或使用 traceroute
traceroute 192.168.100.5
```

## 重复应用

可以多次应用同一策略组，系统会自动清理重复规则：

```bash
# 第一次应用
sudo twnode policy apply vpn_traffic

# 修改配置后重新应用
sudo twnode policy add-cidr vpn_traffic 192.168.102.0/24
sudo twnode policy apply vpn_traffic

# 系统会自动清理旧规则并添加新规则
```

## 批量应用

### 应用所有策略组

```bash
sudo twnode policy apply
```

### 选择性应用

```bash
# 应用多个特定策略组
for group in asia_traffic us_traffic backup_route; do
    sudo twnode policy apply $group
done
```

## 常见问题

### Q: apply 和 create 的区别？

A:
- `create` - 创建策略组配置（保存到文件）
- `apply` - 将配置应用到系统内核（生效）

### Q: 修改配置后需要重新 apply 吗？

A: 是的。修改 CIDR、出口接口等配置后，需要重新 apply：

```bash
sudo twnode policy add-cidr group1 192.168.100.0/24
sudo twnode policy apply group1
```

### Q: apply 会影响正在运行的网络吗？

A: 影响极小。采用"先添加后清理"机制，确保始终有路由规则存在。

### Q: apply 失败后如何恢复？

A: 使用 `revoke` 命令撤销策略：

```bash
sudo twnode policy revoke vpn_traffic
```

或重新 apply：

```bash
sudo twnode policy apply vpn_traffic
```

### Q: 可以在 cron 中定时 apply 吗？

A: 可以，但通常不需要。除非配置经常变化：

```bash
# 每小时重新应用所有策略
0 * * * * /usr/local/bin/twnode policy apply
```

## 下一步

- [撤销策略](revoke.md) - 撤销已应用的策略
- [列出策略组](list.md) - 查看策略组状态
- [故障转移](failover.md) - 自动切换出口

---

**导航**: [← create](create.md) | [返回首页](../../index.md) | [revoke →](revoke.md)
