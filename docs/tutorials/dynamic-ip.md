# 动态 IP 场景处理

本教程介绍如何在动态 IP 场景下配置和管理 WireGuard 隧道。

## 场景说明

**典型场景**: WireGuard 服务器接收动态 IP 客户端

- **服务器端**（固定 IP）: 云服务器、VPS、企业服务器
- **客户端**（动态 IP）: 家庭宽带、移动网络、DHCP 环境

**挑战**:
- 客户端 IP 地址可能变化（网络切换、DHCP 更新、运营商重拨）
- 服务器端的保护路由需要动态更新
- 策略路由可能受影响

## 解决方案

TrueWord Node 通过**保护路由同步**机制自动处理动态 IP：

1. WireGuard 服务器从运行状态获取实际对端 IP
2. 定时检测 IP 变化
3. 自动更新保护路由和配置文件

## 配置步骤

### 服务器端配置

#### 1. 创建 WireGuard 服务器隧道

```bash
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 mobile_client \
  --type wireguard \
  --mode server \
  --listen-port 51820
```

**关键参数**:
- `0.0.0.0` - 对端 IP 占位符（服务器模式不知道客户端 IP）
- `--mode server` - 服务器模式

#### 2. 配置防火墙

```bash
# 允许 WireGuard 端口
sudo iptables -A INPUT -p udp --dport 51820 -j ACCEPT

# 保存规则
sudo netfilter-persistent save  # Ubuntu/Debian
```

#### 3. 启动隧道

```bash
sudo twnode line start mobile_client
```

#### 4. 获取对端配置命令

```bash
sudo twnode line show-peer mobile_client
```

复制输出的完整命令，发送给客户端管理员。

#### 5. 配置保护路由自动同步

```bash
# 编辑 crontab
crontab -e

# 添加定时任务（每 5 分钟同步一次）
*/5 * * * * /usr/local/bin/twnode policy sync-protection >/dev/null 2>&1
```

### 客户端配置

#### 1. 使用服务器提供的命令

```bash
# 替换 <父接口> 为实际接口（如 wlan0, eth0）
sudo twnode line create wlan0 服务器IP 10.0.0.1 10.0.0.2 home_vpn \
  --type wireguard \
  --mode client \
  --private-key '私钥' \
  --peer-pubkey '服务器公钥' \
  --peer-port 51820
```

#### 2. 启动隧道

```bash
sudo twnode line start home_vpn
```

#### 3. 测试连通性

```bash
ping 10.0.0.1
```

## IP 变化处理

### 客户端 IP 变化

当客户端 IP 从 `103.118.40.121` 变为 `103.118.50.200`：

#### 客户端无需操作

客户端的配置不需要修改，WireGuard 会自动处理：

```
1. 客户端网络变化（新 IP: 103.118.50.200）
2. 客户端发送数据包到服务器
3. WireGuard 自动更新 endpoint
4. 隧道继续正常工作
```

#### 服务器自动更新

定时任务自动执行 `sync-protection`：

```bash
$ sudo twnode policy sync-protection

同步保护路由...
  ℹ 从运行状态检测到 WireGuard 隧道 mobile_client 的对端IP: 103.118.50.200
  ⚠ WireGuard 隧道 mobile_client 对端IP已变化: 103.118.40.121 → 103.118.50.200
  ✓ 已删除旧保护路由: 103.118.40.121
  ✓ 已添加新保护路由: 103.118.50.200
  ✓ 已更新配置文件: ProtectedIP = 103.118.50.200
✓ 保护路由同步完成
```

**内部流程**:

```
1. 执行: wg show mobile_client endpoints
   输出: aB3cD...= 103.118.50.200:51820

2. 读取配置文件: ProtectedIP = 103.118.40.121

3. 检测到变化: 103.118.40.121 → 103.118.50.200

4. 更新保护路由:
   - 删除: ip rule del to 103.118.40.121 pref 10
   - 添加: ip rule add to 103.118.50.200 lookup main pref 10

5. 更新配置文件: ProtectedIP = 103.118.50.200
```

## 验证和监控

### 验证保护路由

```bash
# 查看保护路由
sudo ip rule show pref 10

# 应该显示当前客户端 IP
10: from all to 103.118.50.200 lookup main
```

### 查看 WireGuard 状态

```bash
sudo wg show mobile_client

# 输出:
interface: mobile_client
  ...
peer: aB3cD...
  endpoint: 103.118.50.200:51820  # 当前客户端 IP
  latest handshake: 30 seconds ago
  transfer: 15.2 KB received, 8.3 KB sent
```

### 检查配置文件

```bash
cat /etc/trueword_node/tunnels/mobile_client.yaml | grep protected_ip

# 输出:
protected_ip: 103.118.50.200
```

### 监控 IP 变化

查看同步日志：

