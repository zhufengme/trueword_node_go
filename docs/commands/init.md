# init - 系统初始化

## 概述

`init` 命令用于初始化 TrueWord Node 系统环境，配置必要的系统参数，扫描物理网络接口，并创建配置目录结构。

**重要**: 这是使用 TrueWord Node 之前的必需步骤。

## 语法

```bash
sudo twnode init
```

## 执行流程

### 1. 权限检查

验证是否以 root 权限运行。

```
✓ 检查 root 权限...
```

### 2. 命令检查

检查系统是否安装了必需的命令工具：

- `ip` - 网络配置工具（iproute2）
- `iptables` - 防火墙工具
- `ping` - 连通性测试
- `sysctl` - 内核参数配置

```
✓ 检查必需命令...
```

如果缺少任何工具，会提示安装：

```
❌ 缺少必需命令: ip
请安装 iproute2 包
```

### 3. 启用 IP 转发

设置内核参数 `net.ipv4.ip_forward=1`，允许系统转发 IP 数据包。

```bash
# 执行的系统命令
sysctl -w net.ipv4.ip_forward=1
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
```

```
✓ 启用 IP 转发...
```

### 4. 配置 iptables MASQUERADE

设置 NAT 规则，允许流量通过隧道转发。

```bash
# 执行的系统命令
iptables -t nat -A POSTROUTING -j MASQUERADE
```

```
✓ 配置 iptables MASQUERADE...
```

### 5. 处理旧配置

如果检测到已存在的配置目录，会警告并要求确认：

```
⚠️ 检测到已存在的配置目录:
  /etc/trueword_node
  /var/lib/trueword_node

是否清除所有旧配置？这将删除所有隧道、策略组和检查结果。
输入 "yes" 确认删除，或按 Ctrl+C 取消:
```

**必须输入 "yes"（小写）才会继续**。任何其他输入都会取消操作。

### 6. 创建配置目录

清理旧配置后，创建以下目录结构：

```
/etc/trueword_node/
├── interfaces/
├── tunnels/
└── policies/

/var/lib/trueword_node/
├── rev/
├── peer_configs/
└── (check_results.json 将在首次 check 时创建)
```

```
✓ 创建配置目录...
```

### 7. 扫描物理接口

自动扫描所有物理网络接口，获取以下信息：

- 接口名称（如 eth0, ens33）
- IP 地址
- 网关地址

```
【扫描网络接口】
找到以下网络接口:
```

### 8. 交互式选择接口

显示所有找到的接口，用户可以选择要管理的接口：

```
序号  接口名    IP 地址           网关
----  ------    ---------------  ---------------
1     eth0      192.168.1.100    192.168.1.1
2     eth1      10.0.0.50        10.0.0.1
3     wlan0     192.168.2.200    192.168.2.1

请选择要管理的接口（输入序号，用逗号分隔，如 1,2）:
```

**选择方式**:
- 单个接口: 输入 `1`
- 多个接口: 输入 `1,2` 或 `1,3`
- 所有接口: 输入 `1,2,3`

### 9. 保存配置

将选择的接口保存到 `/etc/trueword_node/interfaces/physical.yaml`：

```yaml
interfaces:
  - name: eth0
    ip: 192.168.1.100
    gateway: 192.168.1.1
    managed: true
  - name: eth1
    ip: 10.0.0.50
    gateway: 10.0.0.1
    managed: true
```

```
✓ 已选择接口: eth0, eth1
✓ 配置已保存到 /etc/trueword_node/interfaces/physical.yaml
✓ 初始化完成！
```

## 完整示例

```bash
$ sudo twnode init

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

## 重新初始化

如果需要重新初始化（例如网络接口变化），可以再次运行 `init` 命令：

```bash
sudo twnode init
```

系统会警告并要求确认删除旧配置：

```
⚠️ 检测到已存在的配置目录:
  /etc/trueword_node
  /var/lib/trueword_node

是否清除所有旧配置？这将删除所有隧道、策略组和检查结果。
输入 "yes" 确认删除，或按 Ctrl+C 取消: yes

✓ 已删除旧配置
✓ 创建配置目录...
...
```

## 常见问题

### Q: 可以在没有 root 权限的情况下运行吗？

A: 不可以。`init` 命令需要修改系统配置（IP 转发、iptables），必须以 root 权限运行。

### Q: 如果我的接口没有被检测到怎么办？

A: 确保你的接口已经配置了 IP 地址。使用以下命令检查：

```bash
ip addr show
```

如果接口没有 IP 地址，先配置 IP 地址：

```bash
sudo ip addr add 192.168.1.100/24 dev eth0
sudo ip link set eth0 up
```

然后重新运行 `init`。

### Q: 初始化后可以修改管理的接口吗？

A: 可以。重新运行 `sudo twnode init` 并选择不同的接口。注意这会清除所有旧配置。

### Q: 初始化会影响现有的网络配置吗？

A: `init` 只会：
- 启用 IP 转发（通常对现有网络无影响）
- 添加 iptables MASQUERADE 规则（允许 NAT 转发）

不会修改现有的路由表、接口配置或防火墙规则（除了添加 MASQUERADE）。

### Q: 我不小心清除了配置，可以恢复吗？

A: 配置删除后无法自动恢复。建议在重要操作前备份配置目录：

```bash
# 备份配置
sudo tar -czf twnode_backup_$(date +%Y%m%d).tar.gz \
  /etc/trueword_node \
  /var/lib/trueword_node

# 恢复配置
sudo tar -xzf twnode_backup_20250115.tar.gz -C /
```

## 下一步

初始化完成后，可以开始创建隧道：

- [创建 WireGuard 隧道](../tutorials/wireguard-setup.md)
- [创建 GRE over IPsec 隧道](../tutorials/gre-ipsec-setup.md)
- [line create 命令详解](line/create.md)

---

**导航**: [← 命令参考](../index.md#命令参考) | [返回首页](../index.md) | [line create →](line/create.md)
