# line create - 创建隧道

## 概述

`line create` 命令用于创建 GRE over IPsec 或 WireGuard 隧道。支持交互式和命令行两种模式。

## 语法

### 交互式模式（推荐用于初学者）

```bash
sudo twnode line create
```

系统会引导你完成配置。

### 命令行模式

#### WireGuard 服务器模式

```bash
sudo twnode line create <父接口> 0.0.0.0 <对端VIP> <本地VIP> <隧道名> \
  --type wireguard \
  --mode server \
  --listen-port <端口>
```

#### WireGuard 客户端模式

```bash
sudo twnode line create <父接口> <对端IP> <对端VIP> <本地VIP> <隧道名> \
  --type wireguard \
  --mode client \
  --private-key '<私钥>' \
  --peer-pubkey '<对端公钥>' \
  --peer-port <端口>
```

#### GRE over IPsec

```bash
sudo twnode line create <父接口> <对端IP> <对端VIP> <本地VIP> <隧道名> \
  --auth-key '<认证密钥>' \
  --enc-key '<加密密钥>'
```

## 参数说明

### 位置参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `<父接口>` | 底层传输接口（物理接口或已创建的隧道） | `eth0`, `tun01` |
| `<对端IP>` | 对端真实 IP 地址（WireGuard 服务器模式使用 `0.0.0.0`） | `203.0.113.50`, `0.0.0.0` |
| `<对端VIP>` | 对端虚拟 IP 地址 | `10.0.0.2` |
| `<本地VIP>` | 本地虚拟 IP 地址 | `10.0.0.1` |
| `<隧道名>` | 隧道接口名称（可选，默认自动生成） | `tunnel_ab` |

### 选项参数

#### 通用选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--type` | 隧道类型：`gre`（GRE over IPsec）或 `wireguard` | `gre` |

#### WireGuard 选项

| 选项 | 说明 | 适用模式 |
|------|------|----------|
| `--mode` | WireGuard 模式：`server` 或 `client` | 必需 |
| `--listen-port` | 本地监听端口 | 服务器模式必需 |
| `--private-key` | 本地私钥（Base64 编码） | 客户端模式必需 |
| `--peer-pubkey` | 对端公钥（Base64 编码） | 客户端模式必需 |
| `--peer-port` | 对端监听端口 | 客户端模式必需 |

#### GRE over IPsec 选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--auth-key` | IPsec 认证密钥（十六进制，可选 `0x` 前缀） | 交互模式提示输入 |
| `--enc-key` | IPsec 加密密钥（十六进制，可选 `0x` 前缀） | 交互模式提示输入 |
| `--no-encryption` | 禁用 IPsec 加密（仅 GRE） | 启用加密 |

## 工作流程

### WireGuard 服务器模式

```
1. 验证父接口存在性
   └─ 检查父接口是物理接口还是已创建的隧道

2. 自动从父接口获取本地 IP
   ├─ 物理接口 → 使用物理接口的 IP
   └─ 隧道接口 → 使用隧道的 LocalVIP

3. 生成本地密钥对（私钥A + 公钥A）

4. 生成对端密钥对（私钥B + 公钥B）

5. 创建 WireGuard 接口
   ├─ 添加接口：ip link add <name> type wireguard
   ├─ 配置私钥：wg set <name> private-key <私钥A>
   ├─ 配置监听端口：wg set <name> listen-port <port>
   └─ 配置对端：wg set <name> peer <公钥B> allowed-ips <对端VIP>/32

6. 配置虚拟 IP
   ├─ 添加地址：ip addr add <本地VIP>/32 dev <name>
   └─ 启动接口：ip link set <name> up

7. 添加虚拟 IP 路由规则
   └─ ip rule add to <对端VIP>/32 lookup 80 pref 80

8. 添加保护路由（占位符，等待客户端连接后更新）
   └─ 优先级 10，确保隧道底层连接不走策略路由

9. 保存配置到文件
   └─ /etc/trueword_node/tunnels/<name>.yaml

10. 输出对端配置命令
    └─ 包含私钥B和公钥A，供对端服务器使用
```

