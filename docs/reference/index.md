# 参考资料

本节提供 TrueWord Node 的技术参考文档，包括配置文件格式、路由表设计、核心机制说明和故障排查指南。

## 📚 文档列表

### [配置文件详解](config-files.md)
详细说明所有配置文件的格式和字段含义。

**涵盖内容**：
- 全局配置文件（`/etc/trueword_node/config.yaml`）
- 物理接口配置（`/etc/trueword_node/interfaces/physical.yaml`）
- 隧道配置文件（`/etc/trueword_node/tunnels/*.yaml`）
- 策略组配置文件（`/etc/trueword_node/policies/*.json`）
- 运行时数据（`/var/lib/trueword_node/`）

**适用对象**：
- 需要手动编辑配置文件的用户
- 需要批量导入/导出配置的用户
- 需要备份和恢复配置的用户

---

### [路由表设计](routing-tables.md)
Linux 路由表和优先级系统的详细说明。

**核心内容**：
- 路由表架构（表 5、50、80、main、default）
- 优先级体系（5、10、100-899、900、32766、32767）
- 临时测试路由（优先级 5）
- 保护路由（优先级 10）
- 用户策略组（优先级 100-899）
- 默认路由（优先级 900）

**适用对象**：
- 需要深入理解路由规则的用户
- 需要手动调试路由问题的用户
- 需要设计复杂路由策略的用户

**关键命令**：
```bash
# 查看所有路由规则
ip rule show

# 查看特定路由表
ip route show table 50

# 测试路由
ip route get 8.8.8.8
```

---

### [保护路由机制](protection-routes.md)
防止路由环路的核心机制，深入解释保护路由的工作原理。

**核心问题**：
- 为什么需要保护路由？
- 路由环路是如何产生的？
- 保护路由如何防止环路？

**关键机制**：
- 优先级 10 的保护路由规则
- 隧道对端 IP 强制走主路由表
- 动态 IP 场景下的保护路由同步

**工作流程**：
1. 创建隧道时自动添加保护路由
2. 应用策略时验证保护路由存在
3. 动态 IP 变化时自动更新保护路由
4. 定时任务清理僵尸规则

**适用对象**：
- 遇到路由环路问题的用户
- 使用动态 IP 对端的用户
- 需要理解系统内部机制的开发者

---

### [故障排查](troubleshooting.md)
常见问题和解决方案的完整指南。

**涵盖问题**：

**隧道问题**：
- Ping 不通虚拟 IP
- WireGuard 握手失败
- GRE 隧道无法建立
- 隧道频繁断开

**策略路由问题**：
- 策略路由不生效
- 路由环路
- 优先级冲突
- 流量走向错误

**连通性问题**：
- 连通性检查失败
- 延迟异常高
- 丢包率高
- 间歇性故障

**性能问题**：
- MTU 过大导致丢包
- CPU 使用率高
- 带宽受限

**适用对象**：
- 遇到问题需要快速解决的用户
- 需要优化系统性能的用户
- 需要理解常见陷阱的新手

**排查工具**：
```bash
# 隧道状态检查
ip link show <隧道名>
wg show <隧道名>  # WireGuard
ip xfrm state      # IPsec

# 路由检查
ip rule show
ip route show table 50
ip route get <目标IP>

# 连通性检查
ping -c 4 <IP>
traceroute <IP>
```

---

## 🔍 快速查找

### 按问题类型

**配置相关**：
- [配置文件详解](config-files.md) - 配置文件格式和字段
- [故障排查](troubleshooting.md) - 配置错误排查

**路由相关**：
- [路由表设计](routing-tables.md) - 路由规则和优先级
- [保护路由机制](protection-routes.md) - 路由环路防止
- [故障排查](troubleshooting.md) - 路由问题排查

**隧道相关**：
- [配置文件详解](config-files.md) - 隧道配置格式
- [故障排查](troubleshooting.md) - 隧道问题排查

**性能相关**：
- [故障排查](troubleshooting.md) - 性能优化建议

### 按使用阶段

**规划阶段**：
- [路由表设计](routing-tables.md) - 理解路由表架构
- [保护路由机制](protection-routes.md) - 理解保护机制

**配置阶段**：
- [配置文件详解](config-files.md) - 了解配置格式

**运维阶段**：
- [故障排查](troubleshooting.md) - 解决日常问题

**优化阶段**：
- [路由表设计](routing-tables.md) - 优化路由策略
- [故障排查](troubleshooting.md) - 性能调优

## 📖 深度学习路径

### 路径 1: 理解路由系统

1. **基础概念**
   - [路由表设计](routing-tables.md) - 了解路由表和优先级

