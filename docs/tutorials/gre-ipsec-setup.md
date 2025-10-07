# GRE over IPsec 隧道完整配置教程

本教程将指导你在两台 Linux 服务器之间建立 **GRE over IPsec** 加密隧道。

## 教程目标

完成本教程后，你将学会：

- 在两台服务器上配置 GRE over IPsec 隧道
- 理解双层隧道架构（IPsec ESP + GRE）
- 验证隧道连通性和加密状态
- 配置基于隧道的策略路由

## 前置条件

### 服务器要求

- 两台 Linux 服务器（Ubuntu 20.04+ 或 CentOS 7+）
- 内核支持 GRE 和 XFRM
- Root 权限

### 网络拓扑

```
服务器 A (香港)                    服务器 B (台湾)
公网 IP: 203.0.113.10            公网 IP: 198.51.100.20
接口: eth0                       接口: eth0

         IPsec ESP 隧道 (加密层)
  203.0.113.10 ←─────────────→ 198.51.100.20
                   加密传输

         GRE 隧道 (数据传输层)
      10.0.0.1 ←──────────→ 10.0.0.2
     (虚拟IP)               (虚拟IP)
```

## 步骤1: 初始化系统（两台服务器）

在**两台服务器**上执行相同的初始化操作：

```bash
# 1. 安装 TrueWord Node（如果未安装）
cd /path/to/trueword_node_go
make static
sudo make install

# 2. 检查命令是否可用
twnode --version

# 3. 初始化系统
sudo twnode init
```

### 初始化输出

```
【系统初始化】

检查环境...
✓ Root 权限
✓ 必需命令: ip, iptables, ping, sysctl

系统配置...
✓ IP 转发已启用
✓ iptables MASQUERADE 已配置

扫描网络接口...

可用接口:
1. eth0
   IP: 203.0.113.10
   网关: 203.0.113.1
   状态: UP

选择要管理的接口 (输入编号，多个用逗号分隔): 1

✓ 已保存接口配置

✓ 系统初始化完成
```

## 步骤2: 创建隧道（服务器 A - 香港）

在**服务器 A** 上创建到服务器 B 的隧道：

### 交互式创建

```bash
sudo twnode line create
```

### 交互过程

```
【创建隧道】

可用父接口:
1. eth0 (203.0.113.10) - 物理接口

选择父接口 (输入编号): 1

输入隧道名称: tun_hk_tw
输入对端 IP: 198.51.100.20
输入对端虚拟 IP: 10.0.0.2
输入本地虚拟 IP: 10.0.0.1
输入认证密钥: 0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f
输入加密密钥: 0x9f8e7d6c5b4a39281716050403020100aabbccddeeff0011
是否启用加密? (yes/no): yes

【配置信息】
父接口: eth0
本地 IP: 203.0.113.10 (从父接口自动获取)
对端 IP: 198.51.100.20
本地虚拟 IP: 10.0.0.1
对端虚拟 IP: 10.0.0.2
GRE Key: 2856 (从认证密钥生成)

【建立连接】
✓ 添加策略路由 (表 50)
✓ 创建 IPsec ESP 隧道
✓ 创建 GRE 隧道
✓ 配置虚拟 IP
✓ 启动接口

✓ 隧道 tun_hk_tw 已创建并启动
```

### 或使用命令行模式

```bash
sudo twnode line create eth0 198.51.100.20 10.0.0.2 10.0.0.1 tun_hk_tw \
  --auth-key "0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f" \
  --enc-key "0x9f8e7d6c5b4a39281716050403020100aabbccddeeff0011" \
  --encrypt
```

## 步骤3: 创建隧道（服务器 B - 台湾）

在**服务器 B** 上创建对应的隧道：

**重要**: 参数要对称！

```bash
sudo twnode line create eth0 203.0.113.10 10.0.0.1 10.0.0.2 tun_hk_tw \
  --auth-key "0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f" \
  --enc-key "0x9f8e7d6c5b4a39281716050403020100aabbccddeeff0011" \
  --encrypt
```

**参数对应关系**:

| 参数 | 服务器 A | 服务器 B |
|------|---------|---------|
| 父接口 | eth0 | eth0 |
| 对端 IP | 198.51.100.20 | 203.0.113.10 |
| 对端虚拟 IP | 10.0.0.2 | 10.0.0.1 |
| 本地虚拟 IP | 10.0.0.1 | 10.0.0.2 |
| 认证密钥 | 相同 | 相同 |
| 加密密钥 | 相同 | 相同 |

## 步骤4: 验证隧道状态

### 检查隧道列表

```bash
sudo twnode line list
```

输出：

```
+-------------+---------------+--------------+-----------------+----------------+-------------+
| 隧道名称    | 父接口        | 类型         | 对端IP          | 本地VIP        | 状态        |
+-------------+---------------+--------------+-----------------+----------------+-------------+
| tun_hk_tw   | eth0          | GRE+IPsec    | 198.51.100.20   | 10.0.0.1       | ✓ Active    |
+-------------+---------------+--------------+-----------------+----------------+-------------+
```

