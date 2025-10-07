# policy sync-protection - 同步保护路由

## 概述

`policy sync-protection` 命令用于同步保护路由，自动检测隧道对端 IP 变化并更新保护路由规则。这是**动态 IP 容错机制**的核心功能。

## 语法

```bash
sudo twnode policy sync-protection
```

## 功能说明

### 什么是保护路由？

保护路由确保隧道对端 IP 的流量不走策略路由，防止路由环路。

**问题场景**:
```
1. 创建 WireGuard 隧道: eth0 → 203.0.113.50
2. 设置策略路由: 所有流量 → WireGuard 隧道
3. WireGuard 握手包目标是 203.0.113.50
4. 如果握手包走策略路由，会进入隧道
5. 但握手包本身需要通过 eth0 发送到 203.0.113.50
6. 结果: 路由环路，隧道无法建立
```

**解决方案**:
```bash
# 保护路由规则（优先级 10，高于用户策略）
ip rule add to 203.0.113.50 lookup main pref 10
```

这样，发往 `203.0.113.50` 的流量会优先匹配保护路由，通过主路由表（即 eth0）发送，不会进入隧道。

详见 [保护路由机制](../../reference/protection-routes.md)

### 为什么需要同步？

在动态 IP 场景下，对端 IP 可能变化：

- **WireGuard 服务器**接收**动态 IP 客户端**
- 客户端 IP 从 `1.2.3.4` 变为 `103.118.40.121`
- 配置文件中的 `ProtectedIP` 仍然是 `1.2.3.4`
- 保护路由规则仍然是 `ip rule add to 1.2.3.4 lookup main pref 10`
- 新的对端 IP `103.118.40.121` 没有保护路由，可能导致路由环路

**同步解决方案**:
- 检测对端 IP 变化
- 删除旧保护路由
- 添加新保护路由
- 更新配置文件中的 `ProtectedIP`

## 工作流程

```
1. 加载所有隧道配置
   └─ 从 /etc/trueword_node/tunnels/*.yaml

2. 对每个隧道，获取当前对端 IP:
   ├─ GRE 隧道 → 从配置文件读取 RemoteIP
   ├─ WireGuard 客户端 → 从配置文件读取 RemoteIP
   └─ WireGuard 服务器 → 从运行状态获取实际对端 IP
      └─ 执行: wg show <interface> endpoints
      └─ 解析输出: <公钥> <对端IP>:<端口>

3. 检查 ProtectedIP 字段:
   ├─ 如果当前IP == ProtectedIP → 跳过（无需更新）
   ├─ 如果 ProtectedIP 为空 → 添加保护路由
   └─ 如果当前IP != ProtectedIP → 更新保护路由
      ├─ 删除旧规则: ip rule del to <旧IP> pref 10
      ├─ 添加新规则: ip rule add to <新IP> lookup main pref 10
      └─ 更新配置文件: ProtectedIP = <新IP>

4. 扫描所有优先级10的规则:
   ├─ 获取所有规则: ip rule show pref 10
   ├─ 对每个规则，检查是否有对应的隧道
   └─ 如果没有对应隧道 → 删除规则（僵尸规则清理）
```

## WireGuard 对端 IP 检测

### 运行时获取对端 IP

对于 WireGuard 服务器模式，使用 `wg show` 命令获取实际对端 IP：

```bash
$ sudo wg show wg0 endpoints

# 输出示例:
aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=    103.118.40.121:51820
```

**解析逻辑**:
```go
func GetWireGuardPeerEndpoint(interfaceName string) (string, error) {
    cmd := exec.Command("wg", "show", interfaceName, "endpoints")
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

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

### GRE 隧道和 WireGuard 客户端

对于 GRE 隧道和 WireGuard 客户端模式，对端 IP 在配置文件中固定，直接从配置读取。

## 自动同步时机

保护路由同步会在以下时机**自动执行**：

### 1. policy apply 开始前

```bash
sudo twnode policy apply vpn_traffic

# 内部执行流程:
# 1. SyncProtection()  ← 自动同步保护路由
# 2. ApplyGroup("vpn_traffic")
```

### 2. line start 完成后

```bash
sudo twnode line start tunnel_ab

# 内部执行流程:
# 1. StartTunnel("tunnel_ab")
# 2. SyncProtection()  ← 自动同步保护路由
```

### 3. line start-all 完成后

```bash
sudo twnode line start-all

