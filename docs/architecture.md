# 架构设计

本文档深入介绍 TrueWord Node 的核心架构设计和技术实现。

## 核心设计理念

TrueWord Node 的架构设计遵循以下核心理念：

1. **分层抽象** - 支持多层隧道嵌套，父接口可以是物理接口或已创建的隧道
2. **自动化管理** - 自动获取 IP、自动配置路由、自动保护底层连接
3. **可撤销性** - 所有网络操作都记录撤销命令，支持完全回退
4. **容错机制** - 动态 IP 检测、保护路由同步、故障转移

## 分层隧道系统

### 父接口（Parent Interface）概念

这是 TrueWord Node 最重要的架构特性。

#### 什么是父接口？

父接口是创建新隧道时的**底层传输接口**，可以是：

- **物理网络接口**（如 `eth0`, `ens33`, `wlan0`）
- **已创建的隧道**（如 `tun01`, `tun02`, `wg0`）

#### 为什么需要父接口？

传统的隧道配置需要用户手动指定本地 IP，这在嵌套隧道场景中非常繁琐：

```
❌ 传统方式（手动指定 IP）
创建隧道1: local_ip=192.168.1.100, remote_ip=203.0.113.50
创建隧道2: local_ip=10.0.0.1（隧道1的VIP）, remote_ip=10.0.0.2
创建隧道3: local_ip=172.16.0.1（隧道2的VIP）, remote_ip=172.16.0.2
```

TrueWord Node 的父接口机制简化了这一过程：

```
✅ TrueWord Node（自动获取 IP）
创建隧道1: 父接口=eth0（自动从 eth0 获取 192.168.1.100）
创建隧道2: 父接口=tun01（自动从 tun01 获取 10.0.0.1）
创建隧道3: 父接口=tun02（自动从 tun02 获取 172.16.0.1）
```

#### 父接口的 IP 获取逻辑

```go
// pkg/ipsec/tunnel_manager.go
func getLocalIPFromParent(parentName string) (string, error) {
    // 1. 先检查是否是已创建的隧道
    if tunnel := loadTunnelConfig(parentName); tunnel != nil {
        return tunnel.LocalVIP  // 使用隧道的虚拟 IP
    }

    // 2. 否则，从物理接口获取 IP
    iface := getPhysicalInterface(parentName)
    return iface.IP  // 使用物理接口的 IP
}
```

#### 多层嵌套示例

```
物理接口（eth0）
  ↓ 本地IP: 192.168.1.100
隧道1（tun01）GRE over IPsec
  ↓ 虚拟IP: 10.0.0.1
隧道2（tun02）WireGuard
  ↓ 虚拟IP: 172.16.0.1
隧道3（tun03）GRE over IPsec
  虚拟IP: 192.168.100.1
```

创建命令：

```bash
# 第一层：基于物理接口
sudo twnode line create eth0 203.0.113.50 10.0.0.2 10.0.0.1 tun01

# 第二层：基于第一层隧道
sudo twnode line create tun01 10.0.0.2 172.16.0.2 172.16.0.1 tun02 \
  --type wireguard --mode server --listen-port 51821

# 第三层：基于第二层隧道
sudo twnode line create tun02 172.16.0.2 192.168.100.2 192.168.100.1 tun03
```

### 实现位置

- **父接口 IP 获取**: `pkg/ipsec/tunnel_manager.go` 中的 `getLocalIPFromParent()`
- **父接口列表**: `pkg/network/parent_interface.go` - 列出所有可用父接口
- **物理接口扫描**: `pkg/network/interface.go` - 扫描和管理物理接口

## 隧道类型

TrueWord Node 支持两种隧道类型，通过配置文件中的 `TunnelType` 字段区分。

### GRE over IPsec 隧道

#### 双层结构

**第一层：IPsec ESP 隧道**（可选加密层）

- 协议：ESP（Encapsulating Security Payload）
- 认证：HMAC-SHA256
- 加密：AES-256-CBC
- 模式：Tunnel mode
- 管理工具：`ip xfrm`

**第二层：GRE 隧道**（数据传输层）

- 协议：Generic Routing Encapsulation
- GRE Key：从认证密钥生成（确保对称性）
- 虚拟 IP：在 GRE 接口上配置
- 管理工具：`ip tunnel`