### WireGuard 客户端模式

```
1-2. （同服务器模式）

3. 使用服务器提供的密钥
   ├─ 本地私钥：--private-key
   └─ 对端公钥：--peer-pubkey

4. 创建 WireGuard 接口
   ├─ 添加接口
   ├─ 配置私钥
   ├─ 配置对端：peer <对端公钥> endpoint <对端IP>:<端口> allowed-ips <对端VIP>/32
   └─ 配置 persistent-keepalive 25（保持连接）

5-9. （同服务器模式）

10. 主动触发握手
    └─ 发送 ping 包到对端 VIP，触发 WireGuard 握手
```

### GRE over IPsec

```
1-2. （同 WireGuard）

3. 设置策略路由（仅物理接口）
   └─ 确保对端 IP 通过正确的物理接口路由

4. 创建 IPsec 连接（如果启用加密）
   ├─ 生成对称 SPI（基于 IP 对）
   ├─ 添加 XFRM state（inbound + outbound）
   └─ 添加 XFRM policy（inbound + outbound）

5. 创建 GRE 隧道
   ├─ 生成 GRE Key（从认证密钥生成，确保对称性）
   ├─ 添加 GRE 接口：ip tunnel add <name> mode gre ...
   ├─ 配置虚拟 IP：ip addr add <本地VIP>/32 dev <name>
   └─ 启动接口：ip link set <name> up

6-9. （同 WireGuard）
```

## 示例

### 示例1: WireGuard 服务器模式

**服务器 A**（192.168.1.100）:

```bash
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_ab \
  --type wireguard \
  --mode server \
  --listen-port 51820
```

**输出**:

```
✓ 已创建 WireGuard 隧道: tunnel_ab

【对端配置命令】
在远程服务器上运行以下命令创建对应的隧道:

sudo twnode line create <父接口> 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820

💡 对端配置已保存到: /var/lib/trueword_node/peer_configs/tunnel_ab.txt

提示: 使用 'twnode line show-peer tunnel_ab' 可再次查看对端配置
```

### 示例2: WireGuard 客户端模式

**服务器 B**（203.0.113.50）:

复制服务器 A 输出的命令，替换 `<父接口>` 为实际接口：

```bash
sudo twnode line create eth0 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820
```

**输出**:

```
✓ 已创建 WireGuard 隧道: tunnel_ba
✓ 配置已保存到 /etc/trueword_node/tunnels/tunnel_ba.yaml
```

### 示例3: GRE over IPsec（交互式）

```bash
$ sudo twnode line create

【创建隧道】

可用的父接口:
1. eth0 (192.168.1.100)
2. eth1 (10.0.0.50)
3. tunnel_ab (10.0.0.1)

请选择父接口（输入序号）: 1

隧道类型:
1. GRE over IPsec（传统双层隧道）
2. WireGuard（现代 VPN）

请选择隧道类型（输入序号，默认 1）: 1

请输入对端 IP 地址: 203.0.113.50
请输入对端虚拟 IP: 10.0.1.2
请输入本地虚拟 IP: 10.0.1.1
请输入隧道名称（默认自动生成）: tunnel_cd
请输入认证密钥（十六进制）: 0x1234567890abcdef
请输入加密密钥（十六进制）: 0xfedcba0987654321

【配置信息】
父接口: eth0
本地 IP: 192.168.1.100 (自动获取)
对端 IP: 203.0.113.50
本地 VIP: 10.0.1.1
对端 VIP: 10.0.1.2
隧道名称: tunnel_cd

【建立连接】
✓ 设置策略路由
✓ 创建 IPsec 连接
✓ 创建 GRE 隧道
✓ 配置虚拟 IP
✓ 添加路由规则
✓ 添加保护路由

✓ 隧道创建成功！
✓ 配置已保存到 /etc/trueword_node/tunnels/tunnel_cd.yaml
```

### 示例4: 多层隧道嵌套

