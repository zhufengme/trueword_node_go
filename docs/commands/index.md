# 命令参考

TrueWord Node 提供了一套完整的命令行工具，用于管理网络隧道和策略路由。

## 📋 命令总览

### [init - 系统初始化](init.md)
初始化 TrueWord Node 运行环境，包括：
- 检查系统环境和必需命令
- 启用 IP 转发
- 配置 iptables MASQUERADE
- 扫描物理网络接口
- 创建配置目录结构

**使用场景**：首次安装后运行一次

---

### [line - 隧道管理](line/index.md)
管理 GRE over IPsec 和 WireGuard 隧道的完整生命周期。

**子命令**：
- [create](line/create.md) - 创建新隧道
- [delete](line/delete.md) - 删除隧道
- [start](line/start.md) - 启动隧道
- [stop](line/stop.md) - 停止隧道
- [list](line/list.md) - 列出所有隧道
- [check](line/check.md) - 检查隧道连通性
- [show-peer](line/show-peer.md) - 显示 WireGuard 对端配置

**主要功能**：
- 支持 GRE over IPsec 和 WireGuard 两种隧道类型
- 父接口（Parent Interface）概念，支持多层嵌套
- 自动密钥生成和管理（WireGuard）
- 连通性检查和评分
- 撤销机制（完整清理）

---

### [policy - 策略路由](policy/index.md)
管理基于 CIDR 的策略路由规则，实现灵活的流量控制。

**策略组管理**：
- [create](policy/create.md) - 创建策略组
- [delete](policy/delete.md) - 删除策略组
- [list](policy/list.md) - 列出策略组
- [set-priority](policy/set-priority.md) - 调整优先级

**CIDR 管理**：
- [add-cidr](policy/add-cidr.md) - 添加路由规则
- [remove-cidr](policy/remove-cidr.md) - 删除路由规则

**策略应用**：
- [apply](policy/apply.md) - 应用策略路由
- [revoke](policy/revoke.md) - 撤销策略路由

**默认路由**：
- [set-default](policy/set-default.md) - 设置/取消默认路由

**高级功能**：
- [sync-protection](policy/sync-protection.md) - 同步保护路由（动态 IP 容错）
- [failover](policy/failover.md) - 智能故障转移

**主要功能**：
- 优先级控制（100-899，自动或手动分配）
- 源地址过滤（from 参数）
- 保护路由自动管理
- 无缝规则切换（避免网络中断）
- 连通性评分和自动故障转移

---

## 🎯 常用命令速查

### 初始化和基础操作

```bash
# 系统初始化（首次运行）
sudo twnode init

# 查看所有隧道
sudo twnode line list

# 查看所有策略组
sudo twnode policy list
```

### 创建 WireGuard 隧道

```bash
# 服务器模式
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_hk \
  --type wireguard \
  --mode server \
  --listen-port 51820

# 客户端模式（使用服务器提供的命令）
sudo twnode line create eth0 203.0.113.10 10.0.0.1 10.0.0.2 tunnel_hk \
  --type wireguard \
  --mode client \
  --private-key 'xxx' \
  --peer-pubkey 'xxx' \
  --peer-port 51820
```

### 创建 GRE over IPsec 隧道

```bash
# 创建加密隧道
sudo twnode line create eth0 198.51.100.20 10.0.0.2 10.0.0.1 tunnel_ab \
  --auth-key "0x1a2b3c..." \
  --enc-key "0x9f8e7d..." \
  --encrypt
```

### 配置策略路由

```bash
# 创建策略组
sudo twnode policy create vpn_traffic tunnel_hk

# 添加 CIDR
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24

# 应用策略
sudo twnode policy apply vpn_traffic

# 设置默认路由
sudo twnode policy set-default tunnel_hk
```

### 连通性检查和故障转移

```bash
# 检查隧道连通性
sudo twnode line check tunnel_hk 8.8.8.8

# 故障转移
sudo twnode policy failover vpn_traffic tunnel_hk,tunnel_us \
  --check-ip 8.8.8.8

# 同步保护路由
sudo twnode policy sync-protection
```

### 隧道生命周期管理

```bash
# 启动隧道
sudo twnode line start tunnel_hk

# 停止隧道
sudo twnode line stop tunnel_hk

# 启动所有隧道
sudo twnode line start-all

# 删除隧道
sudo twnode line delete tunnel_hk
```

## 🔍 命令分类

### 按功能分类

**隧道管理**：
- [line create](line/create.md)
- [line delete](line/delete.md)
- [line start](line/start.md)
- [line stop](line/stop.md)
- [line list](line/list.md)

**路由管理**：
- [policy create](policy/create.md)
- [policy add-cidr](policy/add-cidr.md)
- [policy apply](policy/apply.md)
- [policy list](policy/list.md)

**连通性和故障转移**：
- [line check](line/check.md)
- [policy failover](policy/failover.md)
- [policy sync-protection](policy/sync-protection.md)

**配置查看**：
- [line list](line/list.md)
- [line show-peer](line/show-peer.md)
- [policy list](policy/list.md)

### 按使用频率分类

**高频命令**（日常使用）：
- `line list` - 查看隧道状态
- `policy list` - 查看策略组
- `line check` - 检查连通性
- `policy apply` - 应用策略

**中频命令**（配置变更）：
- `line create` - 创建隧道
- `policy create` - 创建策略组
- `policy add-cidr` - 添加路由规则
- `line start/stop` - 启停隧道

**低频命令**（首次配置或维护）：
- `init` - 系统初始化
- `line delete` - 删除隧道
- `policy delete` - 删除策略组
- `policy set-priority` - 调整优先级

**高级命令**（自动化和高可用）：
- `policy failover` - 故障转移
- `policy sync-protection` - 保护路由同步
- `policy set-default` - 默认路由管理

## 📖 学习建议

### 入门顺序

1. **系统初始化**
   - [init](init.md) - 了解系统初始化流程

2. **创建第一个隧道**
   - [line create](line/create.md) - 创建 WireGuard 隧道
   - [line list](line/list.md) - 查看隧道状态

3. **配置第一个策略组**
   - [policy create](policy/create.md) - 创建策略组
   - [policy add-cidr](policy/add-cidr.md) - 添加路由规则
   - [policy apply](policy/apply.md) - 应用策略

4. **测试和验证**
   - [line check](line/check.md) - 检查连通性

5. **高级功能**
   - [policy failover](policy/failover.md) - 故障转移
   - [policy sync-protection](policy/sync-protection.md) - 保护路由同步

### 进阶学习

- 阅读 [架构设计](../architecture.md) 理解核心概念
- 学习 [实战教程](../tutorials/index.md) 了解实际应用场景
- 查看 [参考资料](../reference/index.md) 深入了解技术细节

## 🔗 相关资源

- [快速入门](../getting-started.md) - 5 分钟快速上手
- [实战教程](../tutorials/index.md) - 完整的配置案例
- [参考资料](../reference/index.md) - 技术细节和故障排查

---

**导航**: [返回首页](../index.md) | [line 命令](line/index.md) | [policy 命令](policy/index.md)
