# 实战教程

本节提供 TrueWord Node 的实战教程，涵盖从基础配置到高级场景的完整指南。

## 📚 教程列表

### 基础配置教程

#### [WireGuard 隧道配置](wireguard-setup.md)
完整的 WireGuard 隧道配置流程，包括服务器/客户端模式、密钥管理、握手机制等。

**适合场景**：
- 两台服务器之间建立快速、安全的隧道
- 动态 IP 客户端连接固定 IP 服务器
- 现代化的 VPN 解决方案

**关键概念**：
- 服务器/客户端模式
- 密钥对生成和管理
- 主动握手机制
- 对端配置命令输出

---

#### [GRE over IPsec 隧道配置](gre-ipsec-setup.md)
GRE + IPsec 双层隧道的完整配置指南，包括密钥生成、双层封装、验证方法等。

**适合场景**：
- 需要传统 VPN 技术的场景
- 双层加密安全需求
- 与旧系统的兼容

**关键概念**：
- IPsec ESP 隧道（加密层）
- GRE 隧道（数据传输层）
- GRE Key 生成规则
- IPsec SPI 对称性

---

#### [策略路由实践](policy-routing.md)
策略路由的实战案例，包括基于目标地址、源地址、优先级的流量分发。

**适合场景**：
- 不同地区流量走不同隧道
- 不同用户/设备使用不同出口
- 多策略组优先级管理

**涵盖内容**：
- 基于目标 CIDR 的流量分发
- 基于源地址的流量控制
- 优先级分层管理
- 默认路由 + 例外策略
- Cost 机制应用

---

### 高级场景教程

#### [多层隧道嵌套](nested-tunnels.md)
TrueWord Node 的核心特性 —— 父接口（Parent Interface）概念和多层隧道架构。

**适合场景**：
- 企业多分支互联（通过中转服务器）
- 高安全性传输（双重/多重加密）
- 动态 IP 客户端通过固定 IP 中转
- 负载均衡和流量分散

**关键概念**：
- 父接口（物理接口 vs 隧道接口）
- 本地 IP 自动获取规则
- 数据包多层封装流程
- 路由表层次关系
- 混合隧道类型（WireGuard + GRE）

**架构示例**：
- 两层隧道：物理接口 → 第一层隧道 → 第二层隧道
- 三层隧道：A ↔ B ↔ C，A 和 C 通过 B 中转
- 混合类型：WireGuard（第一层）+ GRE（第二层）

---

#### [故障转移配置](failover-setup.md)
自动化故障转移方案，包括评分算法、Cron 定时任务、高可用架构设计。

**适合场景**：
- 多条线路容灾备份
- 自动选择最优路径
- 高可用性要求的业务

**涵盖内容**：
- 评分算法详解（丢包率 + 延迟 + Cost）
- 策略组故障转移
- 默认路由故障转移
- 自动化脚本和 Cron 配置
- 监控和告警集成
- 高可用架构设计（主备、多活、分级备份）

**评分公式**：
```
基础评分 = 丢包率评分(60%) + 延迟评分(40%)
最终评分 = 基础评分 - Cost惩罚(Cost × 0.5)
```

---

#### [动态 IP 对端处理](dynamic-ip.md)
WireGuard 服务器接收动态 IP 客户端的完整配置方案，包括保护路由同步机制。

**适合场景**：
- 家庭宽带（动态 IP）连接云服务器
- 移动设备连接固定服务器
- 对端 IP 频繁变化的场景

**关键机制**：
- WireGuard 服务器模式（remote_ip 使用 0.0.0.0）
- 运行时获取对端 IP（`wg show` 命令）
- 保护路由自动同步
- Cron 定时任务配置

**工作流程**：
1. 客户端连接服务器
2. 服务器通过 `wg show` 获取实际对端 IP
3. `sync-protection` 检测 IP 变化
4. 自动更新保护路由规则
5. 清理僵尸规则

---

## 🎯 教程导航

### 按难度分类

