# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

TrueWord Node 是一个 Linux 网络隧道管理工具，用于创建和管理 **GRE over IPsec** 和 **WireGuard** 隧道以及策略路由。该项目从 PHP 重写为 Go，支持分层隧道架构。

**当前版本**：v1.4+

**最新特性**（v1.4+）：
- **故障转移守护进程**（毫秒级自动故障转移，支持多任务监控和配置热重载）
- **切换确认机制**（防止网络抖动导致频繁切换，支持全局和单任务配置）
- **物理接口检测修复**（守护进程正确检测物理接口状态）

**核心特性**：
- **GRE over IPsec** 和 **WireGuard** 双隧道技术支持
- **分层隧道嵌套**（隧道可以作为其他隧道的父接口）
- **动态IP容错**（自动检测对端IP变化，智能更新保护路由）
- **保护路由机制**（防止路由环路，优先级10保护隧道底层连接）
- **智能评分算法**（丢包率60% + 延迟40% - 成本惩罚）
- **策略路由优先级系统**（5/10/100-899/900多层次优先级）
- **撤销机制**（所有网络操作可完全回退）
- **静态编译**（单二进制文件，无依赖）
- **完整文档系统**（37个文档，涵盖命令参考、教程和技术参考）

## 文档系统

项目包含完整的 Wiki 式文档系统（位于 `docs/` 目录）：

**文档入口**：`docs/index.md` - 文档中心首页

**文档结构**（37个文档）：
- **命令参考**（18个）：`docs/commands/` - 所有命令的详细说明
  - `init.md` - 系统初始化
  - `line/` (8个) - 隧道管理命令
  - `policy/` (9个) - 策略路由命令
- **实战教程**（6个）：`docs/tutorials/` - 从基础到高级的完整配置案例
  - WireGuard/GRE 隧道配置
  - 策略路由实践（6个场景）
  - 多层隧道嵌套
  - 故障转移配置
  - 动态IP处理
- **技术参考**（4个）：`docs/reference/` - 配置文件、路由表、故障排查
- **索引页面**（3个）：快速导航和学习路径

**重要**：
- 添加新功能后，应同时更新相关文档
- 文档使用简体中文，保持与CLI输出风格一致
- 文档间通过相对路径交叉链接
- README.md 包含指向文档中心的链接

## 构建命令

**重要：始终使用静态编译，确保二进制文件可在任何 Linux 系统上运行**

