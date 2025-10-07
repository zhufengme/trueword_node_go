# WireGuard 隧道完整配置教程

本教程将指导你完成 WireGuard 隧道的完整配置，包括服务器/客户端设置、测试验证和故障排查。

## 场景说明

假设你有两台服务器需要建立 WireGuard 隧道：

- **服务器 A**（香港）
  - 公网 IP: `203.0.113.50`
  - 物理接口: `eth0 = 192.168.1.100`
  - 虚拟 IP: `10.0.0.1`

- **服务器 B**（台湾）
  - 公网 IP: `198.51.100.20`（可能是动态 IP）
  - 物理接口: `eth0 = 10.0.0.50`
  - 虚拟 IP: `10.0.0.2`

**目标**: 在两台服务器之间建立安全的 WireGuard 隧道。

## 前置要求

### 系统要求

- Linux 内核 5.6+ （内置 WireGuard 支持）
- 或者安装 `wireguard-tools` 包

**检查内核版本**:
```bash
uname -r
# 应该 >= 5.6
```

**安装 wireguard-tools**（如果需要）:
```bash
# Ubuntu/Debian
sudo apt install wireguard-tools

# CentOS/RHEL
sudo yum install wireguard-tools

# Arch Linux
sudo pacman -S wireguard-tools
```

### TrueWord Node 已初始化

```bash
sudo twnode init
```

