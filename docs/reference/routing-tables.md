# 路由表设计

本文档详细说明 TrueWord Node 的路由表架构、优先级系统和路由决策过程。

## 路由表概述

TrueWord Node 使用 Linux 的多路由表机制实现灵活的策略路由。

### 系统路由表

| 表编号 | 表名称 | 用途 | 优先级 |
|-------|--------|------|--------|
| 5 | 临时测试表 | check/failover 测试 | 5 |
| 50 | 策略路由表 | 用户策略路由 | 10-899 |
| 80 | 虚拟 IP 表 | 隧道虚拟 IP 路由 | 80 |
| 254 | 主路由表 | 物理接口路由 | 32766 |
| 253 | 默认路由表 | 系统默认路由 | 32767 |

## 优先级系统

### 优先级范围

```
5          临时测试路由（最高优先级）
10         保护路由
100-899    用户策略组
900        默认路由（可选）
32766      主路由表
32767      系统默认路由表
```

**规则**: 数字越小，优先级越高。

### 优先级分配

#### 系统保留优先级

- **5**: 临时测试路由
  - 用途: `line check` 和 `policy failover` 期间使用
  - 生命周期: 测试完成后立即清理（约 4 秒）
  - 确保测试流量不受用户策略干扰

- **10**: 保护路由
  - 用途: 保护隧道底层连接
  - 规则: `ip rule add to <对端IP> lookup main pref 10`
  - 防止路由环路

- **900**: 默认路由（可选）
  - 用途: 兜底路由（0.0.0.0/0）
  - 匹配所有未被其他策略匹配的流量

#### 用户策略组优先级

- **范围**: 100-899
- **自动分配**: 100, 200, 300, ...（递增 100）
- **手动指定**: 任意 100-899 之间的值
- **冲突检测**: 创建时自动检查冲突

### 优先级使用示例

```
5      to 8.8.8.8 lookup 5 pref 5                    # 临时测试
10     to 203.0.113.50 lookup main pref 10           # 保护路由
10     to 103.118.40.121 lookup main pref 10         # 保护路由
100    to 192.168.100.0/24 lookup 50 pref 100        # 高优先级策略组
200    to 10.0.0.0/8 lookup 50 pref 200              # 中优先级策略组
800    to 172.16.0.0/12 lookup 50 pref 800           # 低优先级策略组
900    from all lookup 50 pref 900                    # 默认路由
32766  from all lookup main                           # 主路由表
32767  from all lookup default                        # 系统默认路由表
```

## 路由决策流程

### 流量匹配过程

```
1. 首先匹配临时测试路由（pref 5）
   └─ 仅测试期间存在

2. 然后匹配保护路由（pref 10）
   └─ 隧道对端 IP 走主路由表

3. 接着按优先级匹配用户策略组（pref 100-899）
   └─ 策略组中的 CIDR 规则

4. 匹配虚拟 IP 表（pref 80）
   └─ 隧道虚拟 IP 路由

5. 匹配默认路由（pref 900，如果设置）
   └─ 0.0.0.0/0 兜底路由

6. 最后使用主路由表（pref 32766）
   └─ 物理接口的系统路由
```

### 匹配示例

**场景**: 访问 `192.168.100.5`

```
检查规则优先级 5  → 不匹配（仅测试 IP）
检查规则优先级 10 → 不匹配（仅隧道对端 IP）
检查规则优先级 100 → 匹配！to 192.168.100.0/24 lookup 50
  ↓
查询表 50 → 找到路由: 192.168.100.0/24 dev tunnel_hk
  ↓
流量通过 tunnel_hk 发送
```

**场景**: 访问 `1.1.1.1`（未匹配任何策略）

```
检查规则优先级 5  → 不匹配
检查规则优先级 10 → 不匹配
检查规则优先级 100-899 → 不匹配（不在任何策略组）
检查规则优先级 900 → 匹配！default route lookup 50
  ↓
查询表 50 → 找到默认路由: default via tunnel_hk
  ↓
流量通过 tunnel_hk 发送
```

**场景**: 未设置默认路由

```
检查规则优先级 5  → 不匹配
检查规则优先级 10 → 不匹配
检查规则优先级 100-899 → 不匹配
检查规则优先级 900 → 不存在
检查规则优先级 32766 → 匹配！main table
  ↓
查询主路由表 → 找到系统默认路由
  ↓
流量通过物理接口（如 eth0）发送
```

## 表 50（策略路由表）

### 表内容

表 50 包含所有策略路由规则：

```bash
# 查看表 50
ip route show table 50

# 输出示例:
192.168.100.0/24 dev tunnel_hk scope link
192.168.101.0/24 dev tunnel_hk scope link
10.0.0.0/8 dev tunnel_us scope link
default dev tunnel_hk scope link  # 默认路由（如果设置）
```

