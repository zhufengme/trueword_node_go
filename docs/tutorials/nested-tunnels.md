# 嵌套隧道配置教程

本教程介绍 TrueWord Node 的核心特性之一：**嵌套隧道**（多层隧道架构）。通过父接口（Parent Interface）概念，你可以在隧道之上创建新的隧道，实现多层加密和复杂的网络拓扑。

## 教程目标

完成本教程后，你将学会：

- 理解父接口（Parent Interface）概念
- 在隧道上创建新隧道
- 配置多层嵌套架构
- 理解路由表和虚拟 IP 的层次关系
- 应用嵌套隧道的实际场景

## 核心概念：父接口

### 什么是父接口？

**父接口**是创建新隧道时的基础网络接口，可以是：

1. **物理网络接口**（如 eth0, ens33）
   - 拥有实际的 IP 地址
   - 拥有网关信息
   - 直接连接到物理网络

2. **已创建的隧道**（如 tun01, tunnel_hk）
   - 拥有虚拟 IP（Local VIP）
   - 没有网关信息
   - 建立在其他接口之上

### 本地 IP 自动获取规则

创建隧道时，**本地 IP 自动从父接口获取**：

```
父接口类型 → 本地 IP 来源

物理接口（eth0）
  ↓
使用物理接口的 IP 地址（如 192.168.1.100）

隧道接口（tun01）
  ↓
使用隧道的 Local VIP（如 10.0.0.1）
```

**用户无需手动输入本地 IP**，只需选择父接口。

## 场景1: 两层隧道架构

### 网络拓扑

```
服务器 A (香港)                    服务器 B (台湾)

物理接口 eth0                      物理接口 eth0
  IP: 203.0.113.10                  IP: 198.51.100.20
       ↓                                  ↓
    第一层隧道 (tunnel_base)
  Local IP: 203.0.113.10          Local IP: 198.51.100.20
  Local VIP: 10.0.0.1              Local VIP: 10.0.0.2
       ↓                                  ↓
    第二层隧道 (tunnel_secure)
  Local IP: 10.0.0.1 (自动)       Local IP: 10.0.0.2 (自动)
  Local VIP: 172.16.0.1            Local VIP: 172.16.0.2
```

### 在服务器 A 上配置

#### 1. 创建第一层隧道（基础隧道）

```bash
# 父接口: eth0（物理接口）
sudo twnode line create eth0 198.51.100.20 10.0.0.2 10.0.0.1 tunnel_base \
  --type wireguard \
  --mode server \
  --listen-port 51820
```

输出：

```
【创建 WireGuard 隧道 - 服务器模式】
父接口: eth0
本地 IP: 203.0.113.10 (从父接口自动获取)
对端 IP: 198.51.100.20
本地虚拟 IP: 10.0.0.1
对端虚拟 IP: 10.0.0.2

生成密钥对...
✓ 本地密钥对已生成
✓ 对端密钥对已生成

【建立连接】
✓ 创建 WireGuard 接口
✓ 配置虚拟 IP
✓ 启动接口
✓ 触发握手

✓ 隧道 tunnel_base 已创建并启动

【对端配置命令】
在对端服务器上执行以下命令（替换 <父接口>）：

sudo twnode line create <父接口> 203.0.113.10 10.0.0.1 10.0.0.2 tunnel_base \
  --type wireguard \
  --mode client \
  --private-key 'xYzAbC123...' \
  --peer-pubkey 'aBcDeF456...' \
  --peer-port 51820
```

#### 2. 创建第二层隧道（安全隧道）

**关键**: 父接口选择 **tunnel_base**（第一层隧道）

```bash
# 父接口: tunnel_base（隧道接口）
# 对端 IP: 10.0.0.2（第一层隧道的对端 VIP）
sudo twnode line create tunnel_base 10.0.0.2 172.16.0.2 172.16.0.1 tunnel_secure \
  --type wireguard \
  --mode server \
  --listen-port 51821
```

输出：

