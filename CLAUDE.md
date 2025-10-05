# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

TrueWord Node 是一个 Linux 网络隧道管理工具，用于创建和管理 **GRE over IPsec** 隧道以及策略路由。该项目从 PHP 重写为 Go，支持分层隧道架构。

## 构建命令

```bash
# 动态链接构建
make build
# 或直接: go build -o bin/twnode cmd/main.go

# 静态编译（生产环境推荐）
make static

# 安装到系统
sudo make install

# 格式化代码
make fmt

# 代码检查
make vet

# 运行测试
make test

# 清理构建产物
make clean
```

## 核心架构

### 1. 分层隧道系统（Parent Interface 概念）

这是项目最重要的架构特性：

**父接口（Parent Interface）**：
- 可以是**物理网络接口**（如 eth0, ens33）
- 也可以是**已创建的隧道**（如 tun01, tun02）
- 支持**多层嵌套**：物理接口 → 隧道1 → 隧道2 → 隧道3...

**关键原则**：
- 创建隧道时，用户**选择父接口**，而不是输入本地IP
- **本地IP自动从父接口获取**：
  - 父接口是物理接口 → 使用物理接口的IP地址
  - 父接口是隧道 → 使用该隧道的 LocalVIP（虚拟IP）
- 网关信息仅物理接口拥有，用于策略路由

**实现位置**：
- `pkg/ipsec/tunnel_manager.go` 中的 `getLocalIPFromParent()` 函数
- `pkg/network/parent_interface.go` - 列出所有可用父接口
- `pkg/network/interface.go` - 物理接口扫描和管理

### 2. 双层隧道机制

每个隧道由两层组成：

**第一层：IPsec ESP 隧道**（可选加密层）
- 使用 `ip xfrm` 管理 XFRM state 和 policy
- 认证：SHA256，加密：AES-256
- 模式：Tunnel mode
- 实现：`pkg/ipsec/tunnel.go` 中的 `CreateIPsec()`

**第二层：GRE 隧道**（数据传输层）
- 使用 `ip tunnel add` 创建 GRE 接口
- GRE Key 从认证密钥生成（确保对称性）
- 虚拟IP在GRE接口上配置
- 实现：`pkg/ipsec/tunnel.go` 中的 `(*Tunnel).Create()`

### 3. 路由表架构

项目使用多个 Linux 路由表：

**表 50（Policy Routing Table）**：
- 用于策略路由，确保远程IP通过正确的物理接口路由
- 仅用于物理接口的对端IP路由
- 实现：`pkg/ipsec/tunnel_manager.go` 中的 `setupPolicyRoute()`

**表 80（Virtual IP Table）**：
- 用于虚拟IP（VIP）的路由
- 所有隧道的对端VIP路由到此表
- 优先级规则：`ip rule add from all lookup 80 pref 80`

**策略路由优先级设计**：
- **10**: 系统保护路由（保护隧道底层连接，防止路由环路）
- **100-899**: 用户策略组（自动递增分配）
- **900**: 默认路由 (0.0.0.0/0 兜底路由，可选)
- **32766**: 主路由表
- **32767**: 系统默认路由表

### 4. 撤销机制（Rev Commands）

所有网络操作都会记录对应的撤销命令到 `/var/lib/trueword_node/rev/`：

- 每个隧道的撤销文件：`<tunnel_name>.rev`
- 每个 IPsec 连接的撤销文件：`<ip1>-<ip2>.rev`
- 删除时自动执行撤销命令，确保干净清理

实现：`pkg/ipsec/tunnel.go` 中的 `recordRevCommands()` 和 `executeRevCommands()`

### 5. 配置文件结构

```
/etc/trueword_node/
├── config.yaml                    # 全局配置（默认路由等）
├── interfaces/
│   └── physical.yaml             # 物理接口配置（init时扫描）
├── tunnels/
│   ├── tun01.yaml               # 各个隧道的配置文件
│   └── tun02.yaml
└── policies/
    ├── group1.json              # 策略组配置
    └── group2.json

/var/lib/trueword_node/
└── rev/
    ├── tun01.rev                # 隧道撤销命令
    └── 1.2.3.4-5.6.7.8.rev     # IPsec撤销命令
```