#### 创建流程

```
1. 创建 IPsec 连接
   ├─ 添加 XFRM state（inbound + outbound）
   └─ 添加 XFRM policy（inbound + outbound）

2. 创建 GRE 隧道
   ├─ 创建 GRE 接口（ip tunnel add）
   ├─ 配置虚拟 IP（ip addr add）
   └─ 启动接口（ip link set up）

3. 设置策略路由
   └─ 添加对端 IP 路由到表 50（仅物理接口）
```

#### GRE Key 生成算法

GRE Key 必须在两端保持一致。算法：

```go
func generateGREKey(authKey string) uint32 {
    authKey = strings.TrimPrefix(authKey, "0x")
    var sum uint32
    for _, c := range authKey {
        sum += uint32(c)
    }
    return sum
}
```

**示例**:
```
认证密钥: "0x1234567890abcdef"
去除前缀: "1234567890abcdef"
字符求和: '1'(49) + '2'(50) + ... = 840
GRE Key: 840
```

#### SPI（Security Parameter Index）生成

SPI 必须对称。算法：

```go
func generateSPI(ip1, ip2 string) string {
    // 1. 对 IP 排序（确保两端生成相同的 SPI）
    sortedIP1, sortedIP2 := sortIPs(ip1, ip2)

    // 2. 计算 MD5 哈希
    combined := sortedIP1 + sortedIP2
    hash := md5.Sum([]byte(combined))

    // 3. 取前 8 位（十六进制）
    return "0x" + hex.EncodeToString(hash[:])[:8]
}
```

**示例**:
```
IP1: 192.168.1.100
IP2: 203.0.113.50
排序后: 192.168.1.100, 203.0.113.50
MD5: a1b2c3d4e5f6...
SPI: 0xa1b2c3d4
```

#### 实现位置

- **IPsec 创建**: `pkg/ipsec/tunnel.go` 中的 `CreateIPsec()`
- **GRE 创建**: `pkg/ipsec/tunnel.go` 中的 `(*Tunnel).Create()`
- **隧道管理**: `pkg/ipsec/tunnel_manager.go`

### WireGuard 隧道

#### 核心特性

- **加密算法**: Curve25519（非对称加密）
- **传输方式**: UDP（支持 NAT 穿透）
- **内置加密**: 无需额外的 IPsec 层
- **握手机制**: 静默协议（数据包触发）

#### 密钥管理

每个 WireGuard 隧道端点都有自己的密钥对：

- **私钥**（Private Key）: 保存在本地配置文件，永不传输
- **公钥**（Public Key）: 发送给对端用于加密

**服务器模式（自动生成所有密钥）**:

```
服务器 A 执行创建命令
  ↓
生成密钥对 A（私钥A + 公钥A）
  ↓
生成密钥对 B（私钥B + 公钥B）
  ↓
配置本地 WireGuard（私钥A，对端公钥B）
  ↓
输出对端创建命令（包含私钥B和公钥A）
```

**客户端模式（使用服务器提供的密钥）**:

```
复制服务器输出的命令
  ↓
使用提供的私钥B和对端公钥A
  ↓
配置本地 WireGuard（私钥B，对端公钥A）
```

#### 握手机制

WireGuard 采用"静默协议"（Silent Protocol）：

- **不主动发送握手包**：节省带宽，增强隐蔽性
- **数据包触发握手**：首次发送数据时才建立连接
- **首次握手延迟**：通常需要 5-10 秒

**TrueWord Node 的优化**：

```go
// pkg/wireguard/tunnel.go
func triggerWireGuardHandshake(interfaceName, peerVIP string) {
    // 客户端模式：主动发送 ping 包触发握手
    cmd := exec.Command("ping", "-c", "3", "-W", "1", "-I", interfaceName, peerVIP)
    cmd.Run()
}

func waitForWireGuardHandshake(interfaceName string) {
    // 等待握手完成（检查 latest-handshake 时间戳）
    for i := 0; i < 30; i++ {
        cmd := exec.Command("wg", "show", interfaceName, "latest-handshakes")
        output, _ := cmd.Output()
        if len(output) > 0 && !strings.Contains(string(output), "0") {
            return  // 握手成功
        }
        time.Sleep(1 * time.Second)
    }
}
```

