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
# 临时启用（当前会话）
sysctl -w net.ipv4.ip_forward=1
```

**情况 1：已持久化**（检测到配置文件存在）
```
✓ IP转发已启用（当前会话）
✓ 已持久化（/etc/sysctl.d/99-trueword-node.conf）
```

**情况 2：未持久化**（配置文件不存在，询问用户）
```
✓ IP转发已启用（当前会话）

ℹ️  IP转发配置是临时的，重启后会失效
是否持久化到系统配置? (Y/n):
```

**持久化方式**（用户选择 Y）：
- 创建文件：`/etc/sysctl.d/99-trueword-node.conf`
- 内容：`net.ipv4.ip_forward = 1`
- 系统重启后自动加载
- **下次运行 init 时自动检测，不再重复询问**

### 4. 配置 iptables MASQUERADE

设置 NAT 规则，允许流量通过隧道转发。

```bash
# 临时添加规则（当前会话）
iptables -t nat -A POSTROUTING -j MASQUERADE
```

**情况 1：已持久化**（检测到 systemd service 已启用）
```
✓ iptables MASQUERADE已配置（当前会话）
✓ 已持久化（systemd service已启用）
```

**情况 2：Service 存在但未启用**（配置异常）
```
✓ iptables MASQUERADE已配置（当前会话）
⚠️  systemd service存在但未启用，建议重新配置
```

**情况 3：未持久化**（service 文件不存在，询问用户）
```
✓ iptables MASQUERADE已配置（当前会话）

ℹ️  iptables规则是临时的，重启后会失效
是否通过systemd持久化? (Y/n):
```

**持久化方式**（用户选择 Y）：
- 创建脚本：`/usr/local/bin/twnode-iptables.sh`
- 创建 systemd service：`/etc/systemd/system/twnode-iptables.service`
- 启用开机自启：`systemctl enable twnode-iptables.service`
- 系统启动时自动应用 iptables 规则
- **下次运行 init 时自动检测，不再重复询问**

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

### Q: 持久化配置后，如何验证是否成功？

A: 可以通过以下方式验证：

**验证 IP 转发持久化**：
```bash
# 检查配置文件是否存在
cat /etc/sysctl.d/99-trueword-node.conf

# 应该输出：
# net.ipv4.ip_forward = 1
```

**验证 iptables 持久化**：
```bash
# 检查 systemd service 是否启用
systemctl status twnode-iptables.service

# 应该看到：
# Loaded: loaded (/etc/systemd/system/twnode-iptables.service; enabled)
# Active: active (exited)

# 检查脚本是否存在
ls -lh /usr/local/bin/twnode-iptables.sh
```

**重启测试**（可选）：
```bash
# 重启系统
sudo reboot

# 重启后检查 IP 转发
sysctl net.ipv4.ip_forward
# 应该输出: net.ipv4.ip_forward = 1

# 检查 iptables 规则
sudo iptables -t nat -L POSTROUTING -n -v
# 应该看到 MASQUERADE 规则
```

### Q: 如何手动移除持久化配置？

A: 如果需要移除持久化配置：

**移除 IP 转发持久化**：
```bash
sudo rm /etc/sysctl.d/99-trueword-node.conf
```

**移除 iptables 持久化**：
```bash
# 停止并禁用 service
sudo systemctl stop twnode-iptables.service
sudo systemctl disable twnode-iptables.service

# 删除 service 文件和脚本
sudo rm /etc/systemd/system/twnode-iptables.service
sudo rm /usr/local/bin/twnode-iptables.sh

# 重载 systemd
sudo systemctl daemon-reload
```

### Q: init 时选择了跳过持久化，以后可以手动持久化吗？

A: 可以。手动执行持久化步骤：

**手动持久化 IP 转发**：
```bash
echo "net.ipv4.ip_forward = 1" | sudo tee /etc/sysctl.d/99-trueword-node.conf
```

**手动持久化 iptables**（创建脚本和 service）：
```bash
# 1. 创建脚本
sudo tee /usr/local/bin/twnode-iptables.sh > /dev/null << 'EOF'
#!/bin/bash
iptables -t nat -D POSTROUTING -j MASQUERADE 2>/dev/null || true
iptables -t nat -A POSTROUTING -j MASQUERADE
exit 0
EOF

sudo chmod +x /usr/local/bin/twnode-iptables.sh

# 2. 创建 service
sudo tee /etc/systemd/system/twnode-iptables.service > /dev/null << 'EOF'
[Unit]
Description=TrueWord Node iptables Rules
After=network-pre.target
Before=network.target
DefaultDependencies=no

[Service]
Type=oneshot
ExecStart=/usr/local/bin/twnode-iptables.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

# 3. 启用并启动
sudo systemctl daemon-reload
sudo systemctl enable twnode-iptables.service
sudo systemctl start twnode-iptables.service
```

## 持久化配置说明

### 智能检测机制

**重复运行 init 时不会重复询问**：

系统会自动检测持久化状态：
- **IP 转发**：检查 `/etc/sysctl.d/99-trueword-node.conf` 是否存在
- **iptables 规则**：检查 systemd service 是否已启用

如果已经持久化，会直接显示"✓ 已持久化"，不再询问用户。

**好处**：
- ✅ 避免重复配置
- ✅ 避免重复询问
- ✅ 用户体验更友好
- ✅ 可安全多次运行 init

### 为什么需要持久化？

系统重启后，临时配置会丢失：
- IP 转发会恢复默认值（通常为 0，即禁用）
- iptables 规则会被清空

持久化后，系统启动时自动恢复配置，无需手动干预。

### 持久化方案对比

| 方案 | IP 转发 | iptables 规则 |
|------|---------|---------------|
| **临时** | `sysctl -w` | `iptables -A` |
| **持久化** | `/etc/sysctl.d/` | systemd service |
| **生效时机** | 立即 | 系统启动时 |
| **重启后** | 丢失 ❌ | 保留 ✓ |

### 推荐配置

- **生产环境**：建议持久化，避免重启后需要重新配置
- **测试环境**：可以跳过持久化，重启即可恢复干净状态
- **开发环境**：根据需要选择

## 下一步

初始化完成后，可以开始创建隧道：

- [创建 WireGuard 隧道](../tutorials/wireguard-setup.md)
- [创建 GRE over IPsec 隧道](../tutorials/gre-ipsec-setup.md)
- [line create 命令详解](line/create.md)

---

**导航**: [← 命令参考](../index.md#命令参考) | [返回首页](../index.md) | [line create →](line/create.md)