### 规则添加

```bash
# 添加路由到表 50
ip route add 192.168.100.0/24 dev tunnel_hk table 50

# 添加规则指向表 50
ip rule add to 192.168.100.0/24 lookup 50 pref 100
```

### 物理接口路由

物理接口作为出口时，需要指定网关：

```bash
# 隧道接口（直连）
ip route add 192.168.100.0/24 dev tunnel_hk table 50

# 物理接口（通过网关）
ip route add 192.168.100.0/24 via 192.168.1.1 dev eth0 table 50
```

## 表 80（虚拟 IP 表）

### 用途

专门用于隧道虚拟 IP 的路由。

### 规则示例

```bash
# 虚拟 IP 路由规则
ip rule add to 10.0.0.2/32 lookup 80 pref 80

# 表 80 内容
ip route show table 80
# 通常为空，虚拟 IP 通过隧道接口直接路由
```

### 为什么需要表 80？

分离虚拟 IP 路由和策略路由，避免冲突：

- **虚拟 IP**: 总是通过隧道接口
- **策略路由**: 可能通过不同出口

## 临时测试表（表 5）

### 用途

`line check` 和 `policy failover` 期间临时使用。

### 工作流程

```go
// 1. 添加临时路由
exec.Command("ip", "rule", "add", "to", checkIP, "lookup", "5", "pref", "5").Run()
exec.Command("ip", "route", "add", checkIP, "dev", tunnelInterface, "table", "5").Run()

// 2. 使用 defer 确保清理
defer func() {
    exec.Command("ip", "route", "del", checkIP, "table", "5").Run()
    exec.Command("ip", "rule", "del", "to", checkIP, "pref", "5").Run()
}()

// 3. 执行测试
ping -c 20 -I tunnelInterface checkIP
```

### 优势

- ✅ 最高优先级，确保测试流量不受策略干扰
- ✅ 临时存在，不影响系统路由
- ✅ 自动清理，无残留

## 保护路由机制

### 为什么需要保护路由？

防止路由环路：

```
问题场景:
1. 隧道 tunnel_hk 使用 eth0 连接到 203.0.113.50
2. 策略路由设置所有流量走 tunnel_hk
3. 隧道握手包目标是 203.0.113.50
4. 握手包也会匹配策略路由，进入 tunnel_hk
5. 但握手包需要通过 eth0 发送！
6. 结果：路由环路，隧道无法建立
```

### 解决方案

优先级 10 的保护路由：

```bash
ip rule add to 203.0.113.50 lookup main pref 10
```

这样，发往 `203.0.113.50` 的流量会优先匹配保护路由，通过主路由表（即 eth0）发送，而不会进入隧道。

### 动态更新

通过 `policy sync-protection` 自动检测对端 IP 变化并更新保护路由。

详见 [保护路由机制](protection-routes.md)

## 源地址过滤

### 基本用法

限制策略仅对特定源地址生效：

```bash
# 仅来自 10.0.0.0/8 的流量匹配
ip rule add from 10.0.0.0/8 to 192.168.100.0/24 lookup 50 pref 100
```

### 组合规则

```bash
# 规则1: 来自 10.0.0.0/8，去往 192.168.100.0/24
ip rule add from 10.0.0.0/8 to 192.168.100.0/24 lookup 50 pref 100

# 规则2: 所有源地址，去往 192.168.101.0/24
ip rule add to 192.168.101.0/24 lookup 50 pref 100
```

## 常见问题

### Q: 如何查看当前所有路由规则？

A:
```bash
ip rule show
```

### Q: 如何查看特定表的路由？

A:
```bash
ip route show table <表编号>
# 例如:
ip route show table 50
ip route show table 80
ip route show table main
```

### Q: 如何测试路由决策？

A:
```bash
# 测试目标地址会走哪个路由
ip route get 192.168.100.5

# 指定源地址测试
ip route get 192.168.100.5 from 10.0.0.1
```

### Q: 优先级冲突怎么办？

A: 使用 `policy set-priority` 调整：

```bash
sudo twnode policy set-priority group1 150
```

### Q: 如何清理所有策略路由？

A:
```bash
# 使用 revoke 命令
sudo twnode policy revoke

# 或手动清理
ip rule flush pref 100-899
ip route flush table 50
```

## 下一步

- [保护路由机制](protection-routes.md) - 深入了解保护路由
- [配置文件详解](config-files.md) - 配置文件格式
- [故障排查](troubleshooting.md) - 常见问题解决

---

**导航**: [← 参考资料](../index.md#参考资料) | [返回首页](../index.md) | [保护路由 →](protection-routes.md)
