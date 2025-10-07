# 保护路由机制

本文档详细说明 TrueWord Node 的保护路由机制，这是防止路由环路的核心功能。

## 什么是保护路由？

保护路由是优先级为 10 的特殊路由规则，确保隧道对端 IP 的流量不走策略路由，而是通过主路由表（即物理接口）发送。

### 规则格式

```bash
ip rule add to <对端IP> lookup main pref 10
```

**关键点**:
- **优先级 10**: 高于所有用户策略（100-899）
- **lookup main**: 查询主路由表，使用物理接口
- **to <对端IP>**: 仅匹配隧道对端 IP

## 为什么需要保护路由？

### 路由环路问题

**场景**: 隧道作为策略路由出口

```
1. 创建 WireGuard 隧道
   ├─ 本地: eth0 (192.168.1.100)
   ├─ 对端: 203.0.113.50
   └─ 隧道接口: tunnel_hk

2. 设置策略路由
   └─ 所有流量 → tunnel_hk

3. WireGuard 握手包
   ├─ 目标: 203.0.113.50:51820
   ├─ 匹配策略路由
   └─ 进入 tunnel_hk

4. 问题
   └─ 握手包本身需要通过 eth0 发送到 203.0.113.50
   └─ 但被策略路由重定向到 tunnel_hk
   └─ 隧道无法建立（路由环路）
```

### 解决方案

添加保护路由：

```bash
ip rule add to 203.0.113.50 lookup main pref 10
```

**匹配顺序**:

```
发往 203.0.113.50 的流量
  ↓
优先级 10（保护路由）→ 匹配！→ 查询主路由表 → 通过 eth0
  ↓
优先级 100-899（策略路由）→ 跳过
```

这样，WireGuard 握手包会通过 eth0 发送到 203.0.113.50，隧道正常建立。

## 保护路由创建时机

### 1. 隧道创建时

创建隧道时自动添加保护路由：

```bash
$ sudo twnode line create eth0 203.0.113.50 10.0.0.2 10.0.0.1 tunnel_hk

# 内部执行:
ip rule add to 203.0.113.50 lookup main pref 10

# 记录到配置文件:
protected_ip: 203.0.113.50
```

### 2. 隧道启动时

启动隧道时检查并添加保护路由：

```bash
$ sudo twnode line start tunnel_hk

# 内部执行:
if 保护路由不存在:
    ip rule add to <protected_ip> lookup main pref 10
```

### 3. 策略应用时

应用策略前自动同步保护路由：

```bash
$ sudo twnode policy apply

# 内部执行:
policy sync-protection  # 自动同步所有保护路由
ApplyGroup(...)
```

### 4. 定时同步

通过 cron 定时同步（推荐用于动态 IP 场景）：

```bash
*/5 * * * * /usr/local/bin/twnode policy sync-protection
```

## 动态 IP 容错

### 问题

WireGuard 服务器接收动态 IP 客户端时，客户端 IP 可能变化：

```
初始状态:
  客户端 IP: 103.118.40.121
  保护路由: ip rule add to 103.118.40.121 lookup main pref 10

客户端 IP 变化:
  新 IP: 103.118.50.200
  旧保护路由: 仍然是 103.118.40.121（无效）
  新 IP 没有保护路由 → 可能导致路由环路
```

### 解决方案: sync-protection

`policy sync-protection` 命令自动检测 IP 变化并更新：

```bash
$ sudo twnode policy sync-protection

同步保护路由...
  ℹ 从运行状态检测到 WireGuard 隧道 tunnel_hk 的对端IP: 103.118.50.200
  ⚠ WireGuard 隧道 tunnel_hk 对端IP已变化: 103.118.40.121 → 103.118.50.200
  ✓ 已删除旧保护路由: 103.118.40.121
  ✓ 已添加新保护路由: 103.118.50.200
  ✓ 已更新配置文件: ProtectedIP = 103.118.50.200
✓ 保护路由同步完成
```

### 工作流程

```
1. 加载所有隧道配置

2. 对每个隧道获取当前对端 IP:
   ├─ GRE 隧道 → 从配置文件读取 RemoteIP
   ├─ WireGuard 客户端 → 从配置文件读取 RemoteIP
   └─ WireGuard 服务器 → 从运行状态获取
      └─ wg show <interface> endpoints

3. 检查 ProtectedIP 字段:
   ├─ IP 未变化 → 跳过
   ├─ IP 已变化 → 更新保护路由
   │  ├─ 删除旧规则: ip rule del to <旧IP> pref 10
   │  ├─ 添加新规则: ip rule add to <新IP> lookup main pref 10
   │  └─ 更新配置文件
   └─ 缺失保护路由 → 添加保护路由

4. 清理僵尸规则:
   └─ 删除无对应隧道的保护路由
```

### WireGuard 对端 IP 检测

```bash
# 获取 WireGuard 运行时对端 IP
$ sudo wg show tunnel_hk endpoints

# 输出:
aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=    103.118.50.200:51820
                                                    ↑ 对端 IP
```

解析逻辑：

```go
func GetWireGuardPeerEndpoint(interfaceName string) (string, error) {
    cmd := exec.Command("wg", "show", interfaceName, "endpoints")
    output, _ := cmd.Output()

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        parts := strings.Fields(line)
        if len(parts) >= 2 {
            endpoint := parts[1]  // 格式: IP:端口
            ip := strings.Split(endpoint, ":")[0]
            return ip, nil
        }
    }
    return "", fmt.Errorf("no peer endpoint found")
}
```