```
【创建 WireGuard 隧道 - 服务器模式】
父接口: tunnel_base (隧道接口)
本地 IP: 10.0.0.1 (从父接口 tunnel_base 的 VIP 自动获取)
对端 IP: 10.0.0.2
本地虚拟 IP: 172.16.0.1
对端虚拟 IP: 172.16.0.2

生成密钥对...
✓ 本地密钥对已生成
✓ 对端密钥对已生成

【建立连接】
✓ 创建 WireGuard 接口
✓ 配置虚拟 IP
✓ 启动接口
✓ 触发握手

✓ 隧道 tunnel_secure 已创建并启动

【对端配置命令】
在对端服务器上执行以下命令（替换 <父接口>）：

sudo twnode line create <父接口> 10.0.0.1 172.16.0.1 172.16.0.2 tunnel_secure \
  --type wireguard \
  --mode client \
  --private-key 'pQrStU789...' \
  --peer-pubkey 'vWxYzA012...' \
  --peer-port 51821
```

### 在服务器 B 上配置

#### 1. 创建第一层隧道（使用服务器 A 提供的命令）

```bash
# 复制服务器 A 输出的对端配置命令，替换 <父接口> 为 eth0
sudo twnode line create eth0 203.0.113.10 10.0.0.1 10.0.0.2 tunnel_base \
  --type wireguard \
  --mode client \
  --private-key 'xYzAbC123...' \
  --peer-pubkey 'aBcDeF456...' \
  --peer-port 51820
```

#### 2. 创建第二层隧道（使用服务器 A 提供的命令）

**关键**: 父接口也选择 **tunnel_base**

```bash
# 复制服务器 A 输出的对端配置命令，替换 <父接口> 为 tunnel_base
sudo twnode line create tunnel_base 10.0.0.1 172.16.0.1 172.16.0.2 tunnel_secure \
  --type wireguard \
  --mode client \
  --private-key 'pQrStU789...' \
  --peer-pubkey 'vWxYzA012...' \
  --peer-port 51821
```

### 验证嵌套隧道

#### 查看隧道列表

```bash
sudo twnode line list
```

输出（服务器 A）：

```
+----------------+---------------+--------------+-----------------+----------------+-------------+
| 隧道名称       | 父接口        | 类型         | 对端IP          | 本地VIP        | 状态        |
+----------------+---------------+--------------+-----------------+----------------+-------------+
| tunnel_base    | eth0          | WireGuard    | 198.51.100.20   | 10.0.0.1       | ✓ Active    |
| tunnel_secure  | tunnel_base   | WireGuard    | 10.0.0.2        | 172.16.0.1     | ✓ Active    |
+----------------+---------------+--------------+-----------------+----------------+-------------+
```

**注意**：
- tunnel_base 的父接口是 eth0（物理接口）
- tunnel_secure 的父接口是 tunnel_base（隧道接口）

#### Ping 测试

在**服务器 A** 上测试：

```bash
# 测试第一层隧道
ping -c 4 10.0.0.2
# 输出: 64 bytes from 10.0.0.2: icmp_seq=1 ttl=64 time=12.3 ms

# 测试第二层隧道
ping -c 4 172.16.0.2
# 输出: 64 bytes from 172.16.0.2: icmp_seq=1 ttl=64 time=13.5 ms
```

#### 检查路由

```bash
# 查看到 10.0.0.2 的路由（第一层隧道）
ip route get 10.0.0.2
# 输出: 10.0.0.2 dev tunnel_base src 10.0.0.1

# 查看到 172.16.0.2 的路由（第二层隧道）
ip route get 172.16.0.2
# 输出: 172.16.0.2 dev tunnel_secure src 172.16.0.1
```

### 数据包封装流程

当发送数据到 `172.16.0.2` 时，数据包经过**双重封装**：

```
应用数据
  ↓
IP 包 (dst: 172.16.0.2)
  ↓
【第二层隧道封装】
WireGuard 加密
虚拟 IP 封装 (src: 172.16.0.1, dst: 172.16.0.2)
  ↓
IP 包 (src: 10.0.0.1, dst: 10.0.0.2) ← 第二层隧道的对端 IP
  ↓
【第一层隧道封装】
WireGuard 加密
虚拟 IP 封装 (src: 10.0.0.1, dst: 10.0.0.2)
  ↓
IP 包 (src: 203.0.113.10, dst: 198.51.100.20) ← 物理 IP
  ↓
通过物理网络传输
```