详见 [快速入门](../getting-started.md#系统初始化)

## 步骤 1: 服务器 A（香港）- 创建服务器端隧道

### 1.1 创建 WireGuard 服务器隧道

```bash
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 hk-tw \
  --type wireguard \
  --mode server \
  --listen-port 51820
```

**参数说明**:
- `eth0` - 父接口（物理接口）
- `0.0.0.0` - 对端 IP 占位符（服务器模式不知道客户端 IP）
- `10.0.0.2` - 对端虚拟 IP
- `10.0.0.1` - 本地虚拟 IP
- `hk-tw` - 隧道名称
- `--listen-port 51820` - 监听端口

### 1.2 查看输出

命令执行后会输出完整的对端配置命令：

```
✓ 已创建 WireGuard 隧道: hk-tw

【对端配置命令】
在远程服务器上运行以下命令创建对应的隧道:

sudo twnode line create eth0 203.0.113.50 10.0.0.1 10.0.0.2 tw-hk \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820

💡 对端配置已保存到: /var/lib/trueword_node/peer_configs/hk-tw.txt

提示: 使用 'twnode line show-peer hk-tw' 可再次查看对端配置
```

**复制这个命令**，稍后在服务器 B 上使用。

### 1.3 配置防火墙

```bash
# 允许 WireGuard 端口
sudo iptables -A INPUT -p udp --dport 51820 -j ACCEPT

# 保存规则（Ubuntu/Debian）
sudo netfilter-persistent save

# 或者（CentOS/RHEL）
sudo service iptables save
```

### 1.4 启动隧道

```bash
sudo twnode line start hk-tw
```

**输出**:
```
✓ 隧道 hk-tw 已启动
✓ 保护路由已同步
```

## 步骤 2: 服务器 B（台湾）- 创建客户端隧道

### 2.1 使用服务器 A 输出的命令

将服务器 A 输出的完整命令粘贴到服务器 B，**只需替换父接口**（如果不同）：

```bash
sudo twnode line create eth0 203.0.113.50 10.0.0.1 10.0.0.2 tw-hk \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820
```

**输出**:
```
✓ 已创建 WireGuard 隧道: tw-hk
✓ 配置已保存到 /etc/trueword_node/tunnels/tw-hk.yaml
```

### 2.2 启动隧道

```bash
sudo twnode line start tw-hk
```

**输出**:
```
✓ 隧道 tw-hk 已启动
✓ 主动触发 WireGuard 握手...
✓ WireGuard 握手成功
✓ 保护路由已同步
```

> 💡 **提示**: 客户端会主动发送 ping 包触发 WireGuard 握手，通常需要 5-10 秒。

## 步骤 3: 测试连通性

### 3.1 基本 Ping 测试

**在服务器 A 上**:
```bash
ping 10.0.0.2
```

**在服务器 B 上**:
```bash
ping 10.0.0.1
```

如果能 ping 通，恭喜！隧道已成功建立 🎉

### 3.2 检查 WireGuard 状态

**在服务器 A 上**:
```bash
sudo wg show hk-tw
```

**输出示例**:
```
interface: hk-tw
  public key: xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=
  private key: (hidden)
  listening port: 51820

peer: aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=
  endpoint: 198.51.100.20:51234  ← 客户端实际 IP 和端口
  allowed ips: 10.0.0.2/32
  latest handshake: 15 seconds ago
  transfer: 1.2 KiB received, 892 B sent
```

### 3.3 使用 TrueWord Node 检查

```bash
sudo twnode line check hk-tw 8.8.8.8
```

**输出示例**:
```
【连通性检查结果】
接口名称: hk-tw
测试地址: 8.8.8.8
丢包率: 0%
平均延迟: 25.3 ms
评分: 94.5 分
状态: ✓ 良好
```

## 步骤 4: 配置策略路由（可选）

### 4.1 创建策略组

在服务器 A 上，假设你想让某些流量通过隧道转发：

```bash
sudo twnode policy create tw_traffic hk-tw
```

### 4.2 添加路由规则

```bash
# 添加单个 IP
sudo twnode policy add-cidr tw_traffic 192.168.100.5/32

# 添加 IP 段
sudo twnode policy add-cidr tw_traffic 192.168.100.0/24
```

### 4.3 应用策略

```bash
sudo twnode policy apply tw_traffic
```

### 4.4 验证

```bash
# 查看策略组
sudo twnode policy list

# 查看路由规则
sudo ip rule show
```

## 步骤 5: 动态 IP 场景配置

如果服务器 B 的 IP 是动态的（如家庭宽带、移动网络），需要配置保护路由同步。

### 5.1 配置 Cron 定时任务

**在服务器 A 上**:

```bash
crontab -e
```

添加以下行：

```bash
# 每 5 分钟同步保护路由
*/5 * * * * /usr/local/bin/twnode policy sync-protection >/dev/null 2>&1
```

### 5.2 测试同步

手动执行同步：

```bash
sudo twnode policy sync-protection
```

**输出示例**（IP 未变化）:
```
同步保护路由...
  ✓ 保护 WireGuard 服务器 hk-tw 的远程IP 198.51.100.20
  未检测到 IP 变化，无需更新
✓ 保护路由同步完成
```

**输出示例**（IP 已变化）:
```
同步保护路由...
  ℹ 从运行状态检测到 WireGuard 隧道 hk-tw 的对端IP: 198.51.100.50
  ⚠ WireGuard 隧道 hk-tw 对端IP已变化: 198.51.100.20 → 198.51.100.50
  ✓ 已删除旧保护路由: 198.51.100.20
  ✓ 已添加新保护路由: 198.51.100.50
  ✓ 已更新配置文件: ProtectedIP = 198.51.100.50
  已更新 1 个隧道的保护路由
✓ 保护路由同步完成
```

详见 [保护路由同步](../commands/policy/sync-protection.md)

## 故障排查

### 问题 1: 无法 Ping 通

**检查步骤**:

1. **确认隧道已启动**:
   ```bash
   sudo twnode line list
   ```
   状态应该是 "Active"。

2. **检查 WireGuard 握手**:
   ```bash
   sudo wg show <interface>
   ```
   查看 "latest handshake" 是否在最近（< 3 分钟）。

3. **检查防火墙**:
   ```bash
   sudo iptables -L -v -n | grep 51820
   ```
   确保允许 UDP 51820 端口。

4. **检查路由规则**:
   ```bash
   sudo ip route get 10.0.0.2
   ```
   应该显示通过隧道接口。

### 问题 2: WireGuard 握手失败

**原因**:
- 密钥不匹配
- 防火墙阻止
- 网络不通

**解决方案**:

```bash
# 1. 检查密钥是否正确
sudo wg show <interface>
# 对比配置文件中的 private_key 和 peer_pubkey

# 2. 手动触发握手（客户端）
sudo ping -c 5 -I <interface> <对端VIP>

# 3. 检查网络连通性（客户端）
ping <服务器IP>

# 4. 检查防火墙（服务器）
sudo iptables -L -v -n
```

### 问题 3: 动态 IP 变化后无法连接

**原因**:
- 保护路由未更新
- 同步间隔太长

**解决方案**:

```bash
# 立即同步保护路由
sudo twnode policy sync-protection

# 检查保护路由规则
sudo ip rule show pref 10

# 缩短同步间隔（改为每分钟）
*/1 * * * * /usr/local/bin/twnode policy sync-protection
```

### 问题 4: 性能问题

**检查 MTU**:

WireGuard 建议 MTU 为 1420（以太网 1500 - WireGuard 开销 80）。

```bash
# 设置 MTU
sudo ip link set <interface> mtu 1420

# 验证
ip link show <interface>
```

## 高级配置

### 多层隧道嵌套

基于 WireGuard 隧道创建第二层隧道：

```bash
# 第一层：物理接口 → WireGuard
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 hk-tw \
  --type wireguard --mode server --listen-port 51820

# 第二层：基于第一层隧道 → GRE
sudo twnode line create hk-tw 10.0.0.2 172.16.0.2 172.16.0.1 layer2 \
  --auth-key 0x1234 --enc-key 0x5678

# 启动所有隧道
sudo twnode line start-all
```

### 配合故障转移

```bash
# 创建第二条备用隧道
sudo twnode line create eth0 0.0.0.0 10.0.1.2 10.0.1.1 hk-tw-backup \
  --type wireguard --mode server --listen-port 51821

# 定期检查并故障转移
*/10 * * * * /usr/local/bin/twnode line check hk-tw 8.8.8.8
*/15 * * * * /usr/local/bin/twnode policy failover tw_traffic hk-tw,hk-tw-backup
```

## 下一步

- [策略路由配置](policy-routing.md) - 灵活的流量控制
- [故障转移配置](failover-setup.md) - 高可用方案
- [动态 IP 处理](dynamic-ip.md) - 深入了解动态 IP 场景

---

**导航**: [← 教程](../index.md#实战教程) | [返回首页](../index.md) | [策略路由 →](policy-routing.md)
