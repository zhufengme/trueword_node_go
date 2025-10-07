# TrueWord Node

TrueWord Node 是一个强大的 Linux 网络隧道管理工具，支持 **GRE over IPsec** 和 **WireGuard** 隧道，以及灵活的策略路由系统。

> 📚 **[完整文档请访问文档中心 →](docs/index.md)**
>
> 本 README 包含快速入门信息。完整的命令参考、教程和架构说明请查看 [文档中心](docs/index.md)。

## 特性

### 隧道技术
- ✅ **GRE over IPsec** - 传统双层隧道（GRE + IPsec 加密）
- ✅ **WireGuard** - 现代高性能 VPN（服务器/客户端模式）
- ✅ **分层嵌套** - 支持多层隧道链式连接
- ✅ **动态 IP 支持** - 自动检测和更新对端 IP 变化

### 策略路由
- ✅ **策略组管理** - 灵活的路由策略组织
- ✅ **优先级控制** - 自动或手动分配优先级（100-899）
- ✅ **保护路由** - 自动保护隧道底层连接，防止路由环路
- ✅ **默认路由** - 可选的兜底路由（优先级 900）

### 高可用性
- ✅ **连通性检查** - 精准测试丢包率和延迟（5% 精度）
- ✅ **智能评分** - 基于丢包率、延迟和成本的综合评分
- ✅ **自动故障转移** - 根据检查结果自动切换最佳出口
- ✅ **保护路由同步** - 定时检测并更新动态 IP 场景的保护路由

### 操作体验
- ✅ **交互式 CLI** - 友好的命令行交互界面
- ✅ **静态编译** - 无依赖的单二进制文件，适用任何 Linux 系统
- ✅ **撤销机制** - 所有操作可完全回退
- ✅ **美观输出** - 自动对齐的表格显示（中英文混合友好）

## 用途

TrueWord Node 的核心设计目标是在复杂网络环境下建立**可靠、安全、不受审查的通信链路**。

### 主要用途

#### 1. 规避网络审查和干扰

在某些网络环境中，通信可能受到深度包检测（DPI）、协议识别和连接干扰。TrueWord Node 通过以下机制建立抗审查的通信链路：

- **静态密钥认证**：避免传统握手包被识别和拦截
  - 不使用 IKE/IKEv2 协议协商密钥
  - 预共享密钥在两端预先配置
  - 无特征握手包，降低被识别风险

- **GRE over IPsec 架构**：建立不被识别的隧道
  - IPsec ESP 隧道模式提供加密传输
  - GRE 封装在加密层之上
  - 外层流量特征不明显

- **每隧道独立密钥**：增强安全性和隐蔽性
  - 不同链路使用不同密钥
  - 单条链路被识别不影响其他链路
  - 密钥泄露影响范围可控

**典型场景**：
- 在网络审查严格的地区建立稳定的国际通信链路
- 避免特定协议（如 OpenVPN、WireGuard）被识别和阻断
- 建立长期稳定的备用通信通道

#### 2. 端到端加密通信

即使在不受审查的网络环境中，TrueWord Node 也能提供强加密的端到端通信：

- **IPsec ESP 加密**：
  - 加密算法：AES-256
  - 认证算法：SHA256-HMAC
  - Tunnel 模式封装

- **应用场景**：
  - 企业分支机构互联
  - 数据中心之间的安全传输
  - 云服务与本地网络的安全连接

#### 3. 内网打通和网络互联

通过 GRE 隧道和策略路由，TrueWord Node 可以灵活地连接不同网络：

- **内网互通**：
  - 打通物理隔离的内网
  - 建立虚拟的扁平化网络
  - 无需修改内网路由配置

- **多层级架构**：
  - 支持级联隧道（隧道套隧道）
  - 构建星型或网状拓扑
  - 灵活的流量路由策略

**典型场景**：
- 远程办公接入企业内网
- 多地分支机构组网
- 混合云环境的网络连接

### 设计理念

TrueWord Node 的设计遵循以下原则：

1. **抗审查优先**：通过静态密钥和简化握手，最小化协议特征
2. **安全可靠**：采用成熟的 IPsec 技术栈，经过验证的加密算法
3. **灵活部署**：支持多种网络拓扑和策略路由配置
4. **运维友好**：命令行工具易于自动化和集成

### 与其他方案的对比

| 方案 | 优势 | 劣势 | TrueWord Node 的定位 |
|------|------|------|---------------------|
| OpenVPN | 功能丰富、跨平台 | 协议特征明显，易被识别 | 静态密钥，低特征 |
| WireGuard | 性能优异、现代化 | 握手包特征，可能被阻断 | 无握手协商，抗审查 |
| IPsec (IKE) | 工业标准 | IKE 握手复杂，特征明显 | 静态密钥，无 IKE |
| SSH 隧道 | 简单易用 | SSH 协议易识别，性能较差 | 内核级隧道，性能好 |

TrueWord Node 适用于需要**隐蔽性**和**可靠性**的场景，特别是在网络审查环境下的长期稳定通信。

### 局限性和注意事项