**双重加密**：数据包经过两次 WireGuard 加密，安全性更高。

## 场景2: 三层隧道架构

### 网络拓扑

```
服务器 A (香港) → 服务器 B (台湾) → 服务器 C (日本)

物理层:
  eth0 (203.0.113.10) ←→ eth0 (198.51.100.20) ←→ eth0 (192.0.2.30)

第一层隧道 (A ↔ B):
  tunnel_ab: 10.0.0.1 ←→ 10.0.0.2

第二层隧道 (B ↔ C):
  tunnel_bc: 10.0.1.1 ←→ 10.0.1.2

第三层隧道 (A ↔ C，通过 B 中转):
  tunnel_ac: 172.16.0.1 ←→ 172.16.0.2
```

### 配置步骤

#### 1. 服务器 A 和 B 之间创建第一层隧道

**服务器 A**：

```bash
sudo twnode line create eth0 198.51.100.20 10.0.0.2 10.0.0.1 tunnel_ab \
  --type wireguard --mode server --listen-port 51820
```

**服务器 B**（使用服务器 A 的对端配置命令）：

```bash
sudo twnode line create eth0 203.0.113.10 10.0.0.1 10.0.0.2 tunnel_ab \
  --type wireguard --mode client --private-key '...' --peer-pubkey '...' --peer-port 51820
```

#### 2. 服务器 B 和 C 之间创建第二层隧道

**服务器 B**：

```bash
sudo twnode line create eth0 192.0.2.30 10.0.1.2 10.0.1.1 tunnel_bc \
  --type wireguard --mode server --listen-port 51821
```

**服务器 C**（使用服务器 B 的对端配置命令）：

```bash
sudo twnode line create eth0 198.51.100.20 10.0.1.1 10.0.1.2 tunnel_bc \
  --type wireguard --mode client --private-key '...' --peer-pubkey '...' --peer-port 51821
```

#### 3. 服务器 A 和 C 之间创建第三层隧道（通过 B 中转）

**服务器 A**（父接口选择 tunnel_ab）：

```bash
# 父接口: tunnel_ab（通过 B 到达 C）
# 对端 IP: 10.0.1.2（C 在 tunnel_bc 上的 VIP，通过 B 可达）
sudo twnode line create tunnel_ab 10.0.1.2 172.16.0.2 172.16.0.1 tunnel_ac \
  --type wireguard --mode server --listen-port 51822
```

**服务器 C**（父接口选择 tunnel_bc）：

```bash
# 父接口: tunnel_bc（通过 B 到达 A）
# 对端 IP: 10.0.0.1（A 在 tunnel_ab 上的 VIP，通过 B 可达）
sudo twnode line create tunnel_bc 10.0.0.1 172.16.0.1 172.16.0.2 tunnel_ac \
  --type wireguard --mode client --private-key '...' --peer-pubkey '...' --peer-port 51822
```

#### 4. 服务器 B 配置路由转发

**重要**：服务器 B 需要开启 IP 转发并配置路由，使 A 和 C 能够互通。

```bash
# 在服务器 B 上执行

# 1. 确认 IP 转发已启用（init 时已设置）
sysctl net.ipv4.ip_forward
# 应该输出: net.ipv4.ip_forward = 1

# 2. 添加路由（将 A 的流量转发到 C）
sudo ip route add 172.16.0.2 via 10.0.1.2 dev tunnel_bc

# 3. 添加路由（将 C 的流量转发到 A）
sudo ip route add 172.16.0.1 via 10.0.0.1 dev tunnel_ab
```

### 验证三层隧道

在**服务器 A** 上 Ping 服务器 C：

```bash
ping -c 4 172.16.0.2
```

输出：

```
PING 172.16.0.2 (172.16.0.2) 56(84) bytes of data.
64 bytes from 172.16.0.2: icmp_seq=1 ttl=63 time=25.8 ms
64 bytes from 172.16.0.2: icmp_seq=2 ttl=63 time=24.5 ms
64 bytes from 172.16.0.2: icmp_seq=3 ttl=63 time=26.2 ms
64 bytes from 172.16.0.2: icmp_seq=4 ttl=63 time=25.0 ms

--- 172.16.0.2 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3004ms
rtt min/avg/max/mdev = 24.512/25.375/26.243/0.684 ms
```