### Ping 测试

在**服务器 A** 上 ping 服务器 B 的虚拟 IP：

```bash
ping -c 4 10.0.0.2
```

输出：

```
PING 10.0.0.2 (10.0.0.2) 56(84) bytes of data.
64 bytes from 10.0.0.2: icmp_seq=1 ttl=64 time=12.3 ms
64 bytes from 10.0.0.2: icmp_seq=2 ttl=64 time=11.8 ms
64 bytes from 10.0.0.2: icmp_seq=3 ttl=64 time=12.0 ms
64 bytes from 10.0.0.2: icmp_seq=4 ttl=64 time=12.2 ms

--- 10.0.0.2 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3004ms
rtt min/avg/max/mdev = 11.823/12.075/12.315/0.191 ms
```

### 检查 IPsec 状态

```bash
sudo ip xfrm state
```

输出（应该看到两个 state，in 和 out）：

```
src 203.0.113.10 dst 198.51.100.20
    proto esp spi 0x1a2b3c4d reqid 1 mode tunnel
    auth-trunc hmac(sha256) 0x... 128
    enc cbc(aes) 0x...

src 198.51.100.20 dst 203.0.113.10
    proto esp spi 0x1a2b3c4d reqid 1 mode tunnel
    auth-trunc hmac(sha256) 0x... 128
    enc cbc(aes) 0x...
```

### 检查 GRE 接口

```bash
ip link show tun_hk_tw
```

输出：

```
5: tun_hk_tw@NONE: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1476 qdisc noqueue state UNKNOWN
    link/gre 203.0.113.10 peer 198.51.100.20
```

### 检查虚拟 IP

```bash
ip addr show tun_hk_tw
```

输出：

```
5: tun_hk_tw@NONE: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1476 qdisc noqueue state UNKNOWN
    link/gre 203.0.113.10 peer 198.51.100.20
    inet 10.0.0.1 peer 10.0.0.2/32 scope global tun_hk_tw
```

## 步骤5: 连通性检查

使用内置的连通性检查工具：

```bash
sudo twnode line check tun_hk_tw 8.8.8.8
```

输出：

```
检查接口 tun_hk_tw 到 8.8.8.8 的连通性...

添加临时测试路由 (优先级 5)...
执行 Ping 测试 (20 个包)...
✓ Ping 成功

【测试结果】
丢包率: 0.0%
平均延迟: 12.5 ms
评分: 96.5 分

清理临时路由...
✓ 连通性检查完成
```

## 步骤6: 配置策略路由

现在可以通过隧道路由流量了。

### 场景：将特定网段流量路由到隧道

假设要将访问 `192.168.100.0/24` 的流量通过隧道发送：

```bash
# 1. 创建策略组
sudo twnode policy create asia_traffic tun_hk_tw --priority 100

# 2. 添加 CIDR
sudo twnode policy add-cidr asia_traffic 192.168.100.0/24

# 3. 应用策略
sudo twnode policy apply asia_traffic
```

输出：

```
应用策略组: asia_traffic

【策略组信息】
名称: asia_traffic
出口接口: tun_hk_tw
优先级: 100
CIDR 数量: 1

检查出口接口...
✓ 接口 tun_hk_tw 已启动

同步保护路由...
✓ 保护 GRE 隧道 tun_hk_tw 的远程IP 198.51.100.20
✓ 保护路由同步完成

添加路由规则...
✓ 192.168.100.0/24 → tun_hk_tw (pref 100)

✓ 策略组 asia_traffic 已应用
```

### 验证策略路由

```bash
# 查看路由规则
ip rule show pref 100

# 输出
100:    from all to 192.168.100.0/24 lookup 50

# 测试路由
ip route get 192.168.100.5

# 输出（流量会走隧道）
192.168.100.5 dev tun_hk_tw src 10.0.0.1
```

## 步骤7: 设置默认路由（可选）

如果要让**所有流量**默认走隧道：

```bash
sudo twnode policy set-default tun_hk_tw
```

输出：

```
【设置默认路由】
出口接口: tun_hk_tw
优先级: 900
CIDR: 0.0.0.0/0

检查接口状态...
✓ 接口 tun_hk_tw 存在且已启动

添加保护路由...
✓ 已添加保护路由（优先级 10）

应用默认路由...
✓ 默认路由规则已添加: 0.0.0.0/0 → tun_hk_tw (pref 900)

✓ 默认路由已设置
```

### 测试默认路由

```bash
# 测试任意公网 IP
ip route get 8.8.8.8

# 输出（流量走隧道）
8.8.8.8 dev tun_hk_tw src 10.0.0.1
```

