# line delete - 删除隧道

## 概述

`line delete` 命令用于删除已创建的隧道，包括清理网络配置、撤销路由规则和删除配置文件。

## 语法

```bash
sudo twnode line delete <隧道名>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<隧道名>` | 要删除的隧道名称 | 是 |

## 工作流程

```
1. 加载隧道配置
   └─ 从 /etc/trueword_node/tunnels/<name>.yaml

2. 停止隧道（如果正在运行）
   └─ 执行 stop 操作

3. 执行撤销命令
   ├─ 读取撤销文件: /var/lib/trueword_node/rev/<name>.rev
   ├─ 逐行执行撤销命令
   │  ├─ 删除隧道接口: ip link del <name>
   │  ├─ 删除虚拟 IP 路由规则
   │  └─ 删除保护路由规则
   └─ 删除撤销文件

4. 删除 IPsec 连接（GRE over IPsec 隧道）
   ├─ 读取 IPsec 撤销文件
   ├─ 删除 XFRM state 和 policy
   └─ 删除 IPsec 撤销文件

5. 删除配置文件
   └─ 删除 /etc/trueword_node/tunnels/<name>.yaml

6. 删除对端配置文件（WireGuard）
   └─ 删除 /var/lib/trueword_node/peer_configs/<name>.txt
```

## 撤销命令示例

### WireGuard 隧道

```bash
# /var/lib/trueword_node/rev/tunnel_hk.rev
ip link set tunnel_hk down
ip link del tunnel_hk
ip rule del to 10.0.0.2/32 lookup 80 pref 80
ip rule del to 103.118.40.121 lookup main pref 10
```

### GRE over IPsec 隧道

```bash
# /var/lib/trueword_node/rev/tun01.rev
ip link set tun01 down
ip tunnel del tun01
ip rule del to 10.0.1.2/32 lookup 80 pref 80
ip rule del to 203.0.113.50 lookup main pref 10
```

```bash
# /var/lib/trueword_node/rev/192.168.1.100-203.0.113.50.rev
ip xfrm state del src 192.168.1.100 dst 203.0.113.50 proto esp spi 0xa1b2c3d4
ip xfrm state del src 203.0.113.50 dst 192.168.1.100 proto esp spi 0xa1b2c3d4
ip xfrm policy del src 192.168.1.100 dst 203.0.113.50 dir out
ip xfrm policy del src 203.0.113.50 dst 192.168.1.100 dir in
```

## 示例

### 示例1: 删除单个隧道

```bash
$ sudo twnode line delete tunnel_hk

【删除隧道】
隧道名称: tunnel_hk
隧道类型: WireGuard

确认删除？这将清除所有网络配置。(yes/no): yes

✓ 停止隧道
✓ 删除隧道接口
✓ 清理路由规则
✓ 删除配置文件
✓ 隧道已删除
```

### 示例2: 删除 GRE over IPsec 隧道

```bash
$ sudo twnode line delete tun01

【删除隧道】
隧道名称: tun01
隧道类型: GRE over IPsec

确认删除？这将清除所有网络配置。(yes/no): yes

✓ 停止隧道
✓ 删除 GRE 隧道
✓ 删除 IPsec 连接
✓ 清理路由规则
✓ 删除配置文件
✓ 隧道已删除
```

### 示例3: 删除不存在的隧道

```bash
$ sudo twnode line delete tunnel_notexist

❌ 错误: 隧道不存在: tunnel_notexist
```

## 注意事项

### 1. 确认提示

删除隧道前会提示确认，**必须输入 "yes"** 才会继续。

### 2. 依赖检查

如果其他隧道依赖此隧道（作为父接口），删除会失败：

```bash
$ sudo twnode line delete tun01

❌ 错误: 无法删除隧道 tun01
以下隧道依赖此隧道作为父接口:
  - tun02
  - tun03

请先删除这些隧道，或修改它们的父接口。
```

### 3. 策略路由检查

如果策略组使用此隧道作为出口，会警告但不会阻止删除：

```bash
$ sudo twnode line delete tunnel_hk

⚠️ 警告: 以下策略组使用此隧道作为出口:
  - asia_traffic
  - vpn_routes

删除隧道后，这些策略组将无法正常工作。
建议先修改策略组的出口接口或删除策略组。

确认删除？(yes/no):
```

### 4. 静默删除（脚本使用）

使用 `--force` 参数跳过确认：

```bash
# 静默删除（谨慎使用）
sudo twnode line delete tunnel_hk --force
```

**警告**: 使用 `--force` 会跳过所有检查和确认，可能导致网络中断。

## 批量删除

删除多个隧道：

```bash
# 方法1: 循环删除
for tunnel in tunnel_hk tunnel_tw tunnel_us; do
    sudo twnode line delete $tunnel --force
done

# 方法2: 使用 list 和 awk
sudo twnode line list | awk 'NR>1 {print $1}' | while read tunnel; do
    sudo twnode line delete $tunnel --force
done
```

## 清理验证

删除后验证清理是否完整：

```bash
# 检查隧道接口是否还存在
ip link show tunnel_hk
# 应该显示: Device "tunnel_hk" does not exist.

# 检查路由规则是否清理
ip rule show | grep tunnel_hk
# 应该没有输出

# 检查配置文件是否删除
ls /etc/trueword_node/tunnels/tunnel_hk.yaml
# 应该显示: No such file or directory

# 检查 WireGuard 状态（如果是 WireGuard 隧道）
sudo wg show tunnel_hk
# 应该显示: Unable to access interface: No such device
```

## 恢复删除的隧道

删除后无法自动恢复，需要重新创建。建议在删除前备份配置：

```bash
# 备份配置
sudo cp /etc/trueword_node/tunnels/tunnel_hk.yaml ~/tunnel_hk.yaml.backup

# 删除隧道
sudo twnode line delete tunnel_hk

# 如果需要恢复，使用备份的配置重新创建
# （需要手动执行 create 命令，不能直接恢复配置文件）
```

## 常见问题

### Q: 删除隧道会影响其他隧道吗？

A: 不会，除非其他隧道使用此隧道作为父接口。删除前会检查依赖关系。

### Q: 删除后策略路由会自动更新吗？

A: 不会。需要手动修改或删除使用此隧道的策略组。

### Q: 删除时出错，隧道部分清理怎么办？

A: 撤销命令会继续执行，即使某些步骤失败。可以手动检查并清理：

```bash
# 手动删除接口
sudo ip link del tunnel_hk

# 手动删除路由规则
sudo ip rule del to 10.0.0.2/32 lookup 80 pref 80

# 手动删除配置文件
sudo rm /etc/trueword_node/tunnels/tunnel_hk.yaml
```

### Q: 可以批量删除所有隧道吗？

A: 可以，但不推荐使用 `--force`。建议逐个删除或使用脚本：

```bash
# 安全的批量删除（会提示确认）
for tunnel in $(sudo twnode line list | awk 'NR>1 {print $1}'); do
    sudo twnode line delete $tunnel
done
```

## 下一步

- [创建隧道](create.md) - 重新创建隧道
- [隧道管理](index.md) - 其他隧道管理命令

---

**导航**: [← line 命令](index.md) | [返回首页](../../index.md) | [start →](start.md)
