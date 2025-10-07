# policy revoke - 撤销策略

## 概述

`policy revoke` 命令撤销已应用的策略路由规则，恢复到应用前的状态。

## 语法

```bash
# 撤销单个策略组
sudo twnode policy revoke <策略组名>

# 撤销所有策略组
sudo twnode policy revoke
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<策略组名>` | 策略组名称（可选） | 否 |

## 示例

### 示例1: 撤销单个策略组

```bash
$ sudo twnode policy revoke vpn_traffic

撤销策略组: vpn_traffic

【策略组信息】
名称: vpn_traffic
优先级: 100
CIDR 数量: 3

删除路由规则...
✓ 192.168.100.0/24 (pref 100)
✓ 192.168.101.0/24 (pref 100)
✓ 203.0.113.0/24 (pref 100)

✓ 策略组 vpn_traffic 已撤销
```

### 示例2: 撤销所有策略组

```bash
$ sudo twnode policy revoke

撤销所有策略组...

【策略组 1: asia_traffic】
优先级: 100
✓ 已撤销 3 个 CIDR

【策略组 2: us_traffic】
优先级: 200
✓ 已撤销 2 个 CIDR

共撤销 2 个策略组，5 条路由规则
✓ 所有策略组已撤销
```

### 示例3: 撤销未应用的策略组

```bash
$ sudo twnode policy revoke backup_route

ℹ 策略组 backup_route 未应用，无需撤销
```

## 工作流程

```
1. 检查策略组是否已应用

2. 删除路由规则（每个 CIDR）
   └─ ip rule del to <CIDR> pref <priority>

3. 删除表 50 中的路由
   └─ ip route del <CIDR> table 50

4. 更新状态为 Revoked
```

## 验证撤销结果

### 检查路由规则

```bash
# 查看所有规则
ip rule show

# 应该看不到策略组的规则
ip rule show pref 100  # 应该为空
```

### 检查路由表 50

```bash
ip route show table 50

# 应该看不到撤销的 CIDR
```

### 测试路由

```bash
# 测试流量应该不再走策略路由
ip route get 192.168.100.5

# 应该显示走主路由表
```

## 与 delete 的区别

| 操作 | revoke | delete |
|------|--------|--------|
| 删除路由规则 | ✓ | ✓ |
| 删除配置文件 | ✗ | ✓ |
| 可以重新 apply | ✓ | ✗（需重新 create） |

**总结**:
- `revoke` - 临时撤销，保留配置，可以重新 apply
- `delete` - 永久删除，清除配置文件

## 重新应用

撤销后可以随时重新应用：

```bash
# 撤销
sudo twnode policy revoke vpn_traffic

# 重新应用
sudo twnode policy apply vpn_traffic
```

## 批量操作

### 撤销特定策略组

```bash
for group in asia_traffic us_traffic backup_route; do
    sudo twnode policy revoke $group
done
```

### 撤销所有策略（推荐）

```bash
sudo twnode policy revoke
```

## 常见问题

### Q: revoke 会删除配置文件吗？

A: 不会。只删除路由规则，配置文件保留。

### Q: revoke 后流量走哪里？

A: 走主路由表，通常是物理接口的默认路由。

### Q: revoke 会影响隧道吗？

A: 不会。隧道继续运行，只是策略路由规则被删除。

### Q: 撤销所有策略包括默认路由吗？

A: 包括。`policy revoke` 会撤销所有策略，包括默认路由。

### Q: 撤销后如何恢复？

A: 使用 `policy apply` 重新应用：

```bash
sudo twnode policy apply <策略组名>
```

## 下一步

- [应用策略](apply.md) - 重新应用策略
- [删除策略组](delete.md) - 永久删除策略组
- [列出策略组](list.md) - 查看策略组状态

---

**导航**: [← apply](apply.md) | [返回首页](../../index.md) | [delete →](delete.md)
