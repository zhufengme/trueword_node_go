# policy create - 创建策略组

## 概述

`policy create` 命令用于创建策略路由组，指定一组 CIDR 地址的流量通过特定出口接口转发。

## 语法

```bash
# 自动分配优先级
sudo twnode policy create <策略组名> <出口接口>

# 手动指定优先级
sudo twnode policy create <策略组名> <出口接口> --priority <优先级>

# 带源地址限制
sudo twnode policy create <策略组名> <出口接口> --from <源地址>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<策略组名>` | 策略组唯一标识符 | 是 |
| `<出口接口>` | 流量转发的目标接口（物理接口或隧道） | 是 |
| `--priority` | 路由规则优先级（100-899） | 否 |
| `--from` | 源地址限制（CIDR 格式） | 否 |

## 优先级分配

### 自动分配（默认）

从 100 开始，每次递增 100：

```
第一个策略组: 优先级 100
第二个策略组: 优先级 200
第三个策略组: 优先级 300
...
```

### 手动指定

使用 `--priority` 参数指定 100-899 之间的值：

```bash
sudo twnode policy create high_priority tunnel_hk --priority 100
sudo twnode policy create low_priority tunnel_us --priority 800
```

### 冲突检测

创建时会自动检查优先级冲突：

```bash
$ sudo twnode policy create group2 tunnel_us --priority 150

❌ 错误: 优先级 150 已被策略组 group1 使用
请选择其他优先级或删除冲突的策略组。
```

## 示例

### 示例1: 创建基本策略组（自动优先级）

```bash
$ sudo twnode policy create vpn_traffic tunnel_hk

【创建策略组】
名称: vpn_traffic
出口接口: tunnel_hk
优先级: 100 (自动分配)

✓ 策略组已创建
✓ 配置已保存到 /etc/trueword_node/policies/vpn_traffic.json

下一步:
  1. 添加 CIDR: twnode policy add-cidr vpn_traffic <CIDR>
  2. 应用策略: twnode policy apply vpn_traffic
```

### 示例2: 指定优先级

```bash
$ sudo twnode policy create high_priority tunnel_hk --priority 150

【创建策略组】
名称: high_priority
出口接口: tunnel_hk
优先级: 150 (手动指定)

✓ 策略组已创建
✓ 配置已保存到 /etc/trueword_node/policies/high_priority.json
```

### 示例3: 带源地址限制

```bash
$ sudo twnode policy create office_vpn tunnel_ab --from 10.0.0.0/8

【创建策略组】
名称: office_vpn
出口接口: tunnel_ab
优先级: 200 (自动分配)
源地址: 10.0.0.0/8

✓ 策略组已创建
✓ 配置已保存到 /etc/trueword_node/policies/office_vpn.json

提示: 仅来自 10.0.0.0/8 的流量会匹配此策略。
```

### 示例4: 出口接口不存在

```bash
$ sudo twnode policy create test_group tunnel_notexist

❌ 错误: 出口接口不存在: tunnel_notexist

请先创建隧道或检查接口名称。
可用接口:
  - eth0 (物理接口)
  - tunnel_hk (WireGuard 隧道)
  - tunnel_us (WireGuard 隧道)
```

## 配置文件

策略组配置保存在 `/etc/trueword_node/policies/<策略组名>.json`：

```json
{
  "name": "vpn_traffic",
  "exit_interface": "tunnel_hk",
  "priority": 100,
  "from_source": "",
  "cidrs": [],
  "cost": 0
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 策略组名称 |
| `exit_interface` | string | 出口接口名称 |
| `priority` | int | 路由规则优先级（100-899） |
| `from_source` | string | 源地址限制（可选） |
| `cidrs` | array | CIDR 列表（初始为空） |
| `cost` | int | 成本值（用于故障转移评分） |

## 优先级规则

### 系统优先级分配

```
5          临时测试路由（check/failover）
10         保护路由（保护隧道底层连接）
100-899    用户策略组（可自定义）
900        默认路由（可选的兜底路由）
32766      主路由表
32767      系统默认路由表
```

### 优先级越小越优先

```
优先级 100 > 优先级 200 > 优先级 300 > ...
```

### 最佳实践

- **高优先级（100-300）**: 特定、重要的流量
- **中优先级（400-600）**: 一般业务流量
- **低优先级（700-899）**: 备用或次要流量

## 成本（Cost）字段

用于故障转移时的评分惩罚：

```
最终评分 = 基础评分 - Cost × 0.5
```

创建时 Cost 默认为 0，可通过编辑配置文件修改：

```json
{
  "name": "backup_route",
  "exit_interface": "tunnel_backup",
  "priority": 800,
  "from_source": "",
  "cidrs": ["192.168.100.0/24"],
  "cost": 10
}
```

这样，即使 `tunnel_backup` 连通性良好，由于成本惩罚，故障转移时会优先选择其他接口。

## 源地址限制用法

### 使用场景

限制策略仅对特定源地址生效：

```bash
# 仅 10.0.0.0/8 的流量走 VPN
sudo twnode policy create internal_vpn tunnel_vpn --from 10.0.0.0/8

# 添加目标 CIDR
sudo twnode policy add-cidr internal_vpn 192.168.0.0/16
```

路由规则：

```bash
# 生成的规则
ip rule add from 10.0.0.0/8 to 192.168.0.0/16 lookup 50 pref <priority>
```

### 不限制源地址

不指定 `--from` 参数，策略对所有源地址生效：

```bash
sudo twnode policy create all_vpn tunnel_vpn
sudo twnode policy add-cidr all_vpn 192.168.0.0/16
```

路由规则：

```bash
# 生成的规则
ip rule add to 192.168.0.0/16 lookup 50 pref <priority>
```

## 常见问题

### Q: 创建后需要立即应用吗？

A: 不需要。可以先添加 CIDR，再统一应用：

```bash
sudo twnode policy create group1 tunnel_hk
sudo twnode policy add-cidr group1 192.168.100.0/24
sudo twnode policy add-cidr group1 192.168.101.0/24
sudo twnode policy apply group1
```

### Q: 可以修改已创建策略组的优先级吗？

A: 可以，使用 `set-priority` 命令：

```bash
sudo twnode policy set-priority group1 150
```

### Q: 策略组名称有限制吗？

A: 建议使用字母、数字、下划线、连字符，避免特殊字符和空格。

### Q: 可以使用相同的出口接口创建多个策略组吗？

A: 可以。多个策略组可以使用同一出口，只要优先级不冲突。

### Q: 创建策略组会立即影响网络吗？

A: 不会。创建策略组只是保存配置，需要执行 `apply` 才会生效。

## 下一步

- [添加 CIDR](add-cidr.md) - 向策略组添加路由规则
- [应用策略](apply.md) - 使策略生效
- [列出策略组](list.md) - 查看所有策略组

---

**导航**: [← policy 命令](index.md) | [返回首页](../../index.md) | [add-cidr →](add-cidr.md)
