# line list - 列出隧道

## 概述

`line list` 命令列出所有已创建的隧道及其状态信息。

## 语法

```bash
sudo twnode line list
```

## 输出格式

使用表格显示所有隧道信息，包括：

- 隧道名称
- 隧道类型
- 父接口
- 本地虚拟 IP
- 对端虚拟 IP
- 状态（Active/Inactive）

## 示例输出

```bash
$ sudo twnode line list

╔═══════════════════════════════════════════════════════════════════╗
║                           隧道列表                              ║
╚═══════════════════════════════════════════════════════════════════╝

+-------------+------------+--------------+------------+-------------+----------+
| 隧道名称    | 类型       | 父接口       | 本地 VIP   | 对端 VIP    | 状态     |
+-------------+------------+--------------+------------+-------------+----------+
| tunnel_hk   | WireGuard  | eth0         | 10.0.0.1   | 10.0.0.2    | Active   |
| tunnel_us   | WireGuard  | eth0         | 10.0.1.1   | 10.0.1.2    | Inactive |
| tun01       | GRE        | eth0         | 10.0.2.1   | 10.0.2.2    | Active   |
| tunnel_l2   | WireGuard  | tunnel_hk    | 172.16.0.1 | 172.16.0.2  | Active   |
+-------------+------------+--------------+------------+-------------+----------+

共 4 个隧道（3 个运行中，1 个停止）
```

## 状态说明

| 状态 | 说明 | 图标 |
|------|------|------|
| `Active` | 隧道已启动，接口处于 UP 状态 | ✓ |
| `Inactive` | 隧道已创建但未启动 | ⊗ |

## 详细信息模式

使用 `-v` 或 `--verbose` 参数查看详细信息：

```bash
sudo twnode line list -v
```

**输出示例**:

```bash
╔═══════════════════════════════════════════════════════════════════╗
║                    隧道列表（详细模式）                         ║
╚═══════════════════════════════════════════════════════════════════╝

【隧道 1: tunnel_hk】
  类型: WireGuard (服务器模式)
  父接口: eth0
  本地 IP: 192.168.1.100
  对端 IP: 103.118.40.121 (动态)
  本地 VIP: 10.0.0.1
  对端 VIP: 10.0.0.2
  监听端口: 51820
  状态: ✓ Active
  握手: 30 秒前
  传输: 15.2 KB 接收 / 8.3 KB 发送

【隧道 2: tunnel_us】
  类型: WireGuard (客户端模式)
  父接口: eth0
  本地 IP: 192.168.1.100
  对端 IP: 198.51.100.20
  本地 VIP: 10.0.1.1
  对端 VIP: 10.0.1.2
  对端端口: 51820
  状态: ⊗ Inactive

【隧道 3: tun01】
  类型: GRE over IPsec
  父接口: eth0
  本地 IP: 192.168.1.100
  对端 IP: 203.0.113.50
  本地 VIP: 10.0.2.1
  对端 VIP: 10.0.2.2
  加密: 启用 (AES-256 + SHA256)
  状态: ✓ Active

【隧道 4: tunnel_l2】
  类型: WireGuard (服务器模式)
  父接口: tunnel_hk (嵌套隧道)
  本地 IP: 10.0.0.1
  对端 IP: 0.0.0.0 (等待连接)
  本地 VIP: 172.16.0.1
  对端 VIP: 172.16.0.2
  监听端口: 51821
  状态: ✓ Active

共 4 个隧道（3 个运行中，1 个停止）
```

## 过滤选项

### 按状态过滤

```bash
# 仅显示运行中的隧道
sudo twnode line list --status active

# 仅显示停止的隧道
sudo twnode line list --status inactive
```

### 按类型过滤

```bash
# 仅显示 WireGuard 隧道
sudo twnode line list --type wireguard

# 仅显示 GRE over IPsec 隧道
sudo twnode line list --type gre
```

### 组合过滤

```bash
# 运行中的 WireGuard 隧道
sudo twnode line list --type wireguard --status active
```

## JSON 输出