# 内部执行流程:
# 1. StartAllTunnels()
# 2. SyncProtection()  ← 自动同步保护路由
```

### 4. 手动执行

```bash
sudo twnode policy sync-protection
```

### 5. Cron 定时任务（推荐）

```bash
# 编辑 crontab
crontab -e

# 添加定时任务（每 5 分钟同步一次）
*/5 * * * * /usr/local/bin/twnode policy sync-protection >/dev/null 2>&1
```

## 示例

### 示例1: 正常同步（无变化）

```bash
$ sudo twnode policy sync-protection

同步保护路由...
  ✓ 保护 GRE 隧道 tun01 的远程IP 203.0.113.50
  ✓ 保护 WireGuard 客户端 tunnel_ab 的远程IP 192.168.1.100
  ✓ 保护 WireGuard 服务器 tunnel_hk 的远程IP 103.118.40.121
  未检测到 IP 变化，无需更新
✓ 保护路由同步完成
```

### 示例2: 检测到 IP 变化

```bash
$ sudo twnode policy sync-protection

同步保护路由...
  ℹ 从运行状态检测到 WireGuard 隧道 tunnel_hk 的对端IP: 103.118.40.121
  ⚠ WireGuard 隧道 tunnel_hk 对端IP已变化: 1.2.3.4 → 103.118.40.121
  ✓ 已删除旧保护路由: 1.2.3.4
  ✓ 已添加新保护路由: 103.118.40.121
  ✓ 已更新配置文件: ProtectedIP = 103.118.40.121
  ✓ 保护 GRE 隧道 tun01 的远程IP 203.0.113.50
  已更新 1 个隧道的保护路由
✓ 保护路由同步完成
```

### 示例3: 清理僵尸规则

```bash
$ sudo twnode policy sync-protection

同步保护路由...
  ✓ 保护 GRE 隧道 tun01 的远程IP 203.0.113.50
  ✓ 保护 WireGuard 客户端 tunnel_ab 的远程IP 192.168.1.100
  清理 2 个僵尸规则...
  ✓ 已清理僵尸规则: 5.6.7.8
  ✓ 已清理僵尸规则: 9.10.11.12
  已清理 2 个僵尸规则
✓ 保护路由同步完成
```

**僵尸规则**: 系统中存在的优先级 10 保护路由，但没有对应的隧道配置。这可能是因为：
- 隧道已删除，但保护路由未清理
- 手动添加了保护路由
- 配置文件损坏

### 示例4: 添加缺失的保护路由

```bash
$ sudo twnode policy sync-protection

同步保护路由...
  ⚠ WireGuard 隧道 tunnel_new 缺少保护路由
  ✓ 已添加保护路由: 203.0.113.100
  ✓ 已更新配置文件: ProtectedIP = 203.0.113.100
  ✓ 保护 GRE 隧道 tun01 的远程IP 203.0.113.50
  已添加 1 个缺失的保护路由
✓ 保护路由同步完成
```

## 验证保护路由

### 查看所有保护路由规则

```bash
$ sudo ip rule show pref 10

# 输出示例:
10:     from all to 203.0.113.50 lookup main
10:     from all to 192.168.1.100 lookup main
10:     from all to 103.118.40.121 lookup main
```

### 查看隧道配置文件

```bash
$ cat /etc/trueword_node/tunnels/tunnel_hk.yaml

name: tunnel_hk
parent_interface: eth0
tunnel_type: wireguard
local_ip: 192.168.1.100
remote_ip: 0.0.0.0
local_vip: 10.0.0.1
remote_vip: 10.0.0.2
protected_ip: 103.118.40.121  # ← 已更新为新 IP

listen_port: 51820
private_key: xY9zA0...
peer_pubkey: aB3cD4...
```

### 测试路由

```bash
# 测试发往对端 IP 的流量是否走主路由表
$ sudo ip route get 103.118.40.121

# 应该显示通过主路由表（即物理接口）
103.118.40.121 via 192.168.1.1 dev eth0 src 192.168.1.100
```

## 使用场景

### 场景1: WireGuard 服务器接收动态 IP 客户端

**服务器**（固定 IP: 192.168.1.100）:

```bash
# 1. 创建 WireGuard 服务器
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_mobile \
  --type wireguard --mode server --listen-port 51820

# 2. 启动隧道
sudo twnode line start tunnel_mobile

# 3. 配置定时同步（推荐）
echo "*/5 * * * * /usr/local/bin/twnode policy sync-protection" | crontab -