**入门级**：
- [WireGuard 隧道配置](wireguard-setup.md)
- [策略路由实践](policy-routing.md)（场景 1-3）

**进阶级**：
- [GRE over IPsec 隧道配置](gre-ipsec-setup.md)
- [动态 IP 对端处理](dynamic-ip.md)
- [故障转移配置](failover-setup.md)（基础部分）

**高级**：
- [多层隧道嵌套](nested-tunnels.md)
- [故障转移配置](failover-setup.md)（自动化和高可用架构）
- [策略路由实践](policy-routing.md)（场景 4-6）

### 按应用场景分类

**点对点连接**：
- [WireGuard 隧道配置](wireguard-setup.md)
- [GRE over IPsec 隧道配置](gre-ipsec-setup.md)

**多地域组网**：
- [多层隧道嵌套](nested-tunnels.md)（三层架构）
- [策略路由实践](policy-routing.md)（多策略组）

**高可用性**：
- [故障转移配置](failover-setup.md)
- [策略路由实践](policy-routing.md)（场景 4）

**动态网络**：
- [动态 IP 对端处理](dynamic-ip.md)
- [多层隧道嵌套](nested-tunnels.md)（场景 3）

**流量管理**：
- [策略路由实践](policy-routing.md)（所有场景）

### 按技术栈分类

**WireGuard**：
- [WireGuard 隧道配置](wireguard-setup.md)
- [动态 IP 对端处理](dynamic-ip.md)
- [多层隧道嵌套](nested-tunnels.md)（WireGuard 部分）

**GRE over IPsec**：
- [GRE over IPsec 隧道配置](gre-ipsec-setup.md)
- [多层隧道嵌套](nested-tunnels.md)（混合类型）

**策略路由**：
- [策略路由实践](policy-routing.md)
- [故障转移配置](failover-setup.md)

**嵌套隧道**：
- [多层隧道嵌套](nested-tunnels.md)（所有场景）

## 📖 学习路径推荐

### 路径 1: 快速上手
1. [WireGuard 隧道配置](wireguard-setup.md)
2. [策略路由实践](policy-routing.md)（场景 1）
3. [故障转移配置](failover-setup.md)（场景 1）

### 路径 2: 传统 VPN
1. [GRE over IPsec 隧道配置](gre-ipsec-setup.md)
2. [策略路由实践](policy-routing.md)
3. [故障转移配置](failover-setup.md)

### 路径 3: 高级架构
1. [WireGuard 隧道配置](wireguard-setup.md)
2. [多层隧道嵌套](nested-tunnels.md)
3. [动态 IP 对端处理](dynamic-ip.md)
4. [故障转移配置](failover-setup.md)（高可用架构）

### 路径 4: 企业组网
1. [WireGuard 隧道配置](wireguard-setup.md)
2. [策略路由实践](policy-routing.md)
3. [多层隧道嵌套](nested-tunnels.md)（场景 1）
4. [故障转移配置](failover-setup.md)（架构 2、3）

## 🔗 相关资源

### 命令参考
- [line 命令](../commands/line/index.md) - 隧道管理命令
- [policy 命令](../commands/policy/index.md) - 策略路由命令

### 参考文档
- [路由表设计](../reference/routing-tables.md) - 路由表和优先级规则
- [保护路由机制](../reference/protection-routes.md) - 防止路由环路
- [配置文件详解](../reference/config-files.md) - 配置文件格式
- [故障排查](../reference/troubleshooting.md) - 常见问题解决

### 基础文档
- [快速入门](../getting-started.md) - 5 分钟快速开始
- [架构设计](../architecture.md) - 核心设计理念

## 💡 提示

- 每个教程都是独立的，可以按需选择阅读
- 建议先阅读 [快速入门](../getting-started.md) 了解基本概念
- 实践时请在测试环境先验证，再应用到生产环境
- 遇到问题可查看 [故障排查](../reference/troubleshooting.md)

---

**导航**: [返回首页](../index.md) | [命令参考](../commands/) | [参考资料](../reference/)