**注意 TTL = 63**：说明数据包经过了服务器 B 的转发（TTL 减 1）。

### 数据包流经路径

```
服务器 A (172.16.0.1)
  ↓
发送 IP 包: dst=172.16.0.2
  ↓
通过 tunnel_ac 封装 (基于 tunnel_ab)
  ↓
经过 tunnel_ab 到达服务器 B (10.0.0.2)
  ↓
服务器 B 转发到 tunnel_bc
  ↓
通过 tunnel_bc 到达服务器 C (10.0.1.2)
  ↓
服务器 C 解封装
  ↓
到达服务器 C (172.16.0.2)
```

## 场景3: 混合隧道类型

### 网络拓扑

结合 WireGuard 和 GRE over IPsec：

```
服务器 A → 服务器 B

第一层: WireGuard 隧道
  eth0 (203.0.113.10) ←→ eth0 (198.51.100.20)
  VIP: 10.0.0.1 ←→ 10.0.0.2

第二层: GRE over IPsec 隧道（建立在 WireGuard 之上）
  tunnel_wg (10.0.0.1) ←→ tunnel_wg (10.0.0.2)
  VIP: 172.16.0.1 ←→ 172.16.0.2
```

### 配置步骤

#### 1. 创建第一层 WireGuard 隧道

**服务器 A**：

```bash
sudo twnode line create eth0 198.51.100.20 10.0.0.2 10.0.0.1 tunnel_wg \
  --type wireguard --mode server --listen-port 51820
```

**服务器 B**：

```bash
sudo twnode line create eth0 203.0.113.10 10.0.0.1 10.0.0.2 tunnel_wg \
  --type wireguard --mode client --private-key '...' --peer-pubkey '...' --peer-port 51820
```

#### 2. 创建第二层 GRE over IPsec 隧道

**服务器 A**（父接口选择 tunnel_wg）：

```bash
sudo twnode line create tunnel_wg 10.0.0.2 172.16.0.2 172.16.0.1 tunnel_gre \
  --auth-key "0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f" \
  --enc-key "0x9f8e7d6c5b4a39281716050403020100aabbccddeeff0011" \
  --encrypt
```

**服务器 B**（父接口选择 tunnel_wg）：

```bash
sudo twnode line create tunnel_wg 10.0.0.1 172.16.0.1 172.16.0.2 tunnel_gre \
  --auth-key "0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f" \
  --enc-key "0x9f8e7d6c5b4a39281716050403020100aabbccddeeff0011" \
  --encrypt
```

### 验证

```bash
# 在服务器 A 上
ping -c 4 172.16.0.2
```

这时数据包经过：
1. GRE 封装
2. IPsec ESP 加密
3. WireGuard 加密
4. 物理网络传输

**三层加密**！

## 实际应用场景

### 场景1: 企业多分支互联

```
总部 (A) ←→ 分支1 (B) ←→ 分支2 (C)

- A ↔ B: 第一层隧道（WireGuard）
- B ↔ C: 第一层隧道（WireGuard）
- A ↔ C: 第二层隧道（通过 B 中转）

优势:
- 只需两个隧道即可实现三地互通
- 降低配置复杂度
```

### 场景2: 高安全性传输

```
服务器 A → 服务器 B

第一层: WireGuard（快速、现代加密）
第二层: GRE+IPsec（额外加密层）

优势:
- 双重加密，安全性极高
- 适合传输敏感数据
```

### 场景3: 动态 IP 中转

```
动态 IP 客户端 (A) → 固定 IP 中转服务器 (B) → 目标服务器 (C)

第一层: A ↔ B (WireGuard 服务器模式)
第二层: B ↔ C (固定 IP 连接)
第三层: A ↔ C (通过 B 中转)

优势:
- 客户端 A 无需固定 IP
- 通过 B 中转到达 C
```

### 场景4: 负载均衡

```
客户端 (A) → 多个中转服务器 (B1, B2, B3) → 目标服务器 (C)

第一层: A ↔ B1, A ↔ B2, A ↔ B3
第二层: A ↔ C（通过 B1, B2, B3 中转，策略路由分配）

优势:
- 流量分散到多个中转服务器
- 提高带宽和可靠性
```

