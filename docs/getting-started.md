# 快速入门

本指南将帮助你在 5 分钟内完成 TrueWord Node 的安装和基本配置。

## 前置要求

### 系统要求
- **操作系统**: Linux（任何发行版）
- **权限**: root 或 sudo 权限
- **内核**: 支持 GRE 和 XFRM（大多数现代 Linux 内核都支持）

### 必需工具
- `ip` - 网络配置工具（iproute2 包）
- `iptables` - 防火墙工具
- `ping` - 网络连通性测试
- `sysctl` - 内核参数配置

> 💡 **提示**: 这些工具在大多数 Linux 发行版中都已预装。

## 安装步骤

### 1. 获取源代码

```bash
git clone https://github.com/your-org/trueword_node_go.git
cd trueword_node_go
```

### 2. 编译

**重要**: 始终使用静态编译，确保二进制文件可在任何 Linux 系统上运行。

```bash
# 静态编译（推荐）
make static

# 验证静态编译
file bin/twnode
# 应输出: bin/twnode: ELF 64-bit LSB executable, ... statically linked ...

ldd bin/twnode
# 应输出: not a dynamic executable
```

### 3. 安装到系统

```bash
sudo make install
# 二进制文件将安装到 /usr/local/bin/twnode
```

### 4. 验证安装

```bash
twnode --version
# 应显示版本信息
```

## 系统初始化

### 运行 init 命令

首次使用必须运行初始化命令：

```bash
sudo twnode init
```

### 初始化流程

初始化过程会执行以下操作：

1. ✅ 检查 root 权限
2. ✅ 检查必需命令（ip, iptables, ping, sysctl）
3. ✅ 启用 IP 转发（`net.ipv4.ip_forward=1`）
4. ✅ 配置 iptables MASQUERADE
5. ✅ 扫描物理网络接口
6. ✅ 交互式选择要管理的接口
7. ✅ 创建配置目录结构

### 交互示例

```
╔═══════════════════════════════════════╗
║    TrueWord Node 系统初始化向导     ║
╚═══════════════════════════════════════╝

✓ 检查 root 权限...
✓ 检查必需命令...
✓ 启用 IP 转发...
✓ 配置 iptables MASQUERADE...

【扫描网络接口】
找到以下网络接口:

序号  接口名    IP 地址           网关
----  ------    ---------------  ---------------
1     eth0      192.168.1.100    192.168.1.1
2     eth1      10.0.0.50        10.0.0.1

请选择要管理的接口（输入序号，用逗号分隔，如 1,2）: 1

✓ 已选择接口: eth0
✓ 配置已保存到 /etc/trueword_node/interfaces/physical.yaml
✓ 初始化完成！
```

> ⚠️ **注意**: 如果已有旧配置，init 会警告并要求确认（必须输入 "yes"）才会清除。

## 第一个隧道

让我们创建一个简单的 WireGuard 隧道。

### 场景说明

假设你有两台服务器：

- **服务器 A**（本地）: `eth0 = 192.168.1.100`
- **服务器 B**（远程）: 公网 IP `203.0.113.50`

我们要在两台服务器之间建立 WireGuard 隧道：

- 服务器 A 虚拟 IP: `10.0.0.1`
- 服务器 B 虚拟 IP: `10.0.0.2`

### 服务器 A 操作（服务器模式）

```bash
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 tunnel_ab \
  --type wireguard \
  --mode server \
  --listen-port 51820
```

**参数说明**:
- `eth0` - 父接口（物理接口）
- `0.0.0.0` - 对端 IP（服务器模式使用占位符）
- `10.0.0.2` - 对端虚拟 IP
- `10.0.0.1` - 本地虚拟 IP
- `tunnel_ab` - 隧道名称

**输出示例**:

```
✓ 已创建 WireGuard 隧道: tunnel_ab

【对端配置命令】
在远程服务器上运行以下命令创建对应的隧道:

sudo twnode line create <父接口> 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820

💡 对端配置已保存到: /var/lib/trueword_node/peer_configs/tunnel_ab.txt
```

### 服务器 B 操作（客户端模式）

复制服务器 A 输出的命令，替换 `<父接口>` 为实际接口（如 `eth0`）：

```bash
sudo twnode line create eth0 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820
```

### 启动隧道

在**两台服务器**上分别运行：