#### 动态 IP 支持

WireGuard 服务器模式支持接收动态 IP 客户端：

1. **创建时使用占位符**：`remote_ip = 0.0.0.0`
2. **首次连接后获取实际 IP**：通过 `wg show <interface> endpoints`
3. **保护路由同步**：定期检测 IP 变化并更新保护路由

```bash
# 获取运行时对端 IP
sudo wg show wg0 endpoints
# 输出: aB3cD...=    103.118.40.121:51820
```

#### 实现位置

- **WireGuard 创建**: `pkg/wireguard/tunnel.go`
- **密钥生成**: `pkg/wireguard/keygen.go`
- **握手优化**: `pkg/wireguard/tunnel.go` 中的 `triggerWireGuardHandshake()`

## 路由表架构

TrueWord Node 使用多个 Linux 路由表实现灵活的路由策略。

### 路由表设计

```
表 5（临时测试路由）
  └─ 优先级 5
  └─ 用途：check 和 failover 测试期间使用
  └─ 生命周期：测试完成后立即清理（defer）

表 50（策略路由表）
  └─ 优先级 10-899（保护路由优先级10，用户策略100-899）
  └─ 用途：策略路由规则
  └─ 管理：policy 命令

表 80（虚拟 IP 表）
  └─ 优先级 80
  └─ 用途：隧道虚拟 IP 路由
  └─ 规则：ip rule add from all lookup 80 pref 80

主路由表（Main Table）
  └─ 优先级 32766
  └─ 用途：系统默认路由
  └─ 内容：物理接口的路由

默认路由表（Default Table）
  └─ 优先级 32767
  └─ 用途：系统默认路由
```

### 优先级分配

```
5          临时测试路由（最高优先级，确保测试流量不受策略干扰）
10         保护路由（保护隧道底层连接，防止路由环路）
100-899    用户策略组（支持手动指定或自动分配）
900        默认路由（0.0.0.0/0 兜底路由，可选）
32766      主路由表
32767      系统默认路由表
```

### 策略路由规则示例

```bash
# 查看所有路由规则
ip rule show

# 输出示例:
0:      from all lookup local
5:      from all to 8.8.8.8 lookup 5         # 临时测试路由
10:     from all to 203.0.113.50 lookup main # 保护路由
80:     from all lookup 80                    # 虚拟 IP 表
150:    from all to 192.168.100.0/24 lookup 50 # 用户策略
900:    from all lookup 50                     # 默认路由（可选）
32766:  from all lookup main
32767:  from all lookup default
```

### 实现位置

- **路由表配置**: `pkg/ipsec/tunnel_manager.go` 中的 `setupPolicyRoute()`
- **策略路由管理**: `pkg/routing/policy.go`

## 保护路由机制

### 为什么需要保护路由？

当隧道被设置为策略路由出口时，可能导致**路由环路**：

```
问题场景:
1. 创建 WireGuard 隧道: eth0 → 203.0.113.50
2. 设置策略路由: 所有流量 → WireGuard 隧道
3. WireGuard 握手包也会走策略路由 → WireGuard 隧道
4. 但握手包的目标是 203.0.113.50，应该走 eth0！
5. 结果：握手包在隧道内循环，隧道无法建立
```

### 解决方案

优先级 10 的保护路由确保隧道对端 IP 的流量不走策略路由：

```bash
# 保护路由规则
ip rule add to 203.0.113.50 lookup main pref 10
```

**优先级顺序**:
```
10（保护路由）> 100-899（用户策略）> 900（默认路由）
```

这样，发往 `203.0.113.50` 的流量（如 WireGuard 握手包）会优先匹配保护路由，通过主路由表（即 eth0）发送，而不会走策略路由进入隧道。

### 动态 IP 容错

#### 问题

WireGuard 服务器接收动态 IP 客户端时，客户端 IP 可能变化，但配置文件中的 `ProtectedIP` 不会自动更新。

#### 解决方案

`policy sync-protection` 命令自动检测 IP 变化并更新保护路由：