```bash
# 静态编译（推荐，默认方式）
make static
# 生成: bin/twnode (静态链接，无依赖)

# 验证静态编译
file bin/twnode          # 应显示 "statically linked"
ldd bin/twnode           # 应显示 "not a dynamic executable"

# 安装到系统
sudo make install

# 跨平台编译（用于发布）
./release.sh
# 生成: bin/release/twnode-v{version}-{os}-{arch}.tar.gz
# 支持平台: linux/amd64, linux/arm64, linux/386, linux/arm

# 开发辅助命令
make fmt                 # 格式化代码
make vet                 # 代码检查
make test                # 运行测试
make clean               # 清理构建产物
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

### 2. 隧道类型支持

项目支持两种隧道类型，通过 `TunnelType` 字段区分：

#### 2.1 GRE over IPsec 隧道（传统方式）

**双层结构**：

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

#### 2.2 WireGuard 隧道（现代方式）

**核心特性**：
- 使用 Curve25519 非对称加密
- 每端都有自己的密钥对（私钥 + 公钥）
- 支持服务器/客户端模式
- 内置加密，无需额外的 IPsec 层

**密钥管理**：
- 服务器模式：自动生成两端的密钥对，输出完整的对端配置命令
- 客户端模式：使用服务器提供的私钥和对端公钥
- 私钥安全存储在配置文件中（`/etc/trueword_node/tunnels/*.yaml`）
- 对端配置保存在 `/var/lib/trueword_node/peer_configs/`

**端口管理**（v1.3+）：
- 交互式创建WireGuard服务器隧道时，自动检测可用的UDP端口
- 从51820开始查找，找到第一个未被占用的端口作为默认值
- 用户可直接回车使用默认端口，或手动输入其他端口
- 避免端口冲突导致的创建失败
- 实现：`cmd/main.go` 中的 `isPortAvailable()` 和 `findAvailablePort()`

**握手机制**（重要）：
- WireGuard 采用"静默协议"，不主动发送握手包
- 握手由数据包触发（首次需要 5-10 秒）
- 客户端模式：主动发送 ping 包触发握手
- 服务器模式：被动等待客户端连接
- 实现主动握手：`triggerWireGuardHandshake()` 和 `waitForWireGuardHandshake()`

**实现位置**：
- `pkg/wireguard/tunnel.go` - WireGuard 隧道核心逻辑
- `pkg/wireguard/keygen.go` - 密钥生成和管理
- `pkg/ipsec/tunnel_manager.go` - 统一的隧道管理（包含类型分发）

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
- **5**: 临时测试路由（check 和 failover 使用，执行后立即清理）
- **10**: 系统保护路由（保护隧道底层连接，防止路由环路）
- **100-899**: 用户策略组（支持自动分配或手动指定）
- **900**: 默认路由 (0.0.0.0/0 兜底路由，可选)
- **32766**: 主路由表
- **32767**: 系统默认路由表

**注意**：优先级 5 的临时测试路由优先级最高，确保测试流量不受用户策略干扰，但仅在测试期间存在（通过 defer 清理）

### 4. 保护路由机制（关键特性）

**问题背景**：
- 当隧道设置为策略路由出口时，可能导致路由环路
- 例如：WireGuard 隧道作为默认出口，但握手包本身需要通过物理接口发送

**解决方案**：
- 优先级10的保护路由确保隧道对端IP的流量不走策略路由
- 规则：`ip rule add to <对端IP> lookup main pref 10`
- 这保证隧道底层连接始终通过正确的物理接口

**动态IP容错**（重要）：
- 配置文件中的 `ProtectedIP` 字段记录当前保护的IP
- `policy sync-protection` 命令自动检测IP变化并更新
- 适合 WireGuard 服务器接收动态IP客户端的场景
- 自动清理僵尸规则（无对应隧道的保护路由）

**自动同步机制**：
- `policy apply` → 自动执行 `sync-protection`
- `line start` → 启动后自动执行 `sync-protection`
- `line start-all` → 批量启动后自动执行 `sync-protection`
- 用户也可配置 cron 定时任务：`*/5 * * * * twnode policy sync-protection`

**实现位置**：
- `pkg/routing/policy.go` 中的 `SyncProtection()` 函数
- `pkg/wireguard/tunnel.go` 中的 `GetWireGuardPeerEndpoint()` 函数（获取运行时对端IP）

### 5. 撤销机制（Rev Commands）

所有网络操作都会记录对应的撤销命令到 `/var/lib/trueword_node/rev/`：

- 每个隧道的撤销文件：`<tunnel_name>.rev`
- 每个 IPsec 连接的撤销文件：`<ip1>-<ip2>.rev`
- 删除时自动执行撤销命令，确保干净清理

实现：
- `pkg/ipsec/tunnel.go` 中的 `recordRevCommands()` 和 `executeRevCommands()`
- `pkg/wireguard/tunnel.go` 中的 WireGuard 撤销逻辑

### 6. 配置文件结构

```
/etc/trueword_node/
├── config.yaml                    # 全局配置（默认路由等）
├── failover_daemon.yaml           # 故障转移守护进程配置（v1.4+）
├── interfaces/
│   └── physical.yaml             # 物理接口配置（init时扫描）
├── tunnels/
│   ├── tun01.yaml               # 各个隧道的配置文件
│   └── tun02.yaml
└── policies/
    ├── group1.json              # 策略组配置
    └── group2.json

