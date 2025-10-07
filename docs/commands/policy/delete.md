# policy delete - 删除策略组

## 概述

`policy delete` 命令永久删除策略组及其配置文件。

## 语法

```bash
sudo twnode policy delete <策略组名>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<策略组名>` | 要删除的策略组名称 | 是 |

## 示例

### 示例1: 删除策略组

```bash
$ sudo twnode policy delete backup_route

【删除策略组】
名称: backup_route
出口接口: tunnel_bak
优先级: 800
CIDR 数量: 1

确认删除？这将清除所有配置。(yes/no): yes

✓ 撤销策略规则
✓ 删除配置文件
✓ 策略组已删除
```

### 示例2: 删除已应用的策略组

```bash
$ sudo twnode policy delete vpn_traffic

【删除策略组】
名称: vpn_traffic
出口接口: tunnel_hk
优先级: 100
CIDR 数量: 3
状态: Applied

⚠️ 警告: 策略组当前已应用
删除将自动撤销所有路由规则

确认删除？(yes/no): yes

✓ 撤销策略规则
✓ 删除配置文件
✓ 策略组已删除
```

### 示例3: 策略组不存在

```bash
$ sudo twnode policy delete notexist

❌ 错误: 策略组不存在: notexist
```

## 工作流程

```
1. 检查策略组是否存在

2. 检查策略组状态
   └─ 如果已应用，先撤销规则

3. 删除配置文件
   └─ rm /etc/trueword_node/policies/<name>.json

4. 完成删除
```

## 确认提示

删除策略组前会提示确认，**必须输入 "yes"** 才会继续。

## 强制删除

使用 `--force` 参数跳过确认：

```bash
sudo twnode policy delete backup_route --force
```

**警告**: 使用 `--force` 会跳过所有检查和确认。

## 批量删除

```bash
# 删除多个策略组
for group in group1 group2 group3; do
    sudo twnode policy delete $group --force
done
```

## 恢复删除的策略组

删除后无法自动恢复，需要重新创建：

```bash
# 备份（删除前）
sudo cp /etc/trueword_node/policies/vpn_traffic.json ~/vpn_traffic.json.backup

# 删除
sudo twnode policy delete vpn_traffic

# 恢复（需要手动重新创建）
# 查看备份文件，手动执行 create 和 add-cidr 命令
```

## 与 revoke 的区别

| 操作 | delete | revoke |
|------|--------|--------|
| 删除路由规则 | ✓ | ✓ |
| 删除配置文件 | ✓ | ✗ |
| 可以恢复 | ✗ | ✓（重新 apply） |

**总结**:
- `delete` - 永久删除，清除所有配置
- `revoke` - 临时撤销，保留配置文件

## 常见问题

### Q: 删除后可以恢复吗？

A: 不能自动恢复。需要重新创建策略组并添加 CIDR。

### Q: 删除前应该先 revoke 吗？

A: 不需要。`delete` 会自动检测并撤销已应用的策略。

### Q: 删除策略组会影响隧道吗？

A: 不会。隧道继续运行，只是策略路由规则被删除。

### Q: 如何批量删除所有策略组？

A: 遍历所有策略组：

```bash
for group in $(sudo twnode policy list --json | jq -r '.groups[].name'); do
    sudo twnode policy delete $group --force
done
```

## 下一步

- [创建策略组](create.md) - 重新创建策略组
- [撤销策略](revoke.md) - 临时撤销策略
- [列出策略组](list.md) - 查看策略组

---

**导航**: [← revoke](revoke.md) | [返回首页](../../index.md) | [set-priority →](set-priority.md)