## 重要工作流程

### 初始化流程 (init)

1. 检查 root 权限
2. 检查必需命令（ip, iptables, ping, sysctl）
3. 启用 IP 转发（`net.ipv4.ip_forward=1`）
4. 配置 iptables MASQUERADE
5. **检查旧配置，如果存在则警告并要求确认（必须输入 "yes"）**
6. **清除所有旧配置目录**
7. 重建配置目录结构
8. **扫描物理网络接口**（自动获取IP和网关）
9. **交互式选择要管理的物理接口**
10. 保存物理接口配置到 `/etc/trueword_node/interfaces/physical.yaml`

实现：`pkg/system/init.go` 中的 `Initialize()`

### 隧道创建流程 (line create)

**交互式模式**：
1. **列出所有可用父接口**（物理接口 + 已创建的隧道）
2. **用户选择父接口**
3. 用户输入：remote_ip, remote_vip, local_vip
4. **自动从父接口获取 local_ip**
5. 用户输入认证密钥和加密密钥
6. 验证父接口存在性
7. **设置策略路由**（仅物理接口，确保对端IP通过正确接口）
8. 创建 IPsec 连接（如果启用加密）
9. 创建 GRE 隧道
10. 保存配置到 `/etc/trueword_node/tunnels/<name>.yaml`

**命令行模式**：
```bash
twnode line create <parent_interface> <remote_ip> <remote_vip> <local_vip> [tunnel_name] \
  --auth-key "xxx" --enc-key "xxx"
```

实现：
- `cmd/main.go` - CLI 交互和命令解析
- `pkg/ipsec/tunnel_manager.go` - 核心创建逻辑

### 策略路由流程 (policy)

**策略组管理支持局部操作**：
- 应用：`twnode policy apply` 或 `twnode policy apply <group_name>`
- 撤销：`twnode policy revoke` 或 `twnode policy revoke <group_name>`
- 删除：`twnode policy delete <group_name>`（自动检测并撤销已应用的策略）

**应用流程**：
1. 创建策略组（指定出口接口和可选的 from 源地址限制）
2. 添加 CIDR 到策略组
3. **Apply** 时检查出口接口状态
4. 自动添加保护路由（优先级10，保护隧道底层连接）
5. 应用策略组路由规则（先添加新规则，再清理重复规则，避免网络中断）
6. 可选：设置默认路由（0.0.0.0/0，优先级900）

**故障转移（Failover）流程**：
1. 指定策略组或默认路由及候选出口列表
2. 可选：提供 check_ip 重新检查，或使用 `line check` 的历史结果
3. 根据评分算法（60% 丢包率 + 40% 延迟）选择最佳出口
4. 自动切换策略组或默认路由到最佳出口

实现：`pkg/routing/policy.go`

## 关键技术细节

### GRE Key 生成

GRE Key 必须在隧道两端保持一致。通过对认证密钥字符串的字符求和生成：

```go
func generateGREKey(authKey string) uint32 {
    authKey = strings.TrimPrefix(authKey, "0x")
    var sum uint32
    for _, c := range authKey {
        sum += uint32(c)
    }
    return sum
}
```

### IPsec SPI 生成

SPI（Security Parameter Index）必须对称。通过对IP对排序后生成 MD5 哈希：

```go
func sortIPs(ip1, ip2 string) (string, string)  // 按字典序排序
func generateSPI(ip1, ip2 string) string       // MD5(ip1+ip2)[:8]
```

### 策略规则管理（关键）

**避免网络中断的规则更新策略**：

在 `ApplyGroup()` 和 `applyDefaultRoute()` 中，采用"先添加后清理"的策略：