/var/lib/trueword_node/
├── rev/
│   ├── tun01.rev                # 隧道撤销命令
│   └── 1.2.3.4-5.6.7.8.rev     # IPsec撤销命令
├── peer_configs/
│   └── tunnel_name.txt          # WireGuard 对端配置命令
├── check_results.json           # 连通性检查结果
└── failover_state.json          # 守护进程运行时状态（v1.4+）
```

## 重要工作流程

### 初始化流程 (init)

1. 检查 root 权限
2. 检查必需命令（ip, iptables, ping, sysctl）
3. **启用 IP 转发**（`net.ipv4.ip_forward=1`）
   - 临时启用（当前会话）：`sysctl -w net.ipv4.ip_forward=1`
   - **智能检测持久化**：检查 `/etc/sysctl.d/99-trueword-node.conf` 是否存在
   - 如未持久化，询问用户是否需要持久化
   - 如已持久化，直接显示"✓ 已持久化"，不重复询问
4. **配置 iptables MASQUERADE**
   - 临时添加规则（当前会话）：`iptables -t nat -A POSTROUTING -j MASQUERADE`
   - **智能检测持久化**：检查 systemd service 是否已启用
   - 如未持久化，询问用户是否需要持久化（通过 systemd service）
   - 如已持久化，直接显示"✓ 已持久化"，不重复询问
5. **检查旧配置，如果存在则警告并要求确认（必须输入 "yes"）**
6. **清除所有旧配置目录**
7. 重建配置目录结构
8. **扫描物理网络接口**（自动获取IP和网关）
9. **交互式选择要管理的物理接口**
10. 保存物理接口配置到 `/etc/trueword_node/interfaces/physical.yaml`

**持久化机制**：
- IP转发：写入 `/etc/sysctl.d/99-trueword-node.conf`，系统重启后自动加载
- iptables规则：创建 systemd service (`twnode-iptables.service`)，开机自动应用规则
- 智能检测避免重复配置：每次运行 init 时检查持久化状态，已配置则不再询问

实现：`pkg/system/init.go` 中的 `Initialize()` 和 `setupIptablesPersistence()`

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

### WireGuard 隧道创建流程 (line create --type wireguard)

**服务器模式**（自动生成所有密钥）：
```bash
twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_ab \
  --type wireguard \
  --mode server \
  --listen-port 51820
```

**内部流程**：
1. 为本地（服务器）生成密钥对：私钥A + 公钥A
2. 为对端（客户端）生成密钥对：私钥B + 公钥B
3. 配置本地 WireGuard 接口（使用私钥A，对端公钥B）
4. **输出完整的对端创建命令**（包含私钥B和公钥A）
5. 保存对端配置到 `/var/lib/trueword_node/peer_configs/<name>.txt`

**客户端模式**（使用服务器提供的密钥）：
```bash
# 复制服务器输出的完整命令，替换 <父接口>
twnode line create eth0 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ab \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820
```

**获取对端配置命令**：
```bash
twnode line show-peer <tunnel_name>
```

**重要注意事项**：
- 服务器模式 `remote_ip` 使用 `0.0.0.0`（占位符）
- 客户端首次连接后，服务器通过 `wg show` 获取实际对端IP
- 动态IP场景依赖 `policy sync-protection` 自动更新保护路由

实现：
- `pkg/wireguard/tunnel.go` - WireGuard 隧道创建
- `pkg/wireguard/keygen.go` - 密钥生成
- `cmd/main.go` - CLI 参数处理和对端配置输出

### 策略路由流程 (policy)

**策略组管理支持局部操作**：
- 应用：`twnode policy apply` 或 `twnode policy apply <group_name>`
- 撤销：`twnode policy revoke` 或 `twnode policy revoke <group_name>`
- 删除：`twnode policy delete <group_name>`（自动检测并撤销已应用的策略）

**创建策略组**：
```bash
# 自动分配优先级（默认）
twnode policy create <group_name> <exit_interface>