```bash
# 查看 cron 日志
grep "twnode policy sync-protection" /var/log/syslog

# 或启用详细日志
*/5 * * * * /usr/local/bin/twnode policy sync-protection >> /var/log/twnode_sync.log 2>&1

# 查看 IP 变化历史
grep "对端IP已变化" /var/log/twnode_sync.log
```

## 故障处理

### 客户端重新连接失败

**症状**: 客户端 IP 变化后，无法重新连接。

**可能原因**:
1. 服务器保护路由未更新
2. 防火墙阻止新 IP
3. WireGuard 握手超时

**解决方案**:

```bash
# 服务器端

# 1. 手动同步保护路由
sudo twnode policy sync-protection

# 2. 检查防火墙
sudo iptables -L -v -n | grep 51820

# 3. 重启隧道
sudo twnode line stop mobile_client
sudo twnode line start mobile_client

# 客户端

# 1. 重启隧道
sudo twnode line stop home_vpn
sudo twnode line start home_vpn

# 2. 手动触发握手
ping -c 5 10.0.0.1
```

### 同步任务未执行

**症状**: IP 变化后，保护路由未更新。

**诊断**:

```bash
# 检查 cron 任务
crontab -l | grep sync-protection

# 检查 cron 服务状态
sudo systemctl status cron

# 查看 cron 日志
grep CRON /var/log/syslog
```

**解决方案**:

```bash
# 确保 cron 服务运行
sudo systemctl start cron
sudo systemctl enable cron

# 手动测试同步命令
sudo /usr/local/bin/twnode policy sync-protection

# 重新配置 crontab
crontab -e
```

## 高级场景

### 多个动态 IP 客户端

服务器接收多个动态 IP 客户端：

```bash
# 服务器端创建多个隧道
sudo twnode line create eth0 0.0.0.0 10.0.1.2 10.0.1.1 client1 \
  --type wireguard --mode server --listen-port 51821

sudo twnode line create eth0 0.0.0.0 10.0.2.2 10.0.2.1 client2 \
  --type wireguard --mode server --listen-port 51822

sudo twnode line create eth0 0.0.0.0 10.0.3.2 10.0.3.1 client3 \
  --type wireguard --mode server --listen-port 51823

# 启动所有隧道
sudo twnode line start-all

# 同步命令会处理所有隧道
sudo twnode policy sync-protection
```

### 客户端作为路由器

客户端将 VPN 隧道共享给局域网设备：

```bash
# 客户端配置

# 1. 启用 IP 转发
sudo sysctl -w net.ipv4.ip_forward=1

# 2. 配置 NAT
sudo iptables -t nat -A POSTROUTING -o home_vpn -j MASQUERADE

# 3. 配置默认路由
sudo twnode policy set-default home_vpn

# 局域网设备将网关设置为客户端 IP
```

### 带宽限制和优先级

对动态客户端进行带宽管理：

```bash
# 使用 tc 限制带宽
sudo tc qdisc add dev mobile_client root tbf rate 10mbit burst 32kbit latency 400ms

# 或使用 wondershaper
sudo wondershaper mobile_client 10000 10000
```

## 最佳实践

### 1. 同步间隔

- **推荐**: 5 分钟
- **最短**: 1 分钟（高频变化场景）
- **最长**: 15 分钟（稳定场景）

### 2. 日志记录

```bash
# 记录详细日志
*/5 * * * * /usr/local/bin/twnode policy sync-protection >> /var/log/twnode_sync.log 2>&1

# 日志轮转
sudo nano /etc/logrotate.d/twnode

# 添加:
/var/log/twnode_sync.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
}
```

### 3. 告警机制

```bash
#!/bin/bash
# /usr/local/bin/sync_and_alert.sh

LOG="/var/log/twnode_sync.log"
/usr/local/bin/twnode policy sync-protection >> $LOG 2>&1

# 检查是否有 IP 变化
if tail -10 $LOG | grep -q "对端IP已变化"; then
    CHANGE=$(tail -10 $LOG | grep "对端IP已变化")
    echo "$CHANGE" | mail -s "WireGuard IP Changed" admin@example.com
fi
```

### 4. 备用隧道

配置备用隧道应对主隧道失败：

```bash
# 创建备用隧道
sudo twnode line create eth0 0.0.0.0 10.0.10.2 10.0.10.1 mobile_backup \
  --type wireguard --mode server --listen-port 51830

# 配置故障转移
*/15 * * * * /usr/local/bin/twnode policy failover vpn_traffic mobile_client,mobile_backup
```

## 下一步

- [保护路由同步](../commands/policy/sync-protection.md) - 命令详解
- [保护路由机制](../reference/protection-routes.md) - 技术细节
- [WireGuard 配置](wireguard-setup.md) - 完整配置流程

---

**导航**: [← 教程](../index.md#实战教程) | [返回首页](../index.md)