1. **先添加新规则**（`ip rule add`）
2. **循环检测并清理重复规则**：
   - 使用 `ip rule show pref X | wc -l` 检查规则数量
   - 如果数量 > 1，删除一个
   - 重复直到只剩一个规则
3. **最后验证规则存在**，如被意外删除则重新添加

这确保：
- 始终至少有一个规则存在，不会中断网络
- 清理重复规则，避免冲突
- 有验证和恢复机制

**错误的做法**（会导致网络中断）：
```go
// 错误：先删除旧规则，再添加新规则
ip rule del pref X  // 删除规则
ip rule add ...     // 添加规则（中间有时间窗口，流量无法路由）
```

### 网络操作库

项目使用 **github.com/vishvananda/netlink** 库进行网络配置，而不是直接调用系统命令。仅在必需时（如 `ip xfrm`）才使用 `exec.Command`。

### 连通性检查和评分

**检查结果持久化**：
- 检查结果保存到 `/var/lib/trueword_node/check_results.json`
- 使用 `network.CheckInterface()` 执行检查
- 使用 `network.SaveCheckResults()` 保存结果
- Failover 功能依赖这些检查结果

**评分算法**：
- 丢包率评分：0-60 分（用户对丢包更敏感）
- 延迟评分：0-40 分
- 总分：0-100 分，分数越高越好

## UI 设计原则

- 命令执行时**不显示具体命令**，只显示结果
- **仅在出错时**显示命令和错误输出
- 使用框线装饰重要界面（`╔═══╗`）
- 使用中文分组标记（【配置信息】、【建立连接】）
- 使用图标增强可读性（✓ ✗ ⚠️）
- 保持输出简洁、美观、易读

实现示例：`pkg/ipsec/tunnel.go` 中的 `execCommand()` 函数

## 常见开发任务

### 添加新的隧道功能

1. 在 `pkg/ipsec/tunnel.go` 或 `tunnel_manager.go` 中添加核心逻辑
2. 记录撤销命令到 rev 文件
3. 在 `cmd/main.go` 中添加 CLI 命令
4. 更新配置结构（如需要）：`pkg/network/tunnel_config.go`

### 添加新的策略路由功能

1. 在 `pkg/routing/policy.go` 中添加逻辑
2. **注意策略规则管理**：必须使用"先添加后清理"的策略避免网络中断
3. 注意优先级范围限制（100-899 为用户策略组范围）
4. 在 `cmd/main.go` 中添加子命令
5. 考虑是否需要支持局部操作（单个策略组 vs 所有策略）

### 修改物理接口扫描逻辑

修改 `pkg/network/interface.go` 中的 `ScanPhysicalInterfaces()` 函数，该函数使用 netlink 库扫描接口。

### 修改策略应用逻辑时的注意事项

**关键原则**：策略规则的更新必须保证无缝切换，避免网络中断

- ❌ 不要先删除旧规则再添加新规则
- ✓ 应该先添加新规则，再清理重复规则
- ✓ 使用循环检测确保只保留一个规则
- ✓ 添加验证和恢复机制

参考 `ApplyGroup()` 和 `applyDefaultRoute()` 中的实现。

## 依赖项

```
require (
    github.com/spf13/cobra              // CLI 框架
    github.com/vishvananda/netlink      // 网络接口管理
    gopkg.in/yaml.v3                    // 配置文件解析
)
```

## 测试注意事项

- 所有网络操作需要 **root 权限**
- 测试环境需要 Linux 内核支持 GRE 和 XFRM
- 测试前运行 `sudo twnode init` 初始化环境
- 清理测试环境：删除测试隧道，运行 `twnode policy revoke`

## 语言和文档

- **代码注释**：使用简体中文
- **CLI 输出**：使用简体中文
- **用户交互**：使用简体中文
- **变量命名**：使用英文
- 请你不要在修改完成后自动的帮我commit和push，除非我明确告诉你
- 以后请坚持用tablewriter 库来解决表格显示问题
- 请始终静态编译，以便在任何系统上都能运行二进制文件