# 手动指定优先级（100-899）
twnode policy create <group_name> <exit_interface> --priority 150

# 可选的源地址限制
twnode policy create <group_name> <exit_interface> --from <source_ip/cidr>
```

**调整优先级**：
```bash
# 修改已存在策略组的优先级
twnode policy set-priority <group_name> <new_priority>
# 会自动检查优先级冲突，如已应用则自动重新应用
```

**应用流程**：
1. 创建策略组（指定出口接口、可选优先级和 from 源地址限制）
2. 添加 CIDR 到策略组
3. **Apply** 时检查出口接口状态
4. 自动添加保护路由（优先级10，保护隧道底层连接）
5. 应用策略组路由规则（先添加新规则，再清理重复规则，避免网络中断）
6. 可选：设置默认路由（0.0.0.0/0，优先级900）

**故障转移（Failover）流程**：
1. 指定策略组或默认路由及候选出口列表
2. 可选：提供 check_ip 重新检查，或使用 `line check` 的历史结果
3. 根据评分算法（基础评分 - 成本惩罚）选择最佳出口
   - 基础评分 = 60% 丢包率评分 + 40% 延迟评分
   - 成本惩罚 = Cost × 0.5
4. 自动切换策略组或默认路由到最佳出口

实现：`pkg/routing/policy.go`

### 保护路由同步流程 (policy sync-protection)

**核心功能**：
- 检测隧道对端IP变化并自动更新保护路由
- 添加缺失的保护路由（新隧道或未保护的隧道）
- 清理僵尸规则（无对应隧道的保护路由）

**执行方式**：
```bash
# 手动执行
twnode policy sync-protection

# Cron 定时任务（推荐）
*/5 * * * * /usr/local/bin/twnode policy sync-protection >/dev/null 2>&1
```

**自动调用时机**：
- `policy apply` 开始前
- `line start <name>` 完成后
- `line start-all` 完成后

**工作流程**：
1. 加载所有隧道配置
2. 对每个隧道：
   - GRE 隧道：从配置文件读取 `RemoteIP`
   - WireGuard 客户端：从配置文件读取 `RemoteIP`
   - WireGuard 服务器：通过 `wg show <interface> endpoints` 获取实际对端IP
3. 检查 `ProtectedIP` 字段，如果IP变化：
   - 删除旧保护路由：`ip rule del to <旧IP> pref 10`
   - 添加新保护路由：`ip rule add to <新IP> lookup main pref 10`
   - 更新配置文件中的 `ProtectedIP`
4. 扫描所有优先级10的规则，清理无对应隧道的规则

**动态IP场景示例**：
```
同步保护路由...
  ℹ 从运行状态检测到 WireGuard 隧道 hk-tw 的对端IP: 103.118.40.121
  ⚠ WireGuard 隧道 hk-tw 对端IP已变化: 1.2.3.4 → 103.118.40.121
  ✓ 保护 GRE 隧道 tun01 的远程IP 192.168.1.100
  清理 1 个僵尸规则...
  ✓ 已清理僵尸规则: 5.6.7.8
  已更新 1 个隧道的保护路由
✓ 保护路由同步完成
```

实现：
- `pkg/routing/policy.go` 中的 `SyncProtection()` 函数
- `pkg/wireguard/tunnel.go` 中的 `GetWireGuardPeerEndpoint()` 函数

### 故障转移守护进程 (policy failover daemon)（v1.4+）

**核心架构**：守护进程提供毫秒级自动故障转移，监控多个策略组或默认路由的健康状态。

**配置文件格式**（`/etc/trueword_node/failover_daemon.yaml`）：
```yaml
daemon:
  check_interval_ms: 500            # 检测间隔（100-60000ms）
  score_threshold: 5.0              # 评分差值阈值（触发切换的最小差值）
  switch_confirmation_count: 3      # 切换确认次数（防抖，默认1）
  log_file: ""                      # 日志文件路径（留空则不保存）