```bash
# 手动同步
sudo twnode policy sync-protection

# Cron 定时任务（推荐）
*/5 * * * * /usr/local/bin/twnode policy sync-protection
```

#### 工作流程

```
1. 加载所有隧道配置

2. 对每个隧道：
   ├─ GRE 隧道 → 从配置文件读取 RemoteIP
   ├─ WireGuard 客户端 → 从配置文件读取 RemoteIP
   └─ WireGuard 服务器 → 从运行状态获取实际对端IP
      └─ 执行: wg show <interface> endpoints
      └─ 解析输出获取当前对端IP

3. 检查 ProtectedIP 字段：
   ├─ 如果 IP 未变化 → 跳过
   ├─ 如果 IP 已变化 → 更新保护路由
   │  ├─ 删除旧保护路由: ip rule del to <旧IP> pref 10
   │  ├─ 添加新保护路由: ip rule add to <新IP> lookup main pref 10
   │  └─ 更新配置文件中的 ProtectedIP
   └─ 如果缺失保护路由 → 添加保护路由

4. 扫描所有优先级10的规则：
   └─ 清理无对应隧道的规则（僵尸规则）
```

#### 示例输出

```
同步保护路由...
  ℹ 从运行状态检测到 WireGuard 隧道 hk-tw 的对端IP: 103.118.40.121
  ⚠ WireGuard 隧道 hk-tw 对端IP已变化: 1.2.3.4 → 103.118.40.121
  ✓ 保护 GRE 隧道 tun01 的远程IP 192.168.1.100
  清理 1 个僵尸规则...
  ✓ 已清理僵尸规则: 5.6.7.8
  已更新 1 个隧道的保护路由
✓ 保护路由同步完成
```

### 自动同步时机

保护路由同步会在以下时机自动执行：

- `policy apply` 开始前
- `line start <name>` 完成后
- `line start-all` 完成后

### 实现位置

- **同步逻辑**: `pkg/routing/policy.go` 中的 `SyncProtection()`
- **获取 WireGuard 对端 IP**: `pkg/wireguard/tunnel.go` 中的 `GetWireGuardPeerEndpoint()`

## 撤销机制

### 设计理念

所有网络操作都应该可以完全回退，不留痕迹。

### 实现方式

每个网络操作都会记录对应的撤销命令到 `/var/lib/trueword_node/rev/` 目录：

```
/var/lib/trueword_node/rev/
├── tun01.rev              # 隧道撤销命令
├── 192.168.1.100-203.0.113.50.rev  # IPsec 撤销命令
└── ...
```

### 撤销文件示例

**隧道撤销文件** (`tun01.rev`):

```bash
ip link set tun01 down
ip tunnel del tun01
ip rule del to 10.0.0.2/32 lookup 80 pref 80
ip rule del to 203.0.113.50 lookup main pref 10
```

**IPsec 撤销文件** (`192.168.1.100-203.0.113.50.rev`):

```bash
ip xfrm state del src 192.168.1.100 dst 203.0.113.50 proto esp spi 0xa1b2c3d4
ip xfrm state del src 203.0.113.50 dst 192.168.1.100 proto esp spi 0xa1b2c3d4
ip xfrm policy del src 192.168.1.100 dst 203.0.113.50 dir out
ip xfrm policy del src 203.0.113.50 dst 192.168.1.100 dir in
```

### 执行撤销

```go
// pkg/ipsec/tunnel.go
func executeRevCommands(revFile string) error {
    data, err := ioutil.ReadFile(revFile)
    if err != nil {
        return err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        // 执行撤销命令（静默执行，忽略错误）
        parts := strings.Fields(line)
        cmd := exec.Command(parts[0], parts[1:]...)
        cmd.Run()
    }

    // 删除撤销文件
    os.Remove(revFile)
    return nil
}
```

### 实现位置

- **记录撤销命令**: `pkg/ipsec/tunnel.go` 中的 `recordRevCommands()`
- **执行撤销命令**: `pkg/ipsec/tunnel.go` 中的 `executeRevCommands()`
- **WireGuard 撤销**: `pkg/wireguard/tunnel.go`

## 策略规则管理

### 避免网络中断的关键

在更新策略路由规则时，必须保证无缝切换，避免网络中断。

### 错误的做法 ❌