## 保护路由的限制

### GRE 隧道和 WireGuard 客户端

对端 IP 固定，无需动态更新：

```yaml
# /etc/trueword_node/tunnels/tun01.yaml
remote_ip: 203.0.113.50
protected_ip: 203.0.113.50  # 固定
```

### WireGuard 服务器

对端 IP 可能变化，需要定时同步：

```yaml
# /etc/trueword_node/tunnels/tunnel_hk.yaml
remote_ip: 0.0.0.0  # 占位符
protected_ip: 103.118.50.200  # 动态更新
```

## 配置文件中的 ProtectedIP

### 字段作用

记录当前受保护的 IP 地址：

```yaml
name: tunnel_hk
remote_ip: 0.0.0.0
protected_ip: 103.118.50.200
```

### 更新方式

- **隧道创建时**: 使用 `remote_ip`（GRE 和 WireGuard 客户端）或 `0.0.0.0`（WireGuard 服务器）
- **首次连接后**: WireGuard 服务器通过 `wg show` 获取实际 IP 并更新
- **同步时**: `sync-protection` 检测变化并更新

### 手动修改

不推荐手动修改，应使用 `sync-protection` 自动更新：

```bash
sudo twnode policy sync-protection
```

## 僵尸规则清理

### 什么是僵尸规则？

优先级 10 的保护路由，但没有对应的隧道配置。

**产生原因**:
- 隧道已删除，但保护路由未清理
- 手动添加的保护路由
- 配置文件损坏

### 检测和清理

`sync-protection` 会自动清理僵尸规则：

```bash
$ sudo twnode policy sync-protection

同步保护路由...
  ✓ 保护 GRE 隧道 tun01 的远程IP 203.0.113.50
  ✓ 保护 WireGuard 客户端 tunnel_us 的远程IP 192.168.1.100
  清理 2 个僵尸规则...
  ✓ 已清理僵尸规则: 5.6.7.8
  ✓ 已清理僵尸规则: 9.10.11.12
✓ 保护路由同步完成
```

**清理逻辑**:

```
1. 获取所有优先级 10 的规则
2. 对每个规则提取目标 IP
3. 检查是否有对应的隧道
4. 如果没有对应隧道 → 删除规则
```

## 验证保护路由

### 查看保护路由规则

```bash
# 查看所有优先级 10 的规则
ip rule show pref 10

# 输出示例:
10:     from all to 203.0.113.50 lookup main
10:     from all to 192.168.1.100 lookup main
10:     from all to 103.118.50.200 lookup main
```

### 测试保护路由

```bash
# 测试发往对端 IP 的流量
ip route get 203.0.113.50

# 应该显示通过物理接口:
203.0.113.50 via 192.168.1.1 dev eth0 src 192.168.1.100
```

### 检查配置文件

```bash
# 查看 ProtectedIP 字段
cat /etc/trueword_node/tunnels/tunnel_hk.yaml | grep protected_ip

# 输出:
protected_ip: 103.118.50.200
```

### 验证动态更新

```bash
# 1. 查看当前 ProtectedIP
cat /etc/trueword_node/tunnels/tunnel_hk.yaml | grep protected_ip

# 2. 查看 WireGuard 实际对端 IP
sudo wg show tunnel_hk endpoints

# 3. 执行同步
sudo twnode policy sync-protection

# 4. 再次查看 ProtectedIP（应已更新）
cat /etc/trueword_node/tunnels/tunnel_hk.yaml | grep protected_ip
```

## 自动化同步

### Cron 定时任务（推荐）

```bash
crontab -e
```

添加：

```bash
# 每 5 分钟同步一次
*/5 * * * * /usr/local/bin/twnode policy sync-protection >/dev/null 2>&1
```

### 同步间隔建议

- **动态 IP 场景**: 每 5 分钟
- **稳定 IP 场景**: 每 30 分钟或不定时

### 监控同步结果

记录日志：

```bash
*/5 * * * * /usr/local/bin/twnode policy sync-protection >> /var/log/twnode_sync.log 2>&1
```

检查 IP 变化：

```bash
grep "对端IP已变化" /var/log/twnode_sync.log
```

## 常见问题

### Q: 保护路由会影响性能吗？

A: 不会。保护路由只是一条路由规则，匹配过程非常快。

### Q: 可以手动添加保护路由吗？

A: 可以，但不推荐。建议使用 `twnode` 命令管理。

### Q: 删除隧道会删除保护路由吗？

A: 会。`line delete` 命令会执行撤销文件，删除保护路由。

### Q: 保护路由丢失怎么办？

A: 执行同步命令：

```bash
sudo twnode policy sync-protection
```

或重新启动隧道：

```bash
sudo twnode line stop tunnel_hk
sudo twnode line start tunnel_hk
```

### Q: 如何禁用保护路由？

A: 不建议禁用。如果确实需要，可以手动删除规则：

```bash
# 查看保护路由
ip rule show pref 10

# 手动删除
sudo ip rule del to 203.0.113.50 pref 10
```

**警告**: 禁用保护路由可能导致路由环路，隧道无法正常工作。

## 下一步

- [路由表设计](routing-tables.md) - 路由表和优先级系统
- [sync-protection 命令](../commands/policy/sync-protection.md) - 命令详解
- [动态 IP 教程](../tutorials/dynamic-ip.md) - 动态 IP 场景配置

---

**导航**: [← 路由表设计](routing-tables.md) | [返回首页](../index.md) | [故障排查 →](troubleshooting.md)