1. **静态密钥管理**：
   - 密钥需手动分发和配置
   - 密钥变更需两端同步更新
   - 适合节点数量有限的场景

2. **Linux 专用**：
   - 依赖 Linux 内核的 IPsec 和 GRE 模块
   - 不支持 Windows 或 macOS

3. **需要 Root 权限**：
   - 修改网络配置和路由表需要管理员权限
   - 建议在专用服务器上部署

## 快速开始

### 安装

```bash
# 克隆或进入项目目录
cd trueword_node_go

# 静态编译（推荐）
make static

# 安装到系统
sudo make install
```

编译后的二进制文件位于 `bin/twnode`

### 5 分钟快速上手

完整教程请查看 **[快速入门指南](docs/getting-started.md)**

```bash
# 1. 初始化系统
sudo twnode init

# 2. 创建 WireGuard 隧道（服务器端）
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_hk \
  --type wireguard --mode server --listen-port 51820

# 3. 复制输出的对端配置命令到另一台服务器执行

# 4. 启动隧道
sudo twnode line start tunnel_hk

# 5. 验证连通性
ping 10.0.0.2
```

**更多教程**:
- [WireGuard 完整配置](docs/tutorials/wireguard-setup.md)
- [GRE over IPsec 配置](docs/tutorials/gre-ipsec-setup.md)
- [策略路由配置](docs/tutorials/policy-routing.md)

## 使用方法

### 初始化

首次使用必须先初始化系统环境:

```bash
sudo twnode init
```

初始化会:
- 启用 IP 转发
- 配置 iptables MASQUERADE
- 创建配置目录
- 生成配置文件

### 查看状态

```bash
sudo twnode status
```

### 创建隧道

#### 方式1: 交互式创建 (推荐)

```bash
# 不带参数自动进入交互模式
sudo twnode line create
```

系统会逐步询问:
- 远程IP地址
- 远程虚拟IP
- 本地IP地址
- 本地虚拟IP
- 隧道名称 (可留空自动生成)
- 认证密钥 (不显示)
- 加密密钥 (不显示)

#### 方式2: 命令行创建

```bash
# 格式: twnode line create <远程IP/远程虚拟IP> <本地IP/本地虚拟IP> [隧道名]
sudo twnode line create 1.2.3.4/10.0.1.2 5.6.7.8/10.0.1.1 tun-test \
  --auth-key "your-auth-passphrase" \
  --enc-key "your-enc-passphrase"
```

**重要**: 两端必须使用相同的密钥才能连接成功

### IPsec 管理

```bash
# 创建 IPsec 连接
sudo twnode ipsec create 1.2.3.4 5.6.7.8 \
  --auth-key "your-auth-key" \
  --enc-key "your-enc-key"

# 删除 IPsec 连接
sudo twnode ipsec remove 1.2.3.4 5.6.7.8
```

### 隧道管理

```bash
# 创建 GRE 隧道 (假设IPsec已建立)
sudo twnode tunnel create 1.2.3.4/10.0.1.2 5.6.7.8/10.0.1.1 tun-test

# 删除隧道
sudo twnode tunnel remove tun-test
```

### 策略路由管理

#### 创建策略组

```bash
# 格式: twnode policy create <组名> <出口接口>
# 优先级自动分配，按创建顺序递增
sudo twnode policy create vpn-group tun-test
```

系统会自动分配优先级，无需手动指定。

#### 添加 CIDR

```bash
# 单个添加
sudo twnode policy add vpn-group 192.168.1.0/24

# 从文件批量导入
sudo twnode policy import vpn-group /path/to/cidrs.txt
```

文件格式(每行一个 CIDR):
```
192.168.1.0/24
10.0.0.0/8
172.16.0.0/12
# 注释行会被忽略
```

#### 管理默认路由

**默认路由**是策略路由中的兜底路由（0.0.0.0/0），优先级最低，匹配所有未被其他策略组匹配的流量。

```bash
# 设置/切换默认路由(0.0.0.0/0)出口并立即应用
sudo twnode policy default tun-test

# 切换到另一个出口
sudo twnode policy default eth0

# 清除默认路由设置(使用系统路由表)并立即应用
sudo twnode policy unset-default
```

**命令说明**:
- `default <exit>`: 设置或切换默认路由出口，**自动应用到内核**
- `unset-default`: 清除默认路由设置，**自动应用到内核**

**注意**:
- 这两个命令都会立即生效，无需手动执行 `apply`
- 如果不设置默认路由，策略路由将只处理策略组中的CIDR，其他流量走系统默认路由

#### 应用策略

```bash
# 应用所有策略到内核
sudo twnode policy apply
```

应用前会自动检查:
- 出口接口是否存在
- 接口是否处于 UP 状态
- 自动添加保护路由(避免影响隧道底层连接)

#### 撤销策略

```bash
# 撤销所有策略路由,恢复原状
sudo twnode policy revoke
```

#### 列出策略组

```bash
sudo twnode policy list
```

## 配置文件

配置文件位于: `/etc/trueword_node/config.yaml`

```yaml
routing:
  default_exit: "tun-test"
```

策略组文件位于: `/etc/trueword_node/policies/`

## 策略路由优先级说明