```go
// 错误：先删除旧规则，再添加新规则
exec.Command("ip", "rule", "del", "pref", "150").Run()  // 删除规则
exec.Command("ip", "rule", "add", "to", "192.168.100.0/24", "lookup", "50", "pref", "150").Run()  // 添加新规则

// 问题：中间有时间窗口，流量无法路由！
```

### 正确的做法 ✅

```go
// 正确：先添加新规则，再清理重复规则
// 1. 添加新规则
exec.Command("ip", "rule", "add", "to", "192.168.100.0/24", "lookup", "50", "pref", "150").Run()

// 2. 循环检测并清理重复规则
for {
    cmd := exec.Command("bash", "-c", "ip rule show pref 150 | wc -l")
    output, _ := cmd.Output()
    count, _ := strconv.Atoi(strings.TrimSpace(string(output)))

    if count <= 1 {
        break  // 只剩一个规则，完成
    }

    // 删除一个重复规则
    exec.Command("ip", "rule", "del", "pref", "150").Run()
    time.Sleep(100 * time.Millisecond)
}

// 3. 验证规则存在
cmd := exec.Command("bash", "-c", "ip rule show pref 150 | wc -l")
output, _ := cmd.Output()
count, _ := strconv.Atoi(strings.TrimSpace(string(output)))

if count == 0 {
    // 规则被意外删除，重新添加
    exec.Command("ip", "rule", "add", "to", "192.168.100.0/24", "lookup", "50", "pref", "150").Run()
}
```

### 优势

- ✅ 始终至少有一个规则存在，不会中断网络
- ✅ 清理重复规则，避免冲突
- ✅ 有验证和恢复机制

### 实现位置

- `pkg/routing/policy.go` 中的 `ApplyGroup()`
- `pkg/routing/policy.go` 中的 `applyDefaultRoute()`

## 网络操作库

### 使用 netlink 库

TrueWord Node 优先使用 **github.com/vishvananda/netlink** 库进行网络配置，而不是直接调用系统命令。

### 为什么使用 netlink？

- ✅ **类型安全**: Go 类型检查，避免命令字符串拼接错误
- ✅ **性能更好**: 直接调用 netlink 接口，无需 fork 子进程
- ✅ **错误处理**: 返回结构化错误，便于处理
- ✅ **可测试性**: 易于编写单元测试

### 示例对比

**使用系统命令**（不推荐）:

```go
cmd := exec.Command("ip", "link", "add", "name", "tun01", "type", "gre",
                   "local", "192.168.1.100", "remote", "203.0.113.50")
err := cmd.Run()
```

**使用 netlink 库**（推荐）:

```go
import "github.com/vishvananda/netlink"

link := &netlink.Gretun{
    LinkAttrs: netlink.LinkAttrs{Name: "tun01"},
    Local:     net.ParseIP("192.168.1.100"),
    Remote:    net.ParseIP("203.0.113.50"),
}
err := netlink.LinkAdd(link)
```

### 何时使用系统命令？

仅在 netlink 库不支持的功能时才使用系统命令：

- `ip xfrm` - IPsec 配置（netlink 库支持有限）
- `wg` - WireGuard 配置（需要 wg-tools）
- `ping` - 连通性测试

### 实现位置

- **接口扫描**: `pkg/network/interface.go`
- **GRE 隧道**: `pkg/ipsec/tunnel.go`

## 连通性检查和评分

### 检查结果持久化

检查结果保存到 `/var/lib/trueword_node/check_results.json`：

```json
{
  "tunnel_ab": {
    "interface": "tunnel_ab",
    "check_ip": "8.8.8.8",
    "packet_loss": 0,
    "avg_latency": 15.3,
    "score": 96.2,
    "status": "good",
    "timestamp": "2025-01-15T10:30:00Z"
  }
}
```

### Ping 测试优化

```bash
# 发送 20 个 ping 包（提供 5% 丢包率精度）
ping -c 20 -i 0.2 -W 1 -I <接口> <目标IP>

# 参数说明:
# -c 20     发送 20 个包（100/20 = 5% 精度）
# -i 0.2    包间隔 0.2 秒（加速测试）
# -W 1      超时 1 秒
# -I <接口> 指定出口接口

# 总测试时间: 约 4 秒（20 * 0.2 = 4）
```