使用 `--json` 参数输出 JSON 格式，便于脚本处理：

```bash
sudo twnode line list --json
```

**输出示例**:

```json
{
  "tunnels": [
    {
      "name": "tunnel_hk",
      "type": "wireguard",
      "parent_interface": "eth0",
      "local_ip": "192.168.1.100",
      "remote_ip": "103.118.40.121",
      "local_vip": "10.0.0.1",
      "remote_vip": "10.0.0.2",
      "status": "active",
      "mode": "server",
      "listen_port": 51820,
      "latest_handshake": "30s ago",
      "transfer": {
        "rx_bytes": 15564,
        "tx_bytes": 8499
      }
    },
    {
      "name": "tunnel_us",
      "type": "wireguard",
      "parent_interface": "eth0",
      "local_ip": "192.168.1.100",
      "remote_ip": "198.51.100.20",
      "local_vip": "10.0.1.1",
      "remote_vip": "10.0.1.2",
      "status": "inactive",
      "mode": "client",
      "peer_port": 51820
    }
  ],
  "summary": {
    "total": 2,
    "active": 1,
    "inactive": 1
  }
}
```

## 脚本使用

### 获取所有隧道名称

```bash
# 提取隧道名称（跳过表头）
sudo twnode line list | awk 'NR>3 && NF {print $1}' | grep -v "^+"

# 或使用 JSON 模式
sudo twnode line list --json | jq -r '.tunnels[].name'
```

### 统计隧道数量

```bash
# 统计运行中的隧道
sudo twnode line list --status active | grep -c "Active"

# 或使用 JSON 模式
sudo twnode line list --json | jq '.summary.active'
```

### 批量操作

```bash
# 启动所有停止的隧道
for tunnel in $(sudo twnode line list --status inactive --json | jq -r '.tunnels[].name'); do
    sudo twnode line start $tunnel
done

# 检查所有运行中的隧道
for tunnel in $(sudo twnode line list --status active --json | jq -r '.tunnels[].name'); do
    sudo twnode line check $tunnel 8.8.8.8
done
```

## 空列表

如果没有任何隧道：

```bash
$ sudo twnode line list

╔═══════════════════════════════════════════════════════════════════╗
║                           隧道列表                              ║
╚═══════════════════════════════════════════════════════════════════╝

暂无隧道

提示: 使用 'twnode line create' 创建隧道
```

## 常见问题

### Q: 如何查看单个隧道的详细信息？

A: 使用 `-v` 参数并结合 grep：

```bash
sudo twnode line list -v | grep -A 10 "隧道.*: tunnel_hk"
```

或使用 JSON 模式：

```bash
sudo twnode line list --json | jq '.tunnels[] | select(.name=="tunnel_hk")'
```

### Q: 状态显示 Active 但无法 ping 通？

A: 可能原因：
1. WireGuard 握手失败（检查 `latest_handshake`）
2. 对端未启动
3. 防火墙阻止
4. 路由配置错误

使用 `line check` 命令详细检查：

```bash
sudo twnode line check tunnel_hk 8.8.8.8
```

### Q: 如何导出隧道列表？

A: 使用重定向或 JSON 格式：

```bash
# 表格格式导出
sudo twnode line list > tunnels.txt

# JSON 格式导出
sudo twnode line list --json > tunnels.json
```

### Q: 列表中的隧道顺序是什么？

A: 按创建时间排序（先创建的在前）。可以使用 `--sort` 参数自定义排序：

```bash
# 按名称排序
sudo twnode line list --sort name

# 按状态排序（Active 在前）
sudo twnode line list --sort status
```

## 相关命令

- `line create` - 创建新隧道
- `line start` - 启动隧道
- `line stop` - 停止隧道
- `line delete` - 删除隧道
- `line check` - 检查连通性

## 下一步

- [检查连通性](check.md) - 测试隧道状态
- [启动隧道](start.md) - 启动停止的隧道
- [创建隧道](create.md) - 创建新隧道

---

**导航**: [← stop](stop.md) | [返回首页](../../index.md) | [check →](check.md)