monitors:
  - name: "monitor-cn-routes"       # 监控任务名称
    type: "policy_group"            # 类型：policy_group 或 default_route
    target: "cn_routes"             # 目标策略组名称或 default
    check_targets:                  # 检测目标IP（最多3个，按顺序尝试）
      - "114.114.114.114"
      - "223.5.5.5"
    candidate_exits:                # 候选出口列表
      - "tun01"
      - "tun02"
      - "eth0"
    check_interval_ms: 0            # 可选：覆盖全局检测间隔
    score_threshold: 0              # 可选：覆盖全局评分阈值
    switch_confirmation_count: 0    # 可选：覆盖全局确认次数
```

**重要字段说明**：
- **check_interval_ms**：检测间隔，推荐 500ms（1秒2次检测）
- **score_threshold**：评分差值阈值，只有当新出口评分比当前出口高出此值时才触发切换
- **switch_confirmation_count**（v1.4.1+）：**切换确认次数**，需要连续N次检测都判定需要切换才真正执行
  - 默认值：1（立即切换，向上兼容）
  - 推荐值：3（500ms × 3 = 1.5秒确认时间）
  - 作用：防止网络抖动导致频繁切换
  - 支持全局配置和单任务覆盖
- **check_targets**：按顺序尝试，首个成功的IP用于检测（提供容错能力）

**关键机制**：

1. **切换确认机制**（防抖动）：
   - 检测到需要切换时，确认计数器 +1
   - 连续 N 次检测都判定需要切换，才真正执行
   - 中途评分差值不足或当前出口恢复，计数器重置为 0
   - 日志显示确认进度：`【确认中】切换确认进度: 2/3 (还需 1 次确认)`

2. **物理接口健康检查**：
   - 物理接口通过 **gateway 路由** 检测（`via <gateway>`）
   - 隧道接口通过 **设备路由** 检测（`dev <interface>`）
   - 临时测试路由使用优先级 5（最高，确保不受用户策略干扰）
   - 检测完成后自动清理测试路由

3. **评分决策**：
   - 使用与 `line check` 相同的评分算法
   - 只有当 `新出口评分 - 当前出口评分 >= score_threshold` 时才触发切换
   - 评分相同时优先保持当前出口（避免无意义切换）

4. **配置热重载**：
   - 守护进程运行时发送 SIGHUP 信号重载配置
   - systemd 管理：`sudo systemctl reload twnode-failover`
   - 不中断现有监控任务

**常用命令**：
```bash
# 初始化配置文件
twnode policy failover init-config

# 验证配置
twnode policy failover validate-config

# 查看全局配置
twnode policy failover show-config

# 修改全局配置
twnode policy failover set-config --interval 500 --score-threshold 5.0 --switch-confirmation-count 3

# 添加监控任务
twnode policy failover add-monitor <name> \
  --type policy_group \
  --target cn_routes \
  --check-targets 114.114.114.114,223.5.5.5 \
  --exits tun01,tun02,eth0 \
  --switch-confirmation-count 5

# 查看监控任务
twnode policy failover list-monitors
twnode policy failover show-monitor <name>

# 启动守护进程（调试模式）
sudo twnode policy failover start-daemon --debug