### 评分算法

```
基础评分（0-100分）:
  丢包率评分 = (1 - 丢包率) * 60        # 0-60分，权重更高
  延迟评分 = max(0, (1 - 延迟/200)) * 40  # 0-40分

成本惩罚:
  惩罚值 = Cost × 0.5                   # Cost 字段（用户可配置）

最终评分:
  总分 = 基础评分 - 成本惩罚             # 分数越高越好
```

**示例**:

```
场景1: 丢包率 0%, 延迟 15ms, Cost 0
  丢包率评分 = (1 - 0) * 60 = 60
  延迟评分 = (1 - 15/200) * 40 = 37
  基础评分 = 60 + 37 = 97
  最终评分 = 97 - 0 = 97

场景2: 丢包率 5%, 延迟 80ms, Cost 10
  丢包率评分 = (1 - 0.05) * 60 = 57
  延迟评分 = (1 - 80/200) * 40 = 24
  基础评分 = 57 + 24 = 81
  成本惩罚 = 10 * 0.5 = 5
  最终评分 = 81 - 5 = 76
```

### 临时测试路由

连通性检查使用优先级 5 的临时测试路由：

```go
// 添加临时路由
cmd := exec.Command("ip", "rule", "add", "to", checkIP, "lookup", "5", "pref", "5")
cmd.Run()

// 使用 defer 确保测试完成后立即清理
defer func() {
    exec.Command("ip", "rule", "del", "to", checkIP, "pref", "5").Run()
}()

// 执行 ping 测试
// ...
```

**为什么使用优先级 5？**

- 最高优先级，确保测试流量不受用户策略干扰
- 临时存在（约 4 秒），不会长期影响系统路由

### 实现位置

- **连通性检查**: `pkg/network/check.go` 中的 `CheckInterface()`
- **结果保存**: `pkg/network/check.go` 中的 `SaveCheckResults()`
- **评分算法**: `pkg/network/check.go` 中的 `calculateScore()`
- **故障转移**: `pkg/routing/policy.go` 中的 `FailoverGroup()` 和 `FailoverDefault()`

## 配置文件结构

### 全局配置

`/etc/trueword_node/config.yaml`:

```yaml
default_route:
  enabled: true
  exit_interface: tunnel_ab
  priority: 900
```

### 物理接口配置

`/etc/trueword_node/interfaces/physical.yaml`:

```yaml
interfaces:
  - name: eth0
    ip: 192.168.1.100
    gateway: 192.168.1.1
    managed: true
  - name: eth1
    ip: 10.0.0.50
    gateway: 10.0.0.1
    managed: false
```

### 隧道配置

`/etc/trueword_node/tunnels/tunnel_ab.yaml`:

```yaml
name: tunnel_ab
parent_interface: eth0
tunnel_type: wireguard  # 或 gre
local_ip: 192.168.1.100
remote_ip: 203.0.113.50
local_vip: 10.0.0.1
remote_vip: 10.0.0.2
protected_ip: 203.0.113.50  # 当前保护的 IP（动态更新）

# WireGuard 特有字段
listen_port: 51820
private_key: aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=
peer_pubkey: xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=
peer_port: 51820

# GRE over IPsec 特有字段（如果适用）
auth_key: 0x1234567890abcdef
enc_key: 0xfedcba0987654321
encryption_enabled: true
```

### 策略组配置

`/etc/trueword_node/policies/vpn_traffic.json`:

```json
{
  "name": "vpn_traffic",
  "exit_interface": "tunnel_ab",
  "priority": 150,
  "from_source": "",
  "cidrs": [
    "192.168.100.0/24",
    "192.168.101.0/24"
  ],
  "cost": 0
}
```

## 下一步

深入了解各个子系统：

- [路由表详解](reference/routing-tables.md)
- [保护路由详解](reference/protection-routes.md)
- [配置文件详解](reference/config-files.md)

或查看命令参考：

- [隧道管理命令](commands/line/index.md)
- [策略路由命令](commands/policy/index.md)

---

**导航**: [← 快速入门](getting-started.md) | [返回首页](index.md) | [命令参考 →](commands/)