```bash
# 服务器 A
sudo twnode line start tunnel_ab

# 服务器 B
sudo twnode line start tunnel_ba
```

### 验证连通性

在服务器 A 上 ping 服务器 B 的虚拟 IP：

```bash
ping 10.0.0.2
```

在服务器 B 上 ping 服务器 A 的虚拟 IP：

```bash
ping 10.0.0.1
```

如果能 ping 通，恭喜！隧道已成功建立 🎉

### 使用 check 命令检查

```bash
sudo twnode line check tunnel_ab 8.8.8.8
```

**输出示例**:

```
【连通性检查结果】
接口名称: tunnel_ab
测试地址: 8.8.8.8
丢包率: 0%
平均延迟: 15.3 ms
评分: 96.2 分
状态: ✓ 良好
```

## 配置策略路由

假设你想让某些流量通过隧道转发。

### 1. 创建策略组

```bash
sudo twnode policy create vpn_traffic tunnel_ab
```

### 2. 添加路由规则

```bash
# 添加单个 IP
sudo twnode policy add-cidr vpn_traffic 192.168.100.5/32

# 添加 IP 段
sudo twnode policy add-cidr vpn_traffic 192.168.100.0/24
```

### 3. 应用策略

```bash
sudo twnode policy apply vpn_traffic
```

### 4. 验证策略

```bash
# 查看策略组列表
sudo twnode policy list

# 测试路由
# 从 192.168.100.5 访问外部网络，流量应该通过 tunnel_ab
```

### 5. 撤销策略（可选）

```bash
sudo twnode policy revoke vpn_traffic
```

## 常用命令速查

### 隧道管理

```bash
# 列出所有隧道
sudo twnode line list

# 启动隧道
sudo twnode line start <隧道名>

# 启动所有隧道
sudo twnode line start-all

# 停止隧道
sudo twnode line stop <隧道名>

# 停止所有隧道
sudo twnode line stop-all

# 删除隧道
sudo twnode line delete <隧道名>

# 检查连通性
sudo twnode line check <隧道名> <测试IP>
```

### 策略路由

```bash
# 创建策略组
sudo twnode policy create <组名> <出口接口>

# 添加 CIDR
sudo twnode policy add-cidr <组名> <CIDR>

# 列出策略组
sudo twnode policy list

# 应用策略
sudo twnode policy apply [组名]

# 撤销策略
sudo twnode policy revoke [组名]

# 删除策略组
sudo twnode policy delete <组名>
```

## 配置文件位置

```
/etc/trueword_node/
├── config.yaml              # 全局配置
├── interfaces/
│   └── physical.yaml       # 物理接口配置
├── tunnels/
│   ├── tunnel_ab.yaml     # 隧道配置
│   └── ...
└── policies/
    ├── vpn_traffic.json   # 策略组配置
    └── ...

/var/lib/trueword_node/
├── rev/                    # 撤销命令
├── peer_configs/          # WireGuard 对端配置
└── check_results.json     # 连通性检查结果
```

## 下一步

现在你已经掌握了基础操作，可以深入学习：

- [架构设计](architecture.md) - 了解系统核心设计
- [WireGuard 完整配置](tutorials/wireguard-setup.md) - WireGuard 高级用法
- [GRE over IPsec 配置](tutorials/gre-ipsec-setup.md) - 传统双层隧道
- [策略路由实战](tutorials/policy-routing.md) - 复杂路由场景
- [故障转移配置](tutorials/failover-setup.md) - 高可用方案

## 常见问题

### Q: init 提示找不到网络接口？

A: 确保你的网络接口已配置 IP 地址。使用 `ip addr` 查看。

### Q: 隧道创建后无法 ping 通？

A: 检查以下几点：
1. 两端隧道都已启动（`twnode line list`）
2. 防火墙允许相应端口（WireGuard 默认 51820，IPsec 500/4500）
3. 对端 IP 是否正确（`twnode line list` 查看）

### Q: WireGuard 握手失败？

A: WireGuard 采用"静默协议"，首次握手需要 5-10 秒。客户端会自动发送 ping 包触发握手。等待片刻后重试。

### Q: 如何查看 WireGuard 对端配置？

A: 使用 `twnode line show-peer <隧道名>` 查看已保存的对端配置命令。

---

**导航**: [← 返回首页](index.md) | [架构设计 →](architecture.md)
