# TrueWord Node

TrueWord Node 是一个用于管理 GRE over IPsec 隧道和策略路由的命令行工具，使用 Golang 重写。

## 特性

- **隧道级密钥**: 每条隧道使用独立的认证和加密密钥
- **交互式创建**: 支持交互式问答创建隧道，方便输入参数
- **IPsec 管理**: 基于静态密钥的 IPsec 连接管理
- **GRE 隧道**: 在 IPsec 之上创建 GRE 隧道
- **智能策略路由**: 自动分配优先级，避免冲突
- **默认路由切换**: 一键切换默认路由出口
- **自动化保护**: 自动保护隧道底层连接，避免路由环路
- **批量导入**: 支持从文件批量导入 CIDR 地址

## 安装

### 从源码编译

```bash
# 克隆或进入项目目录
cd trueword_node_go

# 下载依赖
make deps

# 静态编译
make static

# 安装到系统(可选)
sudo make install
```

编译后的二进制文件位于 `bin/twnode`

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
