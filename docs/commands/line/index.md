# line 命令 - 隧道管理

## 概述

`line` 命令用于管理 GRE over IPsec 和 WireGuard 隧道，包括创建、删除、启动、停止、检查连通性等操作。

## 子命令列表

### 隧道生命周期

- [create](create.md) - 创建新隧道（GRE over IPsec 或 WireGuard）
- [delete](delete.md) - 删除已有隧道
- [start](start.md) - 启动隧道
- [start-all](start.md#start-all) - 启动所有隧道
- [stop](stop.md) - 停止隧道
- [stop-all](stop.md#stop-all) - 停止所有隧道

### 隧道信息

- [list](list.md) - 列出所有隧道及状态
- [show-peer](show-peer.md) - 显示 WireGuard 对端配置命令

### 连通性检查

- [check](check.md) - 检查隧道连通性和延迟

## 快速参考

### 创建隧道

```bash
# WireGuard 服务器模式
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_ab \
  --type wireguard --mode server --listen-port 51820

# WireGuard 客户端模式
sudo twnode line create eth0 203.0.113.50 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard --mode client \
  --private-key 'xxx' --peer-pubkey 'yyy' --peer-port 51820

# GRE over IPsec（交互式）
sudo twnode line create
```

### 管理隧道

```bash
# 启动隧道
sudo twnode line start tunnel_ab

# 启动所有隧道
sudo twnode line start-all

# 停止隧道
sudo twnode line stop tunnel_ab

# 列出所有隧道
sudo twnode line list

# 删除隧道
sudo twnode line delete tunnel_ab
```

### 检查隧道

```bash
# 检查连通性
sudo twnode line check tunnel_ab 8.8.8.8

# 查看 WireGuard 对端配置
sudo twnode line show-peer tunnel_ab
```

## 隧道类型

### GRE over IPsec

**双层隧道结构**:
- 第一层: IPsec ESP（加密层）
- 第二层: GRE（数据传输层）

**特点**:
- ✅ 传统、成熟、广泛支持
- ✅ 灵活的加密选项（可选启用/禁用）
- ✅ 支持多层嵌套
- ⚠️ 配置相对复杂

**适用场景**: 需要与传统设备互通、要求灵活加密控制

### WireGuard

**现代 VPN 隧道**:
- 单层隧道，内置加密
- 使用 Curve25519 非对称加密

**特点**:
- ✅ 性能优异，资源占用低
- ✅ 配置简单，密钥管理自动化
- ✅ 支持 NAT 穿透
- ✅ 支持动态 IP 对端
- ⚠️ 需要内核支持（Linux 5.6+）

**适用场景**: 现代化部署、高性能要求、动态 IP 客户端

## 父接口概念

创建隧道时需要指定**父接口**（Parent Interface），它是隧道的底层传输接口：

- **物理接口** - 如 `eth0`, `ens33`
- **已创建的隧道** - 如 `tun01`, `wg0`（实现多层嵌套）

**自动 IP 获取**:
- 父接口是物理接口 → 使用物理接口的 IP 地址
- 父接口是隧道 → 使用该隧道的虚拟 IP（LocalVIP）

详见 [架构设计 - 分层隧道系统](../../architecture.md#分层隧道系统)

## 隧道状态

隧道有以下几种状态：

- **Active** - 隧道已启动，接口处于 UP 状态
- **Inactive** - 隧道已创建但未启动
- **Error** - 隧道启动失败或配置错误

使用 `line list` 查看所有隧道状态。

## 配置文件位置

隧道配置文件保存在:

```
/etc/trueword_node/tunnels/
├── tunnel_ab.yaml
├── tunnel_cd.yaml
└── ...
```

配置文件格式参见 [配置文件详解](../../reference/config-files.md#隧道配置)

## 示例工作流

### 场景1: 两台服务器建立 WireGuard 隧道

**服务器 A**（192.168.1.100）:
```bash
# 1. 创建服务器端隧道
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_ab \
  --type wireguard --mode server --listen-port 51820

# 2. 复制输出的对端配置命令

# 3. 启动隧道
sudo twnode line start tunnel_ab

# 4. 验证连通性
ping 10.0.0.2
```

**服务器 B**（203.0.113.50）:
```bash
# 1. 粘贴服务器 A 输出的对端配置命令，替换父接口
sudo twnode line create eth0 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard --mode client \
  --private-key 'xxx' --peer-pubkey 'yyy' --peer-port 51820

# 2. 启动隧道
sudo twnode line start tunnel_ba

# 3. 验证连通性
ping 10.0.0.1
```

### 场景2: 多层隧道嵌套

```bash
# 第一层: 物理接口 → GRE over IPsec
sudo twnode line create eth0 203.0.113.50 10.0.0.2 10.0.0.1 tun01

# 第二层: 基于第一层隧道 → WireGuard
sudo twnode line create tun01 10.0.0.2 172.16.0.2 172.16.0.1 tun02 \
  --type wireguard --mode server --listen-port 51821

# 启动所有隧道
sudo twnode line start-all
```

## 下一步

- [创建隧道详解](create.md) - 了解所有创建选项
- [WireGuard 完整教程](../../tutorials/wireguard-setup.md)
- [GRE over IPsec 完整教程](../../tutorials/gre-ipsec-setup.md)

---

**导航**: [← 命令参考](../../index.md#命令参考) | [返回首页](../../index.md) | [create →](create.md)
