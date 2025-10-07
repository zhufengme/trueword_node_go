# 配置文件详解

本文档详细说明 TrueWord Node 的所有配置文件格式和字段含义。

## 配置目录结构

```
/etc/trueword_node/
├── config.yaml              # 全局配置
├── interfaces/
│   └── physical.yaml       # 物理接口配置
├── tunnels/
│   ├── tunnel_ab.yaml     # 隧道配置（每个隧道一个文件）
│   ├── tunnel_cd.yaml
│   └── ...
└── policies/
    ├── group1.json        # 策略组配置（每个策略组一个文件）
    ├── group2.json
    └── ...

/var/lib/trueword_node/
├── rev/
│   ├── tunnel_ab.rev      # 隧道撤销命令
│   ├── 192.168.1.100-203.0.113.50.rev  # IPsec 撤销命令
│   └── ...
├── peer_configs/
│   ├── tunnel_ab.txt      # WireGuard 对端配置命令
│   └── ...
└── check_results.json     # 连通性检查结果
```

## 全局配置

### 文件路径

`/etc/trueword_node/config.yaml`

### 格式

```yaml
default_route:
  enabled: true              # 是否启用默认路由
  exit_interface: tunnel_ab  # 默认出口接口
  priority: 900              # 路由规则优先级
```

### 字段说明

| 字段 | 类型 | 说明 | 必需 |
|------|------|------|------|
| `default_route.enabled` | `bool` | 是否启用默认路由（0.0.0.0/0） | 否 |
| `default_route.exit_interface` | `string` | 默认出口接口名称 | 是（如果启用） |
| `default_route.priority` | `int` | 路由规则优先级，固定为 900 | 否 |

### 示例

```yaml
# 启用默认路由
default_route:
  enabled: true
  exit_interface: tunnel_hk
  priority: 900
```

```yaml
# 禁用默认路由
default_route:
  enabled: false
```

## 物理接口配置

### 文件路径

`/etc/trueword_node/interfaces/physical.yaml`

### 格式

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

### 字段说明

| 字段 | 类型 | 说明 | 必需 |
|------|------|------|------|
| `name` | `string` | 接口名称（如 eth0, ens33） | 是 |
| `ip` | `string` | 接口 IP 地址 | 是 |
| `gateway` | `string` | 网关 IP 地址 | 是 |
| `managed` | `bool` | 是否由 TrueWord Node 管理 | 是 |

### 生成方式

此文件由 `twnode init` 命令自动生成，扫描所有物理网络接口并记录其信息。

### 示例

```yaml
interfaces:
  # 管理的接口（用户在 init 时选择）
  - name: eth0
    ip: 192.168.1.100
    gateway: 192.168.1.1
    managed: true

  # 不管理的接口（用户在 init 时未选择）
  - name: wlan0
    ip: 192.168.2.200
    gateway: 192.168.2.1
    managed: false
```

## 隧道配置

### 文件路径

`/etc/trueword_node/tunnels/<隧道名>.yaml`

### WireGuard 隧道配置

```yaml
name: tunnel_ab
parent_interface: eth0
tunnel_type: wireguard
local_ip: 192.168.1.100
remote_ip: 0.0.0.0  # 服务器模式占位符，或客户端模式的对端 IP
local_vip: 10.0.0.1
remote_vip: 10.0.0.2
protected_ip: 103.118.40.121  # 当前保护的 IP（动态更新）

# WireGuard 特有字段
listen_port: 51820
private_key: xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=
peer_pubkey: aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=
peer_port: 51820  # 客户端模式的对端监听端口，服务器模式为 0
mode: server      # server 或 client
```

#### 字段说明

| 字段 | 类型 | 说明 | 必需 |
|------|------|------|------|
| `name` | `string` | 隧道名称（唯一标识符） | 是 |
| `parent_interface` | `string` | 父接口名称（物理接口或隧道） | 是 |
| `tunnel_type` | `string` | 隧道类型，固定为 `wireguard` | 是 |
| `local_ip` | `string` | 本地 IP（从父接口自动获取） | 是 |
| `remote_ip` | `string` | 对端 IP（服务器模式为 `0.0.0.0`） | 是 |
| `local_vip` | `string` | 本地虚拟 IP | 是 |
| `remote_vip` | `string` | 对端虚拟 IP | 是 |
| `protected_ip` | `string` | 当前保护的 IP（由 `sync-protection` 更新） | 否 |
| `listen_port` | `int` | 本地监听端口（服务器模式必需） | 服务器必需 |
| `private_key` | `string` | 本地私钥（Base64 编码） | 是 |
| `peer_pubkey` | `string` | 对端公钥（Base64 编码） | 是 |
| `peer_port` | `int` | 对端监听端口（客户端模式必需） | 客户端必需 |
| `mode` | `string` | `server` 或 `client` | 是 |