# 4. 创建策略路由
sudo twnode policy create mobile_traffic tunnel_mobile
sudo twnode policy add-cidr mobile_traffic 192.168.100.0/24
sudo twnode policy apply mobile_traffic
```

**客户端**（动态 IP，如移动网络）:

```bash
# 使用服务器输出的配置命令
sudo twnode line create wlan0 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_home \
  --type wireguard --mode client \
  --private-key 'xxx' --peer-pubkey 'yyy' --peer-port 51820

sudo twnode line start tunnel_home
```

**客户端 IP 变化后**（如切换网络）:

```
客户端 IP: 103.118.40.10 → 103.118.50.20

服务器自动处理:
1. Cron 任务每 5 分钟执行 sync-protection
2. 检测到对端 IP 变化: 103.118.40.10 → 103.118.50.20
3. 删除旧保护路由: ip rule del to 103.118.40.10 pref 10
4. 添加新保护路由: ip rule add to 103.118.50.20 lookup main pref 10
5. 更新配置文件: ProtectedIP = 103.118.50.20
6. 隧道继续正常工作
```

### 场景2: 多隧道动态 IP 管理

```bash
# 服务器有多个 WireGuard 隧道接收动态 IP 客户端
sudo twnode line create eth0 0.0.0.0 10.0.1.2 10.0.1.1 tunnel_client1 \
  --type wireguard --mode server --listen-port 51821

sudo twnode line create eth0 0.0.0.0 10.0.2.2 10.0.2.1 tunnel_client2 \
  --type wireguard --mode server --listen-port 51822

sudo twnode line create eth0 0.0.0.0 10.0.3.2 10.0.3.1 tunnel_client3 \
  --type wireguard --mode server --listen-port 51823

# 启动所有隧道（自动同步保护路由）
sudo twnode line start-all

# 定时同步（一次性检查所有隧道）
*/5 * * * * /usr/local/bin/twnode policy sync-protection
```

## 故障排查

### 问题1: 同步后隧道仍然无法连接

**可能原因**:
- 防火墙阻止了新的对端 IP
- 对端 IP 变化过快，同步间隔太长

**解决方案**:
```bash
# 检查防火墙规则
sudo iptables -L -v -n

# 缩短同步间隔（如改为每分钟）
*/1 * * * * /usr/local/bin/twnode policy sync-protection

# 手动立即同步
sudo twnode policy sync-protection
```

### 问题2: WireGuard 服务器检测不到对端 IP

**可能原因**:
- 客户端尚未连接
- WireGuard 握手失败

**解决方案**:
```bash
# 检查 WireGuard 状态
sudo wg show <interface>

# 如果没有 peer endpoint，说明客户端未连接
# 检查客户端日志和网络连通性

# 客户端手动触发握手
sudo ping -c 3 -I <interface> <对端VIP>
```

### 问题3: 僵尸规则无法清理

**可能原因**:
- 规则被其他程序添加
- 规则格式不匹配

**解决方案**:
```bash
# 手动查看所有优先级 10 的规则
sudo ip rule show pref 10

# 手动删除指定规则
sudo ip rule del to <IP> pref 10

# 重新同步
sudo twnode policy sync-protection
```

## 最佳实践

### 1. 定时同步

**推荐间隔**: 5 分钟

```bash
*/5 * * * * /usr/local/bin/twnode policy sync-protection >/dev/null 2>&1
```

**不推荐**:
- 太频繁（如每分钟）：浪费资源
- 太少（如每小时）：IP 变化后恢复时间太长

### 2. 监控和告警

```bash
# 记录同步日志
*/5 * * * * /usr/local/bin/twnode policy sync-protection >> /var/log/twnode_sync.log 2>&1

# 检查日志中的 IP 变化
grep "对端IP已变化" /var/log/twnode_sync.log
```

### 3. 配合故障转移

```bash
# 定期检查隧道连通性
*/10 * * * * /usr/local/bin/twnode line check tunnel_hk 8.8.8.8 >/dev/null 2>&1

# 如果隧道故障，自动切换
*/15 * * * * /usr/local/bin/twnode policy failover mobile_traffic tunnel_hk,tunnel_backup
```

## 下一步

- [故障转移](failover.md) - 智能选择最佳出口
- [保护路由详解](../../reference/protection-routes.md) - 深入了解保护路由机制
- [动态 IP 教程](../../tutorials/dynamic-ip.md) - 完整的动态 IP 场景配置

---

**导航**: [← policy 命令](index.md) | [返回首页](../../index.md) | [failover →](failover.md)
