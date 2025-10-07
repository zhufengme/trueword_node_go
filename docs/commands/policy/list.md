# policy list - 列出策略组

## 概述

`policy list` 命令列出所有策略组及其配置信息。

## 语法

```bash
sudo twnode policy list
```

## 输出示例

```bash
$ sudo twnode policy list

╔═══════════════════════════════════════════════════════════════════╗
║                         策略组列表                              ║
╚═══════════════════════════════════════════════════════════════════╝

+---------------+-------------+----------+-------------+-----------+
| 策略组名称    | 出口接口    | 优先级   | CIDR 数量   | 状态      |
+---------------+-------------+----------+-------------+-----------+
| asia_traffic  | tunnel_hk   | 100      | 3           | Applied   |
| us_traffic    | tunnel_us   | 200      | 2           | Applied   |
| backup_route  | tunnel_bak  | 800      | 1           | Revoked   |
+---------------+-------------+----------+-------------+-----------+

共 3 个策略组（2 个已应用，1 个未应用）
```

## 详细模式

使用 `-v` 或 `--verbose` 参数查看详细信息：

```bash
$ sudo twnode policy list -v

╔═══════════════════════════════════════════════════════════════════╗
║                   策略组列表（详细模式）                        ║
╚═══════════════════════════════════════════════════════════════════╝

【策略组 1: asia_traffic】
  出口接口: tunnel_hk
  优先级: 100
  源地址限制: 无
  成本: 0
  状态: ✓ Applied
  CIDR 列表:
    - 192.168.100.0/24
    - 192.168.101.0/24
    - 203.0.113.0/24

【策略组 2: us_traffic】
  出口接口: tunnel_us
  优先级: 200
  源地址限制: 10.0.0.0/8
  成本: 0
  状态: ✓ Applied
  CIDR 列表:
    - 192.168.200.0/24
    - 198.51.100.0/24

【策略组 3: backup_route】
  出口接口: tunnel_bak
  优先级: 800
  源地址限制: 无
  成本: 10
  状态: ⊗ Revoked
  CIDR 列表:
    - 172.16.0.0/12

共 3 个策略组（2 个已应用，1 个未应用）
```

## 状态说明

| 状态 | 说明 |
|------|------|
| `Applied` | 策略已应用到系统 |
| `Revoked` | 策略已撤销或未应用 |
| `Modified` | 配置已修改，需重新 apply |

## 过滤和排序

### 按状态过滤

```bash
# 仅显示已应用的策略组
sudo twnode policy list --status applied

# 仅显示未应用的策略组
sudo twnode policy list --status revoked
```

### 按优先级排序

```bash
# 按优先级升序（默认）
sudo twnode policy list --sort priority

# 按名称排序
sudo twnode policy list --sort name
```

## JSON 输出

```bash
$ sudo twnode policy list --json

{
  "groups": [
    {
      "name": "asia_traffic",
      "exit_interface": "tunnel_hk",
      "priority": 100,
      "from_source": "",
      "cidrs": [
        "192.168.100.0/24",
        "192.168.101.0/24",
        "203.0.113.0/24"
      ],
      "cost": 0,
      "status": "applied"
    },
    {
      "name": "us_traffic",
      "exit_interface": "tunnel_us",
      "priority": 200,
      "from_source": "10.0.0.0/8",
      "cidrs": [
        "192.168.200.0/24",
        "198.51.100.0/24"
      ],
      "cost": 0,
      "status": "applied"
    }
  ],
  "summary": {
    "total": 2,
    "applied": 2,
    "revoked": 0
  }
}
```

## 空列表

```bash
$ sudo twnode policy list

╔═══════════════════════════════════════════════════════════════════╗
║                         策略组列表                              ║
╚═══════════════════════════════════════════════════════════════════╝

暂无策略组

提示: 使用 'twnode policy create' 创建策略组
```

## 常见问题

### Q: 如何查看单个策略组的详细信息？

A: 使用 `-v` 参数：

```bash
sudo twnode policy list -v | grep -A 10 "asia_traffic"
```

或使用 JSON 模式：

```bash
sudo twnode policy list --json | jq '.groups[] | select(.name=="asia_traffic")'
```

### Q: 如何导出策略组列表？

A: 使用 JSON 格式导出：

```bash
sudo twnode policy list --json > policies_backup.json
```

## 下一步

- [创建策略组](create.md) - 创建新策略组
- [应用策略](apply.md) - 应用策略组
- [撤销策略](revoke.md) - 撤销策略组

---

**导航**: [← remove-cidr](remove-cidr.md) | [返回首页](../../index.md) | [apply →](apply.md)