### GRE over IPsec 隧道配置

```yaml
name: tunnel_cd
parent_interface: eth0
tunnel_type: gre
local_ip: 192.168.1.100
remote_ip: 203.0.113.50
local_vip: 10.0.1.1
remote_vip: 10.0.1.2
protected_ip: 203.0.113.50

# GRE over IPsec 特有字段
auth_key: 0x1234567890abcdef
enc_key: 0xfedcba0987654321
encryption_enabled: true
```

#### 字段说明

| 字段 | 类型 | 说明 | 必需 |
|------|------|------|------|
| `name` | `string` | 隧道名称（唯一标识符） | 是 |
| `parent_interface` | `string` | 父接口名称（物理接口或隧道） | 是 |
| `tunnel_type` | `string` | 隧道类型，固定为 `gre` | 是 |
| `local_ip` | `string` | 本地 IP（从父接口自动获取） | 是 |
| `remote_ip` | `string` | 对端 IP | 是 |
| `local_vip` | `string` | 本地虚拟 IP | 是 |
| `remote_vip` | `string` | 对端虚拟 IP | 是 |
| `protected_ip` | `string` | 当前保护的 IP | 否 |
| `auth_key` | `string` | IPsec 认证密钥（十六进制） | 是 |
| `enc_key` | `string` | IPsec 加密密钥（十六进制） | 是 |
| `encryption_enabled` | `bool` | 是否启用 IPsec 加密 | 是 |

### 示例

#### WireGuard 服务器模式

```yaml
name: hk-tw
parent_interface: eth0
tunnel_type: wireguard
local_ip: 192.168.1.100
remote_ip: 0.0.0.0
local_vip: 10.0.0.1
remote_vip: 10.0.0.2
protected_ip: 103.118.40.121

listen_port: 51820
private_key: xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=
peer_pubkey: aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=
peer_port: 0
mode: server
```

#### WireGuard 客户端模式

```yaml
name: tw-hk
parent_interface: eth0
tunnel_type: wireguard
local_ip: 10.0.0.50
remote_ip: 192.168.1.100
local_vip: 10.0.0.2
remote_vip: 10.0.0.1
protected_ip: 192.168.1.100

listen_port: 0
private_key: aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=
peer_pubkey: xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=
peer_port: 51820
mode: client
```

#### GRE over IPsec（启用加密）

```yaml
name: tun01
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

#### GRE 不加密

```yaml
name: tun02
parent_interface: eth0
tunnel_type: gre
local_ip: 192.168.1.100
remote_ip: 203.0.113.50
local_vip: 10.0.2.1
remote_vip: 10.0.2.2
protected_ip: 203.0.113.50

auth_key: ""
enc_key: ""
encryption_enabled: false
```

## 策略组配置

### 文件路径

`/etc/trueword_node/policies/<策略组名>.json`

### 格式

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

### 字段说明

| 字段 | 类型 | 说明 | 必需 |
|------|------|------|------|
| `name` | `string` | 策略组名称（唯一标识符） | 是 |
| `exit_interface` | `string` | 出口接口名称 | 是 |
| `priority` | `int` | 路由规则优先级（100-899） | 是 |
| `from_source` | `string` | 源地址限制（CIDR 格式，可选） | 否 |
| `cidrs` | `[]string` | CIDR 列表 | 是 |
| `cost` | `int` | 成本值（用于故障转移评分） | 否 |

### 示例

#### 基本策略组

```json
{
  "name": "asia_traffic",
  "exit_interface": "tunnel_hk",
  "priority": 100,
  "from_source": "",
  "cidrs": [
    "192.168.100.0/24",
    "203.0.113.0/24"
  ],
  "cost": 0
}
```

#### 带源地址限制的策略组

```json
{
  "name": "office_vpn",
  "exit_interface": "tunnel_ab",
  "priority": 200,
  "from_source": "10.0.0.0/8",
  "cidrs": [
    "192.168.200.0/24"
  ],
  "cost": 0
}
```

#### 带成本的策略组（用于故障转移）

```json
{
  "name": "backup_route",
  "exit_interface": "tunnel_backup",
  "priority": 300,
  "from_source": "",
  "cidrs": [
    "192.168.100.0/24"
  ],
  "cost": 10
}
```

**说明**: Cost 值越高，故障转移时评分扣除越多，优先级越低。

## 连通性检查结果

### 文件路径

`/var/lib/trueword_node/check_results.json`

### 格式

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
  },
  "tunnel_cd": {
    "interface": "tunnel_cd",
    "check_ip": "8.8.8.8",
    "packet_loss": 5,
    "avg_latency": 80.5,
    "score": 78.3,
    "status": "degraded",
    "timestamp": "2025-01-15T10:30:05Z"
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `interface` | `string` | 接口名称 |
| `check_ip` | `string` | 测试目标 IP |
| `packet_loss` | `float` | 丢包率（百分比） |
| `avg_latency` | `float` | 平均延迟（毫秒） |
| `score` | `float` | 评分（0-100） |
| `status` | `string` | 状态：`good`, `degraded`, `bad` |
| `timestamp` | `string` | 检查时间（ISO 8601 格式） |

### 评分规则

```
基础评分 = 丢包率评分(60%) + 延迟评分(40%)
最终评分 = 基础评分 - Cost惩罚(Cost × 0.5)