- **10**: 系统保护路由(自动管理,保护隧道底层连接)
- **100+**: 用户策略组(自动递增分配)
- **900**: 默认路由(0.0.0.0/0,可选的兜底路由)
- **32766**: 主路由表
- **32767**: 系统默认路由表

匹配顺序:
1. 首先保护隧道底层连接的路由(优先级10)
2. 按优先级匹配用户策略组中的CIDR(优先级100+)
3. 如果设置了默认路由,匹配0.0.0.0/0(优先级900)
4. 最后使用系统默认路由表(优先级32766)

**关于默认路由**:
- 默认路由是可选的，用于将所有未匹配的流量指向特定出口
- 不设置时，未匹配的流量将使用系统默认路由表
- 设置后，相当于在策略路由中添加一个0.0.0.0/0的兜底规则

## 技术细节

### IPsec 实现

- 使用 `ip xfrm` 管理 IPsec SA 和 Policy
- 认证: SHA256
- 加密: AES
- 模式: Tunnel
- **每条隧道独立密钥**: 用户字符串通过 SHA256 派生

### 策略路由

- 隧道接口: 使用 `dev` 直连
- 物理接口: 自动获取网关,通过 `via` 路由
- 自动添加保护路由,避免路由环路
- 优先级自动分配,避免冲突

## 注意事项

1. 必须以 root 权限运行
2. **每条隧道使用独立密钥**，两端密钥必须相同
3. iptables 规则不会自动持久化,重启后需重新应用
4. 应用策略前会检查所有出口接口状态
5. 策略组优先级自动分配,创建后不可修改

## 示例场景

### 场景1: 两个节点建立隧道

节点A (IP: 1.2.3.4, VIP: 10.0.1.1):
```bash
# 使用交互式模式
sudo twnode line create
  远程IP地址: 5.6.7.8
  远程虚拟IP: 10.0.1.2
  本地IP地址: 1.2.3.4
  本地虚拟IP: 10.0.1.1
  隧道名称: tun-b
  认证密钥: mySecretAuthKey
  加密密钥: mySecretEncKey
```

节点B (IP: 5.6.7.8, VIP: 10.0.1.2):
```bash
# 使用交互式模式
sudo twnode line create
  远程IP地址: 1.2.3.4
  远程虚拟IP: 10.0.1.1
  本地IP地址: 5.6.7.8
  本地虚拟IP: 10.0.1.2
  隧道名称: tun-a
  认证密钥: mySecretAuthKey  # 必须与节点A相同
  加密密钥: mySecretEncKey   # 必须与节点A相同
```

### 场景2: 设置策略路由

```bash
# 创建策略组: 特定网段走隧道
sudo twnode policy create vpn-routes tun-test

# 批量导入 CIDR
sudo twnode policy import vpn-routes /etc/trueword_node/vpn-networks.txt

# 设置默认路由走物理接口
sudo twnode policy default eth0

# 应用策略
sudo twnode policy apply

# 查看状态
sudo twnode status
```

### 场景3: 使用默认路由作为兜底

```bash
# 创建策略组：特定网段走隧道
sudo twnode policy create special-routes tun-vpn
sudo twnode policy add special-routes 192.168.0.0/16

# 应用策略组
sudo twnode policy apply

# 设置默认路由：其他所有流量走本地网络（立即生效）
sudo twnode policy default eth0

# 稍后切换：让所有未匹配流量走VPN（立即生效）
sudo twnode policy default tun-vpn

# 或者取消默认路由设置，使用系统默认路由表（立即生效）
sudo twnode policy unset-default
```

**说明**:
- 192.168.0.0/16 会走 tun-vpn (策略组)
- 其他流量走 eth0 或 tun-vpn (取决于默认路由设置)
- 不设默认路由时，其他流量走系统路由表

## 命令速查

```bash
# 初始化
sudo twnode init

# 创建隧道(交互式，推荐)
sudo twnode line create

# 创建隧道(命令行)
sudo twnode line create <remote/remote_vip> <local/local_vip> [name] --auth-key "xxx" --enc-key "xxx"

# 创建策略组
sudo twnode policy create <name> <exit>

# 添加CIDR
sudo twnode policy add <group> <cidr>

# 批量导入
sudo twnode policy import <group> <file>

# 应用策略
sudo twnode policy apply

# 设置/切换默认路由(0.0.0.0/0，立即生效)
sudo twnode policy default <exit>

# 清除默认路由(立即生效)
sudo twnode policy unset-default

# 撤销策略
sudo twnode policy revoke

# 查看状态
sudo twnode status

# 列出策略组
sudo twnode policy list
```

## 开发

```bash
# 格式化代码
make fmt

# 代码检查
make vet

# 运行测试
make test

# 清理
make clean
```

## 变更说明

相比PHP版本的改进:
1. **隧道级密钥**: 每条隧道独立密钥,更安全
2. **交互式创建**: 更友好的创建方式
3. **自动优先级**: 策略组优先级自动分配
4. **默认路由切换**: 一键切换命令
5. **静态编译**: 单个可执行文件,无依赖

## 许可证

(根据您的需求添加)
