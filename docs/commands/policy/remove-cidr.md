# policy remove-cidr - 删除路由规则

## 概述

`policy remove-cidr` 命令从策略组删除 CIDR 路由规则。

## 语法

```bash
sudo twnode policy remove-cidr <策略组名> <CIDR>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<策略组名>` | 策略组名称 | 是 |
| `<CIDR>` | 要删除的 CIDR | 是 |

## 示例

### 示例1: 删除单个 CIDR

```bash
$ sudo twnode policy remove-cidr vpn_traffic 192.168.100.0/24

✓ 已从策略组 vpn_traffic 删除 CIDR: 192.168.100.0/24
✓ 配置已保存

提示: 使用 'twnode policy apply vpn_traffic' 应用更改
```

### 示例2: 删除不存在的 CIDR

```bash
$ sudo twnode policy remove-cidr vpn_traffic 192.168.200.0/24

❌ 错误: CIDR 192.168.200.0/24 不存在于策略组 vpn_traffic
```

### 示例3: 批量删除

```bash
sudo twnode policy remove-cidr vpn_traffic 192.168.100.0/24
sudo twnode policy remove-cidr vpn_traffic 192.168.101.0/24
sudo twnode policy remove-cidr vpn_traffic 10.0.0.0/8
```

## 应用更改

删除 CIDR 后需要重新应用策略：

```bash
# 删除 CIDR
sudo twnode policy remove-cidr vpn_traffic 192.168.100.0/24

# 应用策略
sudo twnode policy apply vpn_traffic
```

这会从系统路由规则中删除对应的规则。

## 验证

```bash
# 查看策略组内容
sudo twnode policy list

# 或查看配置文件
cat /etc/trueword_node/policies/vpn_traffic.json
```

## 常见问题

### Q: 删除后需要重新 apply 吗？

A: 是的，否则路由规则仍然存在。

### Q: 可以删除所有 CIDR 吗？

A: 可以，但策略组会变为空，apply 时会警告。

## 下一步

- [添加 CIDR](add-cidr.md) - 向策略组添加 CIDR
- [应用策略](apply.md) - 使策略生效

---

**导航**: [← add-cidr](add-cidr.md) | [返回首页](../../index.md) | [list →](list.md)