## 双层隧道架构理解

### 第一层：IPsec ESP 隧道（加密层）

```
功能: 提供加密和认证
协议: ESP (Encapsulating Security Payload)
工作方式:
  - 原始数据包被加密
  - 添加 ESP 头部
  - 封装在新的 IP 包中（203.0.113.10 → 198.51.100.20）
```

### 第二层：GRE 隧道（数据传输层）

```
功能: 提供虚拟点对点链路
协议: GRE (Generic Routing Encapsulation)
工作方式:
  - 创建虚拟网络接口（tun_hk_tw）
  - 分配虚拟 IP（10.0.0.1 ↔ 10.0.0.2）
  - 封装数据包并通过 IPsec 隧道传输
```

### 数据包封装流程

```
应用数据
  ↓
IP 包 (dst: 192.168.100.5)
  ↓
GRE 封装 (src: 10.0.0.1, dst: 10.0.0.2)
  ↓
IPsec ESP 加密
  ↓
新 IP 包 (src: 203.0.113.10, dst: 198.51.100.20)
  ↓
通过物理网络传输
```

## 密钥管理最佳实践

### 生成强密钥

```bash
# 认证密钥（192 位 = 48 个十六进制字符）
openssl rand -hex 24
# 输出: 1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f

# 加密密钥（256 位 = 64 个十六进制字符）
openssl rand -hex 32
# 输出: 9f8e7d6c5b4a39281716050403020100aabbccddeeff00112233445566778899
```

### 密钥安全存储

```bash
# 密钥保存在配置文件中（仅 root 可读）
ls -la /etc/trueword_node/tunnels/

# 输出
-rw------- 1 root root 412 Jan 15 10:30 tun_hk_tw.yaml
```

### 定期更换密钥

```bash
# 1. 停止隧道
sudo twnode line stop tun_hk_tw

# 2. 删除隧道
sudo twnode line delete tun_hk_tw

# 3. 生成新密钥
AUTH_KEY=$(openssl rand -hex 24)
ENC_KEY=$(openssl rand -hex 32)

# 4. 在两台服务器上重新创建隧道（使用新密钥）
sudo twnode line create eth0 198.51.100.20 10.0.0.2 10.0.0.1 tun_hk_tw \
  --auth-key "0x$AUTH_KEY" \
  --enc-key "0x$ENC_KEY" \
  --encrypt
```

## 故障排除

### 问题1: Ping 不通虚拟 IP

**检查步骤**:

```bash
# 1. 检查隧道状态
sudo twnode line list

# 2. 检查 GRE 接口是否 UP
ip link show tun_hk_tw

# 3. 检查 IPsec state
sudo ip xfrm state

# 4. 检查对端 IP 是否可达
ping -c 4 198.51.100.20
```

**可能原因**:
- 防火墙阻止 ESP 协议（协议号 50）或 GRE 协议（协议号 47）
- 对端服务器未启动隧道
- 密钥不匹配

**解决方案**:

```bash
# 允许 ESP 和 GRE 通过防火墙
sudo iptables -A INPUT -p esp -j ACCEPT
sudo iptables -A INPUT -p gre -j ACCEPT
```

### 问题2: 策略路由不生效

**检查步骤**:

```bash
# 1. 检查路由规则
ip rule show

# 2. 检查路由表 50
ip route show table 50

# 3. 测试路由
ip route get 192.168.100.5
```

**解决方案**:

```bash
# 重新应用策略
sudo twnode policy apply asia_traffic
```

### 问题3: 路由环路

**症状**: 隧道无法建立，或间歇性断开

**原因**: 隧道对端 IP 的流量也被路由到了隧道（形成环路）

**解决方案**:

```bash
# 同步保护路由（自动添加优先级 10 的规则）
sudo twnode policy sync-protection

# 验证保护路由
ip rule show pref 10

# 应该看到
10:     from all to 198.51.100.20 lookup main
```

## 性能优化

### MTU 调整

GRE over IPsec 的开销：
- IPsec ESP 头部：~50 字节
- GRE 头部：~24 字节
- 总开销：~74 字节

推荐 MTU：

```bash
# 物理接口 MTU 1500，隧道 MTU 应设置为 1426
sudo ip link set tun_hk_tw mtu 1426
```

### 开启硬件加速（如果支持）

```bash
# 检查网卡是否支持 ESP offload
ethtool -k eth0 | grep esp

# 如果支持，启用
sudo ethtool -K eth0 esp-hw-offload on
```

## 下一步

- [配置 WireGuard 隧道](wireguard-setup.md) - 现代化的替代方案
- [策略路由实践](policy-routing.md) - 高级路由场景
- [嵌套隧道](nested-tunnels.md) - 多层隧道架构

---

**导航**: [← WireGuard 配置](wireguard-setup.md) | [返回首页](../index.md) | [策略路由实践 →](policy-routing.md)
