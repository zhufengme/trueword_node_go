# policy 命令 - 策略路由管理

## 概述

`policy` 命令用于管理策略路由，实现基于 CIDR 的灵活路由控制、故障转移、保护路由同步等功能。

## 子命令列表

### 策略组管理

- [create](create.md) - 创建策略路由组
- [delete](delete.md) - 删除策略组
- [list](list.md) - 列出所有策略组
- [set-priority](set-priority.md) - 调整策略组优先级

### CIDR 管理

- [add-cidr](add-cidr.md) - 向策略组添加 CIDR 规则
- [remove-cidr](remove-cidr.md) - 从策略组删除 CIDR 规则

### 策略应用

- [apply](apply.md) - 应用策略路由规则
- [revoke](revoke.md) - 撤销策略路由规则

### 默认路由

- [set-default](set-default.md) - 设置默认出口路由
- [unset-default](set-default.md#unset-default) - 取消默认出口路由

### 高级功能

- [sync-protection](sync-protection.md) - 同步保护路由（动态 IP 容错）
- [failover](failover.md) - 智能故障转移

## 快速参考

### 创建和应用策略

```bash
# 1. 创建策略组（自动分配优先级）
sudo twnode policy create vpn_traffic tunnel_ab

# 2. 创建策略组（手动指定优先级）
sudo twnode policy create vpn_traffic tunnel_ab --priority 150

# 3. 添加 CIDR 规则
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24

# 4. 应用策略
sudo twnode policy apply vpn_traffic

# 5. 列出所有策略组
sudo twnode policy list

# 6. 撤销策略
sudo twnode policy revoke vpn_traffic

# 7. 删除策略组
sudo twnode policy delete vpn_traffic
```

### 默认路由

```bash
# 设置默认路由
sudo twnode policy set-default tunnel_ab

# 取消默认路由
sudo twnode policy unset-default
```

### 保护路由和故障转移

```bash
# 同步保护路由（动态 IP 容错）
sudo twnode policy sync-protection

# 故障转移（策略组）
sudo twnode policy failover vpn_traffic tunnel_ab,tunnel_cd

# 故障转移（默认路由）
sudo twnode policy failover default tunnel_ab,tunnel_cd --check-ip 8.8.8.8
```

## 核心概念

### 策略组（Policy Group）

策略组是一组 CIDR 规则的集合，指定这些 CIDR 的流量通过指定的出口接口。

**组成要素**:
- **名称** - 唯一标识符（如 `vpn_traffic`）
- **出口接口** - 流量转发的目标接口（可以是物理接口或隧道）
- **优先级** - 路由规则优先级（100-899，可选）
- **CIDR 列表** - 一组 IP 地址段
- **From 源地址** - 可选的源地址限制

**示例**:
```json
{
  "name": "vpn_traffic",
  "exit_interface": "tunnel_ab",
  "priority": 150,
  "from_source": "",
  "cidrs": [
    "192.168.100.0/24",
    "192.168.101.0/24"
  ]
}
```

### 优先级系统

策略路由使用 Linux 路由规则优先级，数字越小优先级越高。

```
5          临时测试路由（check/failover 期间）
10         保护路由（保护隧道底层连接）
100-899    用户策略组（可自动分配或手动指定）
900        默认路由（可选的兜底路由）
32766      主路由表
32767      系统默认路由表
```

**优先级分配**:
- **自动分配**: 从 100 开始，每次递增 100（100, 200, 300, ...）
- **手动指定**: 使用 `--priority` 参数指定 100-899 之间的值
- **冲突检测**: 创建时自动检查优先级冲突

详见 [路由表设计](../../reference/routing-tables.md)

### 保护路由（Protection Routes）

保护路由确保隧道对端 IP 的流量不走策略路由，防止路由环路。

**问题**: 如果隧道被设置为策略路由出口，隧道自身的通信包（如 WireGuard 握手）也会走策略路由，导致环路。

**解决**: 优先级 10 的保护路由强制隧道对端 IP 走主路由表。

```bash
# 保护路由规则示例
ip rule add to 203.0.113.50 lookup main pref 10
```

详见 [保护路由机制](../../reference/protection-routes.md)

### 保护路由同步（Dynamic IP Fault Tolerance）

动态 IP 场景下，对端 IP 可能变化，保护路由需要同步更新。

**自动同步时机**:
- `policy apply` 开始前
- `line start <name>` 完成后
- `line start-all` 完成后

**手动同步**:
```bash
sudo twnode policy sync-protection
```

**Cron 定时任务**（推荐）:
```bash
# 每 5 分钟同步一次
*/5 * * * * /usr/local/bin/twnode policy sync-protection
```

详见 [sync-protection 命令](sync-protection.md)

### 故障转移（Failover）

根据连通性检查结果，自动选择最佳出口接口。

**评分算法**:
```
基础评分 = 丢包率评分(60%) + 延迟评分(40%)
最终评分 = 基础评分 - Cost惩罚(Cost × 0.5)
```

**使用场景**:
- 多条线路容灾
- 自动选择最优路径
- 定时检查并切换

详见 [failover 命令](failover.md)

## 策略路由规则管理

### 无缝切换原则

更新策略路由时，必须保证无缝切换，避免网络中断。

**正确做法** ✅:
1. 先添加新规则
2. 检测并清理重复规则
3. 验证规则存在

**错误做法** ❌:
1. 先删除旧规则
2. 再添加新规则
3. （中间有时间窗口，流量无法路由）

### 局部操作支持

大多数 policy 命令支持**局部操作**（单个策略组）和**全局操作**（所有策略组）：

```bash
# 应用单个策略组
sudo twnode policy apply vpn_traffic

# 应用所有策略组
sudo twnode policy apply

# 撤销单个策略组
sudo twnode policy revoke vpn_traffic

# 撤销所有策略组
sudo twnode policy revoke
```

## 配置文件位置

策略组配置文件保存在:

```
/etc/trueword_node/policies/
├── vpn_traffic.json
├── branch_office.json
└── ...
```

全局配置（默认路由）:

```
/etc/trueword_node/config.yaml
```

配置文件格式参见 [配置文件详解](../../reference/config-files.md#策略组配置)

## 示例工作流

### 场景1: 特定 IP 段走 VPN

```bash
# 1. 创建策略组
sudo twnode policy create vpn_traffic tunnel_ab

# 2. 添加 CIDR
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24
sudo twnode policy add-cidr vpn_traffic 192.168.101.0/24

# 3. 应用策略
sudo twnode policy apply vpn_traffic

# 4. 验证
ip rule show
# 应该看到类似:
# 150: from all to 192.168.100.0/24 lookup 50
# 150: from all to 192.168.101.0/24 lookup 50
```

### 场景2: 多策略组优先级控制

```bash
# 高优先级策略组（特定流量）
sudo twnode policy create high_priority tunnel_ab --priority 100
sudo twnode policy add-cidr high_priority 192.168.200.0/24

# 中优先级策略组（普通流量）
sudo twnode policy create medium_priority tunnel_cd --priority 200
sudo twnode policy add-cidr medium_priority 192.168.100.0/24

# 应用所有策略
sudo twnode policy apply

# 列出策略组（按优先级排序）
sudo twnode policy list
```

### 场景3: 动态 IP 场景 + 故障转移

```bash
# 1. 创建 WireGuard 服务器（接收动态 IP 客户端）
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_hk \
  --type wireguard --mode server --listen-port 51820

# 2. 创建策略组
sudo twnode policy create asia_traffic tunnel_hk

# 3. 添加 CIDR
sudo twnode policy add-cidr asia_traffic 192.168.100.0/24

# 4. 应用策略（自动同步保护路由）
sudo twnode policy apply asia_traffic

# 5. 配置 Cron 定时同步保护路由
echo "*/5 * * * * /usr/local/bin/twnode policy sync-protection" | crontab -

# 6. 定期检查并故障转移
sudo twnode line check tunnel_hk 8.8.8.8
sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us
```

## 下一步

- [创建策略组](create.md) - 详细了解策略组创建
- [保护路由同步](sync-protection.md) - 动态 IP 容错机制
- [故障转移](failover.md) - 智能选择最佳出口
- [策略路由实战教程](../../tutorials/policy-routing.md)

---

**导航**: [← 命令参考](../../index.md#命令参考) | [返回首页](../../index.md) | [create →](create.md)
