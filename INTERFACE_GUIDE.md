# 物理接口管理和上层链路功能

## 概述

本项目现已支持物理接口(上层链路)管理功能,参考原PHP项目的设计,使用Go语言的现代化实现。

## 核心概念

### 1. 物理接口(Physical Interface)
- **定义**: 实际的网卡设备(如 eth0, ens33等)
- **作用**: 作为所有隧道的基础,隧道必须通过物理接口建立
- **配置**: 包含接口名、IP地址、网关等信息

### 2. 上层链路(Parent Line)
- **定义**: 隧道的父接口,可以是物理接口或另一个隧道
- **层级**: 支持多层嵌套(物理接口 -> 隧道1 -> 隧道2)
- **策略路由**: 自动根据物理接口的网关配置策略路由

### 3. 策略路由
- **目的**: 确保隧道流量通过正确的物理接口
- **实现**: 使用Linux策略路由表(table 50)
- **网关**: 自动从物理接口配置中获取

## 技术实现

### 使用的Go库
- **github.com/vishvananda/netlink**: Linux网络配置(替代原有的shell命令)
  - 接口扫描
  - 路由管理
  - 规则管理
- **gopkg.in/yaml.v3**: 配置文件管理

### 关键改进
1. **使用netlink库**: 不再依赖shell命令解析,更稳定可靠
2. **类型安全**: Go的类型系统保证数据安全
3. **自动网关检测**: 通过netlink API自动检测网关
4. **配置持久化**: YAML格式配置文件,易于管理

## 使用流程

### 1. 初始化系统
```bash
sudo ./bin/twnode init
```

初始化过程会:
- 启用IP转发
- 配置iptables MASQUERADE
- **扫描物理网络接口**
- **保存接口配置(包括网关信息)**

### 2. 查看物理接口
```bash
sudo ./bin/twnode interface list
```

输出示例:
```
物理网络接口:
============================================================

✓ eth0
  IP地址: 192.168.1.100
  网关: 192.168.1.1
  状态: 已启用
```

### 3. 重新扫描接口
```bash
sudo ./bin/twnode interface scan
```

### 4. 创建隧道(自动使用物理接口)

#### 方式1: 交互式创建(推荐)
```bash
sudo ./bin/twnode line create
```

系统会自动:
1. 从配置中读取可用的物理接口
2. 验证父接口状态
3. 获取本地IP(从父接口)
4. 获取网关(从父接口)
5. **自动设置策略路由**
6. 创建IPsec连接和GRE隧道

#### 方式2: 命令行模式
```bash
sudo ./bin/twnode line create \
  1.2.3.4/10.0.0.2 \
  192.168.1.100/10.0.0.1 \
  tun01 \
  --auth-key myauthkey \
  --enc-key myenckey
```

## 配置文件

### 物理接口配置
**位置**: `/etc/trueword_node/interfaces/physical.yaml`

```yaml
interfaces:
  - name: eth0
    ip: 192.168.1.100
    gateway: 192.168.1.1
    enabled: true
  - name: eth1
    ip: 10.0.0.1
    gateway: ""  # P2P连接,无网关
    enabled: true
```

### 隧道配置
**位置**: `/etc/trueword_node/tunnels/<tunnel_name>.yaml`

```yaml
name: tun01
parent_interface: eth0      # 指定父接口
local_ip: 192.168.1.100     # 从eth0自动获取
remote_ip: 1.2.3.4
local_vip: 10.0.0.1
remote_vip: 10.0.0.2
auth_key: 0x...
enc_key: 0x...
enabled: true
use_encryption: true
```

## 策略路由说明

### 路由表配置
- **表50**: 策略路由表,用于强制指定流量路径
- **优先级**: 50

### 自动配置规则
创建隧道时,系统会自动:
```bash
# 添加路由规则(如不存在)
ip rule add from all lookup 50 pref 50

# 添加目标路由
ip route add <remote_ip>/32 via <gateway> dev <parent_interface> table 50
```

### 示例场景

#### 场景: 多网卡环境
```
物理接口:
  eth0: 192.168.1.100, 网关 192.168.1.1
  eth1: 10.0.0.1, 网关 10.0.0.254

隧道:
  tun01: 父接口=eth0, 远程=1.2.3.4
  tun02: 父接口=eth1, 远程=5.6.7.8
```

**策略路由配置(自动)**:
```
# 确保1.2.3.4通过eth0路由
ip route add 1.2.3.4/32 via 192.168.1.1 dev eth0 table 50

# 确保5.6.7.8通过eth1路由
ip route add 5.6.7.8/32 via 10.0.0.254 dev eth1 table 50
```

## 与原PHP项目的对比

| 功能 | PHP实现 | Go实现 |
|------|---------|--------|
| 接口扫描 | `ifconfig`命令解析 | netlink API |
| 网关检测 | `ip route`命令解析 | netlink RouteList |
| 配置存储 | INI格式 | YAML格式 |
| 路由管理 | shell命令 | netlink API |
| 错误处理 | 字符串匹配 | 类型化错误 |
| 并发安全 | 无 | Goroutine-safe |

## 调试

### 查看策略路由
```bash
# 查看路由规则
ip rule list

# 查看路由表50
ip route show table 50
```

### 查看系统状态
```bash
sudo ./bin/twnode status
```

## 注意事项

1. **Root权限**: 所有网络操作需要root权限
2. **网关检测**: 如果自动检测失败,初始化时可手动输入
3. **P2P连接**: 无网关的点对点连接是支持的
4. **配置备份**: 建议定期备份 `/etc/trueword_node` 目录
5. **netlink依赖**: 确保系统内核支持netlink(Linux 2.6+)

## 故障排查

### 问题: 无法检测到网关
**原因**: 可能使用DHCP或特殊网络配置
**解决**: 初始化时手动输入网关地址

### 问题: 策略路由不生效
**检查步骤**:
1. `ip rule list` 查看规则是否存在
2. `ip route show table 50` 查看路由是否正确
3. `ip route get <remote_ip>` 测试路由路径

### 问题: 隧道无法连接
**检查步骤**:
1. 验证物理接口状态: `twnode interface list`
2. 检查父接口是否启用
3. 验证网关可达性: `ping <gateway>`
4. 检查远程IP路由: `ip route get <remote_ip>`