2. **核心机制**
   - [保护路由机制](protection-routes.md) - 理解保护路由的必要性

3. **实践验证**
   - 使用 `ip rule show` 和 `ip route show table 50` 查看实际配置
   - 使用 `ip route get` 测试路由走向

4. **问题排查**
   - [故障排查](troubleshooting.md) - 路由问题部分

### 路径 2: 配置文件管理

1. **配置格式**
   - [配置文件详解](config-files.md) - 学习所有配置文件格式

2. **手动编辑**
   - 练习手动编辑配置文件
   - 理解各字段的作用

3. **批量操作**
   - 使用脚本批量生成配置
   - 备份和恢复配置

4. **问题排查**
   - [故障排查](troubleshooting.md) - 配置错误排查

### 路径 3: 故障排查专家

1. **常见问题**
   - [故障排查](troubleshooting.md) - 阅读所有常见问题

2. **核心机制**
   - [路由表设计](routing-tables.md) - 理解路由规则
   - [保护路由机制](protection-routes.md) - 理解保护路由

3. **诊断工具**
   - 掌握 `ip`、`wg`、`ping`、`traceroute` 等工具
   - 学会查看日志和配置文件

4. **实践经验**
   - 在测试环境中模拟各种故障
   - 练习使用诊断工具快速定位问题

## 🛠️ 常用诊断命令

### 查看系统状态

```bash
# 查看所有隧道
sudo twnode line list

# 查看所有策略组
sudo twnode policy list

# 查看路由规则
ip rule show

# 查看路由表
ip route show table main
ip route show table 50
ip route show table 80
```

### 隧道诊断

```bash
# WireGuard 隧道
wg show <隧道名>
wg show <隧道名> endpoints
wg show <隧道名> latest-handshakes

# GRE 隧道
ip link show <隧道名>
ip addr show <隧道名>

# IPsec
sudo ip xfrm state
sudo ip xfrm policy
```

### 路由诊断

```bash
# 测试路由
ip route get 8.8.8.8
ip route get 192.168.100.5

# 指定源地址测试
ip route get 8.8.8.8 from 10.10.1.5

# 查看特定优先级规则
ip rule show pref 10
ip rule show pref 100
```

### 连通性测试

```bash
# Ping 测试
ping -c 4 <目标IP>

# 指定接口 Ping
ping -I <接口名> -c 4 <目标IP>

# 路由跟踪
traceroute <目标IP>

# TrueWord Node 连通性检查
sudo twnode line check <隧道名> 8.8.8.8
```

### 日志和配置

```bash
# 查看配置文件
cat /etc/trueword_node/config.yaml
cat /etc/trueword_node/tunnels/<隧道名>.yaml
cat /etc/trueword_node/policies/<策略组名>.json

# 查看运行时数据
cat /var/lib/trueword_node/check_results.json
cat /var/lib/trueword_node/peer_configs/<隧道名>.txt

# 查看撤销命令
cat /var/lib/trueword_node/rev/<隧道名>.rev

# 系统日志（如果有）
journalctl -u twnode -n 100
dmesg | grep -i wireguard
dmesg | grep -i gre
```

## 🔗 相关资源

### 命令参考
- [line 命令](../commands/line/index.md) - 隧道管理命令详解
- [policy 命令](../commands/policy/index.md) - 策略路由命令详解

### 教程
- [WireGuard 配置](../tutorials/wireguard-setup.md) - 完整的 WireGuard 配置教程
- [GRE over IPsec 配置](../tutorials/gre-ipsec-setup.md) - GRE 隧道配置教程
- [策略路由实践](../tutorials/policy-routing.md) - 策略路由实战案例
- [故障转移配置](../tutorials/failover-setup.md) - 高可用架构配置
- [动态 IP 处理](../tutorials/dynamic-ip.md) - 动态 IP 场景配置
- [多层隧道嵌套](../tutorials/nested-tunnels.md) - 嵌套隧道配置

### 基础文档
- [快速入门](../getting-started.md) - 5 分钟快速开始
- [架构设计](../architecture.md) - 核心设计理念

## 💡 提示

- 遇到问题时，先查看 [故障排查](troubleshooting.md) 的常见问题部分
- 理解 [路由表设计](routing-tables.md) 和 [保护路由机制](protection-routes.md) 有助于快速定位路由问题
- 使用 `ip rule show` 和 `ip route show table X` 是诊断路由问题的关键
- 配置文件备份很重要，定期备份 `/etc/trueword_node/` 目录

---

**导航**: [返回首页](../index.md) | [命令参考](../commands/index.md) | [教程](../tutorials/index.md)