## 路由表和优先级

### 保护路由的层次

嵌套隧道会产生多个保护路由（优先级 10）：

```bash
# 查看保护路由
ip rule show pref 10
```

输出示例：

```
10:     from all to 198.51.100.20 lookup main   # 保护 tunnel_base 的对端 IP
10:     from all to 10.0.0.2 lookup main         # 保护 tunnel_secure 的对端 IP
```

**重要**：
- 第一层隧道的对端 IP（198.51.100.20）走主路由表（物理接口）
- 第二层隧道的对端 IP（10.0.0.2）走主路由表（但会匹配到 tunnel_base）

### 虚拟 IP 路由表（表 80）

所有隧道的虚拟 IP 都在路由表 80：

```bash
ip route show table 80
```

输出示例：

```
10.0.0.2 dev tunnel_base scope link
172.16.0.2 dev tunnel_secure scope link
```

## 性能考虑

### 延迟

每增加一层隧道，延迟会增加：

```
物理网络延迟: 10ms
+ 第一层隧道处理: +2ms
+ 第二层隧道处理: +2ms
= 总延迟: 14ms
```

### MTU

每层隧道会减少 MTU：

```
物理接口 MTU: 1500
- WireGuard 开销: ~60 字节
= 第一层隧道 MTU: 1440

- 第二层 WireGuard 开销: ~60 字节
= 第二层隧道 MTU: 1380
```

建议手动设置 MTU：

```bash
# 设置第一层隧道 MTU
sudo ip link set tunnel_base mtu 1440

# 设置第二层隧道 MTU
sudo ip link set tunnel_secure mtu 1380
```

### CPU 开销

双重/多重加密会增加 CPU 使用率：

```
单层 WireGuard: ~5% CPU
双层 WireGuard: ~10% CPU
三层 WireGuard: ~15% CPU
```

## 故障排除

### 问题1: 第二层隧道无法 Ping 通

**检查步骤**：

```bash
# 1. 检查第一层隧道是否正常
ping -c 4 10.0.0.2

# 2. 检查第二层隧道接口是否 UP
ip link show tunnel_secure

# 3. 检查第二层隧道的路由
ip route get 172.16.0.2

# 4. 检查保护路由
ip rule show pref 10
```

### 问题2: 三层隧道中转失败

**检查中转服务器**：

```bash
# 1. 检查 IP 转发是否启用
sysctl net.ipv4.ip_forward
# 应该输出 1

# 2. 检查路由表
ip route show

# 3. 检查 iptables 是否阻止转发
sudo iptables -L FORWARD -v
```

### 问题3: MTU 过大导致丢包

**症状**：大数据包无法传输，小数据包正常

**解决**：

```bash
# 降低隧道 MTU
sudo ip link set tunnel_secure mtu 1380

# 或启用 PMTU 发现
sudo sysctl -w net.ipv4.ip_no_pmtu_disc=0
```

## 常见问题

### Q: 嵌套隧道的层数有限制吗？

A: 理论上无限制，但实际受限于：
- MTU 大小（每层减少约 60 字节）
- 性能开销（每层增加延迟和 CPU 使用）
- 建议不超过 3 层

### Q: 如何选择父接口？

A: 创建隧道时：
- 直接连接物理网络 → 选择物理接口（eth0）
- 通过已有隧道连接 → 选择隧道接口（tunnel_base）

### Q: 嵌套隧道的安全性如何？

A: 每层隧道提供一层加密，双重/多重加密安全性更高。但过多的层数会影响性能。

### Q: 删除父隧道会影响子隧道吗？

A: 是的！删除父隧道（如 tunnel_base）会导致子隧道（如 tunnel_secure）失效。应该先删除子隧道，再删除父隧道。

## 下一步

- [WireGuard 配置](wireguard-setup.md) - WireGuard 隧道详细配置
- [GRE over IPsec 配置](gre-ipsec-setup.md) - GRE 隧道详细配置
- [架构说明](../architecture.md) - 深入理解系统架构

---

**导航**: [← 故障转移配置](failover-setup.md) | [返回首页](../index.md)
