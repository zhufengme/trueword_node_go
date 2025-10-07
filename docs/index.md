# TrueWord Node 文档中心

欢迎使用 TrueWord Node！这是一个强大的 Linux 网络隧道管理工具，支持 GRE over IPsec 和 WireGuard 隧道，以及灵活的策略路由系统。

## 🚀 快速导航

### 入门指南
- [快速入门](getting-started.md) - 5分钟上手 TrueWord Node
- [架构设计](architecture.md) - 了解系统核心设计理念

### 命令参考

- **[命令总览](commands/index.md)** - 所有命令的完整索引和速查表

#### 系统初始化
- [init - 系统初始化](commands/init.md) - 配置系统环境和扫描网络接口

#### 隧道管理 (line)
- [line 命令总览](commands/line/index.md) - 隧道管理命令概述
- [create - 创建隧道](commands/line/create.md) - 创建 GRE/WireGuard 隧道
- [delete - 删除隧道](commands/line/delete.md) - 删除指定隧道
- [start - 启动隧道](commands/line/start.md) - 启动单个或所有隧道
- [stop - 停止隧道](commands/line/stop.md) - 停止单个或所有隧道
- [list - 列出隧道](commands/line/list.md) - 查看所有隧道状态
- [check - 连通性检查](commands/line/check.md) - 测试隧道连通性和延迟
- [show-peer - 查看对端配置](commands/line/show-peer.md) - 获取 WireGuard 对端配置

#### 策略路由 (policy)
- [policy 命令总览](commands/policy/index.md) - 策略路由命令概述
- [create - 创建策略组](commands/policy/create.md) - 创建新的策略路由组
- [add-cidr - 添加路由规则](commands/policy/add-cidr.md) - 向策略组添加 CIDR
- [remove-cidr - 删除路由规则](commands/policy/remove-cidr.md) - 从策略组删除 CIDR
- [list - 列出策略组](commands/policy/list.md) - 查看所有策略组
- [apply - 应用策略](commands/policy/apply.md) - 应用策略路由规则
- [revoke - 撤销策略](commands/policy/revoke.md) - 撤销策略路由规则
- [delete - 删除策略组](commands/policy/delete.md) - 删除策略组配置
- [set-priority - 调整优先级](commands/policy/set-priority.md) - 修改策略组优先级
- [sync-protection - 同步保护路由](commands/policy/sync-protection.md) - 自动更新保护路由
- [set-default - 设置默认路由](commands/policy/set-default.md) - 配置默认出口
- [failover - 故障转移](commands/policy/failover.md) - 智能选择最佳出口

### 实战教程

- **[教程总览](tutorials/index.md)** - 所有教程的完整索引和学习路径

#### 基础配置
- [配置 WireGuard 隧道](tutorials/wireguard-setup.md) - 完整的 WireGuard 配置流程
- [配置 GRE over IPsec 隧道](tutorials/gre-ipsec-setup.md) - GRE + IPsec 双层隧道配置
- [配置策略路由](tutorials/policy-routing.md) - 策略路由实战案例

#### 高级场景
- [多层隧道嵌套](tutorials/nested-tunnels.md) - 实现隧道链式嵌套
- [故障转移配置](tutorials/failover-setup.md) - 自动化故障转移方案
- [动态 IP 对端处理](tutorials/dynamic-ip.md) - WireGuard 动态 IP 场景

### 参考资料

- **[参考资料总览](reference/index.md)** - 技术参考文档和诊断工具索引

- [配置文件详解](reference/config-files.md) - 所有配置文件格式说明
- [路由表设计](reference/routing-tables.md) - 路由表和优先级规则
- [保护路由机制](reference/protection-routes.md) - 防止路由环路的核心机制
- [故障排查](reference/troubleshooting.md) - 常见问题和解决方案

### 开发者文档
- [开发指南](development.md) - 参与项目开发的指南

## 📋 功能特性

### 隧道技术
- ✅ **GRE over IPsec** - 传统双层隧道（GRE + IPsec 加密）
- ✅ **WireGuard** - 现代高性能 VPN（服务器/客户端模式）
- ✅ **分层嵌套** - 支持多层隧道链式连接
- ✅ **动态 IP 支持** - 自动检测和更新对端 IP 变化

### 策略路由
- ✅ **策略组管理** - 灵活的路由策略组织
- ✅ **优先级控制** - 自动或手动分配优先级（100-899）
- ✅ **源地址过滤** - 支持 from 源地址限制
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

## 🎯 典型应用场景

### 场景 1: 双地域容灾
在两个数据中心之间建立 WireGuard 隧道，配置策略路由实现自动故障转移。

### 场景 2: 分支机构组网
总部与多个分支通过 GRE over IPsec 建立安全隧道，策略路由控制流量走向。

### 场景 3: 跨云服务商连接
在不同云服务商（如 AWS、阿里云、腾讯云）之间建立隧道，实现混合云组网。

### 场景 4: 动态 IP 客户端
使用 WireGuard 服务器模式接收动态 IP 客户端，保护路由同步自动适配 IP 变化。

## 💡 快速开始

```bash
# 1. 编译安装
make static
sudo make install

# 2. 初始化系统
sudo twnode init

# 3. 创建 WireGuard 隧道（服务器端）
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_hk \
  --type wireguard --mode server --listen-port 51820

# 4. 创建策略路由组
sudo twnode policy create vpn_traffic tunnel_hk

# 5. 添加路由规则
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24

# 6. 应用策略
sudo twnode policy apply
```

更多详情请查看 [快速入门](getting-started.md)。

## 📚 版本历史

- **v1.2+** - WireGuard 支持、动态 IP 容错、保护路由同步
- **v1.1** - 优先级自定义、成本机制、优化连通性检查
- **v1.0** - GRE over IPsec、策略路由、故障转移

## 🤝 贡献和反馈

欢迎提交 Issue 和 Pull Request！详见 [开发指南](development.md)。

## 📄 许可证

本项目采用开源许可证，详见 LICENSE 文件。

---

**导航**: [返回顶部](#trueword-node-文档中心) | [快速入门](getting-started.md) | [命令参考](commands/) | [教程](tutorials/)
