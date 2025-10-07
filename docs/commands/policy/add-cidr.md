# policy add-cidr - 添加路由规则

## 概述

`policy add-cidr` 命令向策略组添加 CIDR 路由规则。

## 语法

```bash
sudo twnode policy add-cidr <策略组名> <CIDR>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<策略组名>` | 策略组名称 | 是 |
| `<CIDR>` | IP 地址段（CIDR 格式） | 是 |

## 示例

### 示例1: 添加单个 CIDR

```bash
$ sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24

✓ 已添加 CIDR: 192.168.100.0/24 到策略组 vpn_traffic
✓ 配置已保存

提示: 使用 'twnode policy apply vpn_traffic' 应用策略
```

### 示例2: 添加多个 CIDR

```bash
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24
sudo twnode policy add-cidr vpn_traffic 192.168.101.0/24
sudo twnode policy add-cidr vpn_traffic 10.0.0.0/8
```

### 示例3: 添加单个 IP

```bash
sudo twnode policy add-cidr vpn_traffic 192.168.100.5/32
```

### 示例4: 策略组不存在

```bash
$ sudo twnode policy add-cidr notexist 192.168.100.0/24

❌ 错误: 策略组不存在: notexist
请先创建策略组: twnode policy create notexist <出口接口>
```

### 示例5: 无效的 CIDR

```bash
$ sudo twnode policy add-cidr vpn_traffic 192.168.100.0/33

❌ 错误: 无效的 CIDR: 192.168.100.0/33
CIDR 格式应为: IP/掩码长度 (如 192.168.0.0/24)
```

## CIDR 格式

### 标准格式

```
<IP地址>/<掩码长度>
```

### 常用 CIDR 示例

| CIDR | 含义 | IP 数量 |
|------|------|---------|
| `192.168.1.0/24` | 192.168.1.0 - 192.168.1.255 | 256 |
| `10.0.0.0/8` | 10.0.0.0 - 10.255.255.255 | 16,777,216 |
| `172.16.0.0/12` | 172.16.0.0 - 172.31.255.255 | 1,048,576 |
| `192.168.1.5/32` | 仅 192.168.1.5 | 1 |
| `0.0.0.0/0` | 所有 IP | 全部 |

### 掩码长度范围

- IPv4: `/0` - `/32`
- 常用: `/24` (256个IP), `/16` (65,536个IP), `/8` (16,777,216个IP)

## 批量添加

### 方法1: 循环添加

```bash
for cidr in 192.168.100.0/24 192.168.101.0/24 192.168.102.0/24; do
    sudo twnode policy add-cidr vpn_traffic $cidr
done
```

### 方法2: 从文件导入

创建文件 `cidrs.txt`:

```
192.168.100.0/24
192.168.101.0/24
10.0.0.0/8
172.16.0.0/12
# 注释会被忽略
```

导入:

```bash
while read cidr; do
    # 跳过注释和空行
    [[ "$cidr" =~ ^#.*$ ]] && continue
    [[ -z "$cidr" ]] && continue

    sudo twnode policy add-cidr vpn_traffic "$cidr"
done < cidrs.txt
```

### 方法3: 使用 import 命令（如果实现）

```bash
sudo twnode policy import vpn_traffic cidrs.txt
```

## 配置文件

CIDR 添加到策略组配置文件：

```json
{
  "name": "vpn_traffic",
  "exit_interface": "tunnel_hk",
  "priority": 100,
  "from_source": "",
  "cidrs": [
    "192.168.100.0/24",
    "192.168.101.0/24",
    "10.0.0.0/8"
  ],
  "cost": 0
}
```

## 重复检测

添加已存在的 CIDR 会提示警告：

```bash
$ sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24

⚠️ 警告: CIDR 192.168.100.0/24 已存在于策略组 vpn_traffic
```

## 应用策略

添加 CIDR 后需要应用策略才能生效：

```bash
# 添加 CIDR
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24

# 应用策略
sudo twnode policy apply vpn_traffic
```

或一次性操作：

```bash
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24 && \
sudo twnode policy apply vpn_traffic
```

## 常见问题

### Q: 添加 CIDR 后立即生效吗？

A: 不会。需要执行 `policy apply` 才能生效。

### Q: 可以添加重叠的 CIDR 吗？

A: 可以，但不推荐。路由规则会按添加顺序匹配。

### Q: 如何验证 CIDR 已添加？

A: 使用 `policy list` 查看：

```bash
sudo twnode policy list
```

### Q: CIDR 数量有限制吗？

A: 理论上无限制，但太多 CIDR 会影响路由性能。建议合并相邻 CIDR。

## 下一步

- [删除 CIDR](remove-cidr.md) - 从策略组删除 CIDR
- [应用策略](apply.md) - 使策略生效
- [列出策略组](list.md) - 查看策略组内容

---

**导航**: [← create](create.md) | [返回首页](../../index.md) | [remove-cidr →](remove-cidr.md)