```bash
# 第一层：基于物理接口 eth0
sudo twnode line create eth0 203.0.113.50 10.0.0.2 10.0.0.1 tun01

# 第二层：基于隧道 tun01
sudo twnode line create tun01 10.0.0.2 172.16.0.2 172.16.0.1 tun02 \
  --type wireguard --mode server --listen-port 51821

# 第三层：基于隧道 tun02
sudo twnode line create tun02 172.16.0.2 192.168.100.2 192.168.100.1 tun03
```

**隧道链**:
```
eth0 (192.168.1.100)
  └─ tun01 (10.0.0.1)
      └─ tun02 (172.16.0.1)
          └─ tun03 (192.168.100.1)
```

### 示例5: GRE 不加密（仅 GRE）

```bash
sudo twnode line create eth0 203.0.113.50 10.0.2.2 10.0.2.1 tunnel_plain \
  --no-encryption
```

**注意**: 不加密的 GRE 隧道流量是明文传输，仅适用于可信网络环境。

## 配置文件

隧道配置保存在 `/etc/trueword_node/tunnels/<name>.yaml`：

### WireGuard 配置示例

```yaml
name: tunnel_ab
parent_interface: eth0
tunnel_type: wireguard
local_ip: 192.168.1.100
remote_ip: 0.0.0.0  # 服务器模式占位符
local_vip: 10.0.0.1
remote_vip: 10.0.0.2
protected_ip: ""    # 客户端连接后更新

listen_port: 51820
private_key: xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=
peer_pubkey: aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=
peer_port: 0        # 服务器模式不需要
```

### GRE over IPsec 配置示例

```yaml
name: tunnel_cd
parent_interface: eth0
tunnel_type: gre
local_ip: 192.168.1.100
remote_ip: 203.0.113.50
local_vip: 10.0.1.1
remote_vip: 10.0.1.2
protected_ip: 203.0.113.50

auth_key: 0x1234567890abcdef
enc_key: 0xfedcba0987654321
encryption_enabled: true
```

## 撤销命令

每个隧道的撤销命令保存在 `/var/lib/trueword_node/rev/<name>.rev`：

```bash
# tunnel_ab.rev 示例
ip link set tunnel_ab down
ip link del tunnel_ab
ip rule del to 10.0.0.2/32 lookup 80 pref 80
```

删除隧道时会自动执行这些撤销命令。

## 常见问题

### Q: 父接口必须是什么？

A: 父接口可以是**物理网络接口**（如 eth0）或**已创建的隧道**（如 tun01）。系统会自动从父接口获取本地 IP。

### Q: WireGuard 服务器模式为什么对端 IP 是 0.0.0.0？

A: 服务器模式不知道客户端的 IP 地址（尤其是动态 IP 场景）。客户端首次连接后，服务器通过 `wg show` 获取实际对端 IP，并在 `policy sync-protection` 时更新保护路由。

### Q: 如何查看 WireGuard 对端配置命令？

A: 使用 `twnode line show-peer <隧道名>` 查看已保存的对端配置命令。

### Q: GRE Key 如何保证对称性？

A: GRE Key 通过对认证密钥字符串求和生成，确保两端使用相同的认证密钥时生成相同的 GRE Key。

### Q: IPsec SPI 如何保证对称性？

A: SPI 通过对 IP 对排序后计算 MD5 哈希生成，确保无论在哪端创建都生成相同的 SPI。

### Q: 可以创建同名隧道吗？

A: 不可以。隧道名称必须唯一。如果已存在同名隧道，创建会失败。

### Q: 创建后隧道自动启动吗？

A: 不会。创建后隧道处于 inactive 状态，需要使用 `twnode line start <name>` 启动。

## 下一步

- [启动隧道](start.md) - 启动已创建的隧道
- [检查连通性](check.md) - 测试隧道连通性和延迟
- [WireGuard 完整教程](../../tutorials/wireguard-setup.md)
- [GRE over IPsec 完整教程](../../tutorials/gre-ipsec-setup.md)

---

**导航**: [← line 命令](index.md) | [返回首页](../../index.md) | [start →](start.md)