# systemd 服务管理
sudo systemctl start twnode-failover
sudo systemctl enable twnode-failover
sudo systemctl status twnode-failover
sudo systemctl reload twnode-failover  # 重载配置
```

**实现位置**：
- `pkg/failover/daemon.go` - 守护进程核心逻辑
- `pkg/failover/health_checker.go` - 健康检查（支持物理接口和隧道接口）
- `pkg/failover/config.go` - 配置管理和验证
- `pkg/failover/state_manager.go` - 运行时状态持久化
- `cmd/main.go` - CLI 命令实现

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

**Ping 测试优化**（v1.1+）：
- 发送 20 个 ping 包（提供 5% 丢包率精度）
- 包间隔 `-i 0.2` 秒（加速测试）
- 超时 `-W 1` 秒
- 总测试时间：约 4 秒

**临时测试路由**：
- 优先级 5（最高优先级，确保不受用户策略干扰）
- 规则：`to <目标IP> lookup 5 pref 5`
- 使用 defer 确保测试完成后立即清理
- 测试期间可能短暂影响访问目标IP的流量（约4秒）

**评分算法（v1.1+）**：
- 基础评分（0-100分）：
  - 丢包率评分：0-60 分（权重更高）
  - 延迟评分：0-40 分
- 成本惩罚：Cost × 0.5
- 最终评分 = 基础评分 - 成本惩罚
- 分数越高越好，用于 failover 选择最佳出口

### YAML 配置字段命名规范

**重要**：所有 YAML 配置文件使用 **下划线（_）** 作为单词分隔符，而不是连字符（-）。

**正确示例**：
```yaml
daemon:
  check_interval_ms: 500              # ✓ 正确
  score_threshold: 5.0                # ✓ 正确
  switch_confirmation_count: 3        # ✓ 正确

monitors:
  - name: "test"
    check_targets: ["8.8.8.8"]        # ✓ 正确
    candidate_exits: ["eth0"]         # ✓ 正确
    switch_confirmation_count: 5      # ✓ 正确
```

**错误示例**：
```yaml
daemon:
  check-interval-ms: 500              # ✗ 错误：使用了连字符
  switch-confirmation-count: 3        # ✗ 错误：使用了连字符
```

**Go 结构体标签与 YAML 的映射**：
```go
type MonitorConfig struct {
    SwitchConfirmationCount int `yaml:"switch_confirmation_count"`  // 注意是下划线
}
```

如果配置文件中使用连字符（如 `switch-confirmation-count`），YAML 解析器将无法正确映射到 Go 结构体，导致字段被忽略或使用默认值。

## UI 设计原则

- 命令执行时**不显示具体命令**，只显示结果
- **仅在出错时**显示命令和错误输出
- 使用框线装饰重要界面（`╔═══╗`）
- 使用中文分组标记（【配置信息】、【建立连接】）
- 使用图标增强可读性（✓ ✗ ⚠️）
- 保持输出简洁、美观、易读

**表格显示**：
- **始终使用 tablewriter 库**处理表格显示
- 自动处理中英文字符对齐问题
- 提供美观的 ASCII 边框
- 库：`github.com/olekukonko/tablewriter`

实现示例：
- 命令执行：`pkg/ipsec/tunnel.go` 中的 `execCommand()` 函数
- 表格显示：`pkg/routing/policy.go` 中的 `ListGroups()` 函数

## 常见开发任务

### 添加新的隧道功能

1. 在 `pkg/ipsec/tunnel.go` 或 `tunnel_manager.go` 中添加核心逻辑
2. 记录撤销命令到 rev 文件
3. 在 `cmd/main.go` 中添加 CLI 命令
4. 更新配置结构（如需要）：`pkg/network/tunnel_config.go`
5. **更新相关文档**：
   - 命令文档：`docs/commands/line/` 下对应命令的 .md 文件
   - 如果是新命令，在 `docs/commands/line/index.md` 添加索引
   - 考虑是否需要添加教程：`docs/tutorials/`

### 添加新的策略路由功能

1. 在 `pkg/routing/policy.go` 中添加逻辑
2. **注意策略规则管理**：必须使用"先添加后清理"的策略避免网络中断
3. 注意优先级范围限制（100-899 为用户策略组范围）
4. 优先级冲突检查：创建或修改优先级时检查是否与其他策略组冲突
5. 在 `cmd/main.go` 中添加子命令
6. 考虑是否需要支持局部操作（单个策略组 vs 所有策略）
7. 如果涉及表格显示，使用 tablewriter 库
8. **更新相关文档**：
   - 命令文档：`docs/commands/policy/` 下对应命令的 .md 文件
   - 如果是新命令，在 `docs/commands/policy/index.md` 添加索引
   - 考虑是否需要添加教程示例：`docs/tutorials/policy-routing.md`
   - 如果涉及路由表变更，更新 `docs/reference/routing-tables.md`

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
    github.com/olekukonko/tablewriter   // 表格显示（处理中英文对齐）
    gopkg.in/yaml.v3                    // 配置文件解析
)
```

