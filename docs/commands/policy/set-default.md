# policy set-default - 设置默认路由

## 概述

`policy set-default` 命令设置或取消系统的默认路由（0.0.0.0/0）。默认路由作为兜底路由，捕获所有未被其他策略匹配的流量。

## 语法

```bash
# 设置默认路由
sudo twnode policy set-default <出口接口>

# 取消默认路由
sudo twnode policy set-default --remove
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<出口接口>` | 默认路由的出口接口 | 是（设置时） |
| `--remove` | 取消默认路由 | 否 |

## 默认路由优先级

默认路由固定使用**优先级 900**，确保它是最后匹配的规则：

```
优先级范围说明:
  5: 临时测试路由
  10: 保护路由
  100-899: 用户策略组
  900: 默认路由 ← 兜底路由
  32766: 主路由表
  32767: 系统默认路由表
```

## 示例

### 示例1: 设置默认路由

```bash
$ sudo twnode policy set-default tunnel_hk

【设置默认路由】
出口接口: tunnel_hk
优先级: 900
CIDR: 0.0.0.0/0

检查接口状态...
✓ 接口 tunnel_hk 存在且已启动

添加保护路由...
✓ 已添加保护路由（优先级 10）

应用默认路由...
✓ 默认路由规则已添加: 0.0.0.0/0 → tunnel_hk (pref 900)

✓ 默认路由已设置
```

### 示例2: 修改默认路由

```bash
$ sudo twnode policy set-default tunnel_us

⚠️ 检测到已存在默认路由: tunnel_hk
是否替换为新的默认路由: tunnel_us? (yes/no): yes

撤销旧默认路由...
✓ 已删除旧规则: 0.0.0.0/0 → tunnel_hk (pref 900)

设置新默认路由...
✓ 默认路由规则已添加: 0.0.0.0/0 → tunnel_us (pref 900)

✓ 默认路由已更新
```

### 示例3: 取消默认路由

```bash
$ sudo twnode policy set-default --remove

【取消默认路由】
当前默认路由: tunnel_hk
优先级: 900

确认取消？(yes/no): yes

✓ 已删除默认路由规则
✓ 流量将回退到主路由表

✓ 默认路由已取消
```

### 示例4: 接口不存在

```bash
$ sudo twnode policy set-default tunnel_notexist

❌ 错误: 接口不存在: tunnel_notexist

可用接口列表:
- eth0 (192.168.1.100)
- tunnel_hk (10.0.0.1)
- tunnel_us (10.0.1.1)
```

## 工作流程

```
设置默认路由:

1. 检查接口存在性
   └─ ip link show <接口名>

2. 检查接口状态
   └─ 接口必须处于 UP 状态

3. 添加保护路由（优先级 10）
   └─ 防止隧道底层连接被路由回隧道

4. 应用默认路由规则（优先级 900）
   └─ ip rule add to 0.0.0.0/0 lookup 50 pref 900
   └─ ip route add 0.0.0.0/0 dev <接口> table 50

5. 保存到配置文件
   └─ /etc/trueword_node/config.yaml
```

## 验证默认路由

### 检查路由规则

```bash
# 查看所有规则
ip rule show

# 应该看到优先级 900 的规则
900:    from all to 0.0.0.0/0 lookup 50
```

### 检查路由表 50

```bash
ip route show table 50

# 应该看到默认路由
default dev tunnel_hk scope link
```

### 测试默认路由

```bash
# 测试未被策略匹配的流量
ip route get 8.8.8.8

# 应该显示走默认路由的接口
8.8.8.8 dev tunnel_hk src 10.0.0.1
```

## 使用场景

### 场景1: 全局 VPN

所有流量默认走 VPN 隧道：

```bash
# 设置 WireGuard 隧道为默认出口
sudo twnode policy set-default tunnel_vpn

# 特定流量走物理接口（策略组优先级更高）
sudo twnode policy create local_traffic eth0 --priority 100
sudo twnode policy add-cidr local_traffic 192.168.0.0/16
sudo twnode policy apply local_traffic
```

### 场景2: 智能分流

核心流量走专用隧道，其他流量走备用隧道：

```bash
# 核心业务走香港隧道（优先级 100）
sudo twnode policy create core_traffic tunnel_hk --priority 100
sudo twnode policy add-cidr core_traffic 203.0.113.0/24
sudo twnode policy apply core_traffic

# 其他流量走美国隧道（默认路由）
sudo twnode policy set-default tunnel_us
```