丢包率评分 = (1 - 丢包率) × 60
延迟评分 = max(0, (1 - 延迟/200)) × 40
```

### 状态判断

- `good` - 评分 >= 80
- `degraded` - 60 <= 评分 < 80
- `bad` - 评分 < 60

## 撤销命令文件

### 隧道撤销文件

**文件路径**: `/var/lib/trueword_node/rev/<隧道名>.rev`

**格式**: 纯文本，每行一个 shell 命令

**示例** (`tunnel_ab.rev`):
```bash
ip link set tunnel_ab down
ip link del tunnel_ab
ip rule del to 10.0.0.2/32 lookup 80 pref 80
ip rule del to 203.0.113.50 lookup main pref 10
```

### IPsec 撤销文件

**文件路径**: `/var/lib/trueword_node/rev/<IP1>-<IP2>.rev`

**格式**: 纯文本，每行一个 shell 命令

**示例** (`192.168.1.100-203.0.113.50.rev`):
```bash
ip xfrm state del src 192.168.1.100 dst 203.0.113.50 proto esp spi 0xa1b2c3d4
ip xfrm state del src 203.0.113.50 dst 192.168.1.100 proto esp spi 0xa1b2c3d4
ip xfrm policy del src 192.168.1.100 dst 203.0.113.50 dir out
ip xfrm policy del src 203.0.113.50 dst 192.168.1.100 dir in
```

## WireGuard 对端配置

### 文件路径

`/var/lib/trueword_node/peer_configs/<隧道名>.txt`

### 格式

纯文本，包含完整的 `line create` 命令

### 示例

```bash
sudo twnode line create <父接口> 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820
```

**使用方式**: 复制到对端服务器，替换 `<父接口>` 为实际接口名称后执行。

## 配置文件管理

### 备份配置

```bash
# 备份所有配置
sudo tar -czf twnode_backup_$(date +%Y%m%d).tar.gz \
  /etc/trueword_node \
  /var/lib/trueword_node
```

### 恢复配置

```bash
# 恢复配置
sudo tar -xzf twnode_backup_20250115.tar.gz -C /

# 重新加载（如果需要）
sudo twnode line start-all
sudo twnode policy apply
```

### 手动编辑配置

**不推荐直接编辑配置文件**，应该使用命令行工具：

```bash
# 推荐方式
sudo twnode line delete tunnel_ab
sudo twnode line create ...

# 不推荐
sudo nano /etc/trueword_node/tunnels/tunnel_ab.yaml
```

如果必须手动编辑，编辑后需要：

```bash
# 重新加载隧道
sudo twnode line stop tunnel_ab
sudo twnode line start tunnel_ab

# 重新应用策略
sudo twnode policy revoke
sudo twnode policy apply
```

## 下一步

- [路由表设计](routing-tables.md) - 了解路由表和优先级
- [保护路由机制](protection-routes.md) - 深入理解保护路由
- [故障排查](troubleshooting.md) - 常见问题解决方案

---

**导航**: [← 参考资料](../index.md#参考资料) | [返回首页](../index.md) | [路由表设计 →](routing-tables.md)