## 常见问题和解决方案

### 1. 守护进程读取的配置值不正确

**症状**：配置文件中设置了 monitor 级别的参数（如 `switch_confirmation_count`），但守护进程运行时使用的是全局配置值。

**原因**：YAML 字段名使用了连字符（-）而不是下划线（_）。

**错误示例**：
```yaml
monitors:
  - name: "default"
    switch-confirmation-count: 5   # ✗ 错误：使用了连字符
```

**正确示例**：
```yaml
monitors:
  - name: "default"
    switch_confirmation_count: 5   # ✓ 正确：使用下划线
```

**验证方法**：
```bash
# 查看监控任务详情，确认字段是否正确读取
sudo twnode policy failover show-monitor <monitor_name>
```

### 2. 守护进程检测物理接口总是 DOWN

**症状**：守护进程检测物理接口时显示 DOWN，但手动 `line check` 或 `policy failover` 正常。

**原因**：v1.4.0 版本的 bug，已在 v1.4.1 修复。守护进程未正确使用物理接口的 gateway 路由。

**解决方案**：
- 升级到 v1.4.1 或更高版本
- v1.4.1+ 版本中，健康检查器会自动读取物理接口配置并使用正确的路由方式

### 3. 网络抖动导致频繁切换

**症状**：守护进程频繁切换出口，导致网络不稳定。

**原因**：未设置切换确认次数，或阈值设置过低。

**解决方案**（v1.4.1+）：
```yaml
daemon:
  switch_confirmation_count: 3  # 全局默认：需要连续3次确认
  score_threshold: 5.0          # 评分差值至少5分才触发

monitors:
  - name: "critical-service"
    switch_confirmation_count: 5  # 关键业务：需要连续5次确认
```

**推荐配置**：
- 检测间隔 500ms + 确认次数 3 = 1.5秒确认时间
- 关键业务可设置更高的确认次数（5-10）

## 测试注意事项

- 所有网络操作需要 **root 权限**
- 测试环境需要 Linux 内核支持 GRE 和 XFRM
- 测试前运行 `sudo twnode init` 初始化环境
- 清理测试环境：删除测试隧道，运行 `twnode policy revoke`
- 测试守护进程时使用 `--debug` 参数查看详细日志

## 开发规范

### 语言和文档
- **代码注释**：使用简体中文
- **CLI 输出**：使用简体中文
- **用户交互**：使用简体中文
- **变量命名**：使用英文

### Git 工作流
- **不要自动 commit 和 push**，除非用户明确要求
- 创建 commit 时使用有意义的中文提交信息
- 提交信息格式：简短标题 + 详细说明（新增功能、修复问题、技术改进）

### 编译和构建
- **始终使用静态编译**（`make static`）
- 确保二进制文件可在任何 Linux 系统上运行
- 编译后验证静态链接（`file` 和 `ldd`）

### UI 和显示
- **表格显示必须使用 tablewriter 库**
- 自动处理中英文字符对齐
- 保持输出简洁美观

### 文档更新
- 添加新功能时**必须同步更新文档**
- 文档使用简体中文，与CLI输出保持一致
- 保持文档间的交叉链接完整性
- 重要功能应有对应的教程和示例
- 参考现有文档的格式和结构（`docs/` 目录）