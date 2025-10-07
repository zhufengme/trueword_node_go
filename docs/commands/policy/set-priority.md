# policy set-priority - 调整策略组优先级

## 概述

`policy set-priority` 命令调整已创建策略组的优先级。如果策略组已应用，会自动重新应用以使新优先级生效。

## 语法

```bash
sudo twnode policy set-priority <策略组名> <新优先级>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<策略组名>` | 策略组名称 | 是 |
| `<新优先级>` | 新的优先级值（100-899） | 是 |

## 优先级范围

| 优先级 | 用途 | 说明 |
|--------|------|------|
| **5** | 临时测试路由 | check/failover 使用，执行后自动清理 |
| **10** | 保护路由 | 保护隧道底层连接，防止路由环路 |
| **100-899** | **用户策略组** | 可手动指定或自动分配 |
| **900** | 默认路由 | 0.0.0.0/0 兜底路由 |
| **32766** | 主路由表 | Linux 系统主路由表 |
| **32767** | 默认路由表 | Linux 系统默认路由表 |

## 示例

### 示例1: 调整未应用策略组的优先级

```bash
$ sudo twnode policy set-priority vpn_traffic 150

✓ 策略组 vpn_traffic 优先级已更新: 100 → 150
✓ 配置已保存

提示: 使用 'twnode policy apply vpn_traffic' 应用更改
```

### 示例2: 调整已应用策略组的优先级

```bash
$ sudo twnode policy set-priority asia_traffic 120

✓ 策略组 asia_traffic 优先级已更新: 100 → 120
⚠️ 策略组已应用，正在重新应用...

撤销旧规则（优先级 100）...
✓ 已撤销 3 条路由规则

应用新规则（优先级 120）...
✓ 192.168.100.0/24 → tunnel_hk (pref 120)
✓ 192.168.101.0/24 → tunnel_hk (pref 120)
✓ 203.0.113.0/24 → tunnel_hk (pref 120)

✓ 策略组 asia_traffic 已重新应用
```

### 示例3: 优先级冲突检测

```bash
$ sudo twnode policy set-priority backup_route 200

❌ 错误: 优先级 200 已被策略组 us_traffic 使用
请选择其他优先级或先修改策略组 us_traffic 的优先级
```

### 示例4: 优先级超出范围

```bash
$ sudo twnode policy set-priority vpn_traffic 50

❌ 错误: 优先级 50 超出允许范围
用户策略组优先级范围: 100-899

$ sudo twnode policy set-priority vpn_traffic 1000

❌ 错误: 优先级 1000 超出允许范围
用户策略组优先级范围: 100-899
```

## 优先级冲突检查

系统自动检查优先级冲突：

```bash
# 查看当前所有策略组的优先级
sudo twnode policy list

+---------------+-------------+----------+-------------+-----------+
| 策略组名称    | 出口接口    | 优先级   | CIDR 数量   | 状态      |
+---------------+-------------+----------+-------------+-----------+
| asia_traffic  | tunnel_hk   | 100      | 3           | Applied   |
| us_traffic    | tunnel_us   | 200      | 2           | Applied   |
| backup_route  | tunnel_bak  | 800      | 1           | Revoked   |
+---------------+-------------+----------+-------------+-----------+

# 尝试使用已存在的优先级会被拒绝
sudo twnode policy set-priority backup_route 100  # 失败：100 已被 asia_traffic 使用
```

## 批量调整优先级

### 场景：插入新策略组

假设当前有：
- asia_traffic: 100
- us_traffic: 200
- backup_route: 800

现在要插入优先级 150 的新策略组，需要调整现有策略：

```bash
# 1. 调整 us_traffic 为 250（避开 200）
sudo twnode policy set-priority us_traffic 250

# 2. 创建新策略组，优先级 150
sudo twnode policy create new_traffic tunnel_new --priority 150

# 3. 如需要，调整其他策略组
sudo twnode policy set-priority backup_route 900
```

## 优先级策略建议

### 按业务划分

```
100-199: 核心业务流量（最高优先）
200-399: 普通业务流量
400-599: 备份和测试流量
600-799: 低优先级流量
800-899: 保留/备用
```

### 按地区划分

```
100: 亚洲流量
200: 美洲流量
300: 欧洲流量
400: 其他地区
```

### 按隧道类型划分

```
100: WireGuard 隧道
200: GRE 隧道
300: 备份隧道
```

## 验证优先级

```bash
# 查看所有路由规则
ip rule show

# 应该看到
100:    from all to 192.168.100.0/24 lookup 50
200:    from all to 192.168.200.0/24 lookup 50
...

# 查看策略组列表
sudo twnode policy list
```

## 重新应用机制

**已应用的策略组**：
1. 自动撤销旧规则（使用旧优先级）
2. 应用新规则（使用新优先级）
3. 更新配置文件

**未应用的策略组**：
1. 仅更新配置文件
2. 不执行路由操作
3. 下次 apply 时使用新优先级

## 常见问题

### Q: 修改优先级会中断网络吗？

A: 如果策略组已应用，会自动重新应用。重新应用时采用"先添加后清理"策略，确保不中断网络。

### Q: 可以将多个策略组设置为相同优先级吗？

A: 不可以。系统会检查冲突并拒绝操作。

### Q: 修改优先级后需要重启隧道吗？

A: 不需要。策略路由规则独立于隧道，修改优先级只影响路由规则。

### Q: 如何查看策略组当前优先级？

A: 使用 `policy list` 命令：

```bash
sudo twnode policy list
```

### Q: 优先级数字越小越优先吗？

A: 是的。Linux 路由规则按优先级从小到大匹配，优先级值越小越优先。

## 与 create 的区别

| 操作 | set-priority | create --priority |
|------|--------------|-------------------|
| 时机 | 策略组已存在 | 创建新策略组时 |
| 检查 | 优先级冲突 | 优先级冲突 |
| 自动应用 | 是（如已应用） | 否（需手动 apply） |

## 下一步

- [创建策略组](create.md) - 创建时指定优先级
- [应用策略](apply.md) - 使策略生效
- [列出策略组](list.md) - 查看所有策略组优先级

---

**导航**: [← delete](delete.md) | [返回首页](../../index.md) | [set-default →](set-default.md)