### 场景3: 故障转移

主隧道故障时，自动切换默认路由：

```bash
# 检查隧道连通性
sudo twnode line check tunnel_hk 8.8.8.8

# 如果失败，切换默认路由
sudo twnode policy set-default tunnel_bak
```

或使用自动故障转移：

```bash
sudo twnode policy failover --default tunnel_hk,tunnel_us,tunnel_bak
```

## 配置文件

默认路由保存在全局配置文件：

```yaml
# /etc/trueword_node/config.yaml
default_route:
  exit_interface: tunnel_hk
  priority: 900
  enabled: true
```

## 与策略组的交互

### 优先级顺序

```
数字越小 = 优先级越高

10: 保护路由（最高，保护隧道底层连接）
  ↓
100-899: 用户策略组（按优先级匹配）
  ↓
900: 默认路由（最低，兜底路由）
  ↓
主路由表（未被任何策略匹配的流量）
```

### 示例流量分发

假设配置：
- 优先级 100: 192.168.100.0/24 → tunnel_hk
- 优先级 200: 10.0.0.0/8 → tunnel_us
- 优先级 900: 0.0.0.0/0 → tunnel_bak

流量走向：
```
192.168.100.5 → 匹配优先级 100 → tunnel_hk
10.0.5.10 → 匹配优先级 200 → tunnel_us
8.8.8.8 → 匹配优先级 900（默认路由） → tunnel_bak
```

## 安全考虑

### 保护路由的重要性

如果默认路由的出口接口是隧道，**必须**添加保护路由，否则会导致路由环路：

```
错误场景（无保护路由）:
1. 隧道握手包需要发送到对端IP（如 1.2.3.4）
2. 默认路由导致握手包走隧道
3. 隧道未建立，无法传输握手包
4. 死锁！

正确场景（有保护路由）:
1. 隧道握手包需要发送到对端IP（1.2.3.4）
2. 保护路由（优先级 10）匹配，走主路由表
3. 握手包通过物理接口发送
4. 隧道成功建立
```

**系统自动添加保护路由**，无需手动配置。

### 验证保护路由

```bash
# 查看优先级 10 的规则
ip rule show pref 10

# 应该看到对端IP的保护路由
10:     from all to 1.2.3.4 lookup main
```

## 常见问题

### Q: 设置默认路由会覆盖系统默认路由吗？

A: 不会。系统默认路由（主路由表）仍然存在，但优先级更低（32766）。策略路由（优先级 900）会优先匹配。

### Q: 默认路由和策略组有什么区别？

A:
- 策略组：匹配特定 CIDR
- 默认路由：匹配所有流量（0.0.0.0/0），作为兜底

### Q: 可以修改默认路由的优先级吗？

A: 不可以。默认路由固定使用优先级 900，确保它是最后匹配的策略。

### Q: 如何查看当前默认路由？

A: 使用多种方式：

```bash
# 方法1: 查看配置文件
cat /etc/trueword_node/config.yaml

# 方法2: 查看路由规则
ip rule show pref 900

# 方法3: 查看路由表 50
ip route show table 50 | grep default
```

### Q: 默认路由会影响策略组吗？

A: 不会。策略组优先级（100-899）高于默认路由（900），策略组会优先匹配。

### Q: 取消默认路由后流量走哪里？

A: 走主路由表的默认路由，通常是物理接口的默认网关。

## 故障排除

### 默认路由不生效

```bash
# 1. 检查规则是否存在
ip rule show pref 900

# 2. 检查路由表 50
ip route show table 50

# 3. 检查接口状态
ip link show <接口名>

# 4. 测试路由
ip route get 8.8.8.8

# 5. 重新应用
sudo twnode policy set-default <接口名>
```

### 路由环路

```bash
# 检查保护路由是否存在
ip rule show pref 10

# 如果缺失，执行同步
sudo twnode policy sync-protection

# 或重新设置默认路由（会自动添加保护路由）
sudo twnode policy set-default <接口名>
```

## 下一步

- [创建策略组](create.md) - 创建特定流量的策略
- [故障转移](failover.md) - 自动切换默认路由
- [保护路由](../../reference/protection-routes.md) - 深入理解保护路由机制

---

**导航**: [← set-priority](set-priority.md) | [返回首页](../../index.md) | [policy 首页](index.md)
