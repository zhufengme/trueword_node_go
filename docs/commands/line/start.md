# line start - 启动隧道

## 概述

`line start` 命令用于启动已创建的隧道。支持启动单个隧道或所有隧道。

## 语法

```bash
# 启动单个隧道
sudo twnode line start <隧道名>

# 启动所有隧道
sudo twnode line start-all
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<隧道名>` | 要启动的隧道名称（start 命令） | 是 |

## 工作流程

### 单个隧道启动

```
1. 加载隧道配置
   └─ 从 /etc/trueword_node/tunnels/<name>.yaml

2. 检查隧道是否已经运行
   └─ 如果已运行，跳过启动

3. 根据隧道类型启动
   ├─ WireGuard:
   │  ├─ 创建接口: ip link add <name> type wireguard
   │  ├─ 配置密钥: wg set <name> private-key ...
   │  ├─ 配置对端: wg set <name> peer ...
   │  ├─ 配置 IP: ip addr add <local_vip>/32 dev <name>
   │  ├─ 启动接口: ip link set <name> up
   │  └─ 触发握手（客户端模式）
   └─ GRE over IPsec:
      ├─ 创建 IPsec 连接（如果启用加密）
      ├─ 创建 GRE 隧道: ip tunnel add ...
      ├─ 配置 IP: ip addr add <local_vip>/32 dev <name>
      └─ 启动接口: ip link set <name> up

4. 添加虚拟 IP 路由规则
   └─ ip rule add to <remote_vip>/32 lookup 80 pref 80

5. 添加保护路由（如果需要）
   └─ ip rule add to <remote_ip> lookup main pref 10

6. 同步保护路由
   └─ 执行 policy sync-protection
```

### 批量启动 (start-all)

```
1. 加载所有隧道配置
   └─ 扫描 /etc/trueword_node/tunnels/*.yaml

2. 按依赖关系排序
   ├─ 物理接口的隧道优先
   ├─ 然后是基于一层隧道的隧道
   └─ 最后是更深层的隧道

3. 依次启动每个隧道
   └─ 使用 start 命令逐个启动

4. 同步保护路由
   └─ 执行 policy sync-protection
```

## 示例

### 示例1: 启动单个隧道

```bash
$ sudo twnode line start tunnel_hk

启动隧道: tunnel_hk

✓ 创建 WireGuard 接口
✓ 配置密钥和对端
✓ 配置虚拟 IP: 10.0.0.1
✓ 启动接口
✓ 主动触发 WireGuard 握手...
✓ WireGuard 握手成功
✓ 添加虚拟 IP 路由规则
✓ 添加保护路由
✓ 同步保护路由

✓ 隧道 tunnel_hk 已启动
```

### 示例2: 启动所有隧道

```bash
$ sudo twnode line start-all

启动所有隧道...

【第一层隧道】
✓ tunnel_hk (基于 eth0)
✓ tunnel_us (基于 eth0)

【第二层隧道】
✓ tunnel_layer2 (基于 tunnel_hk)

【第三层隧道】
✓ tunnel_layer3 (基于 tunnel_layer2)

共启动 4 个隧道
✓ 同步保护路由

✓ 所有隧道已启动
```

### 示例3: 启动已经运行的隧道

```bash
$ sudo twnode line start tunnel_hk

ℹ 隧道 tunnel_hk 已经在运行
```

### 示例4: 启动失败

```bash
$ sudo twnode line start tunnel_hk

启动隧道: tunnel_hk

✓ 创建 WireGuard 接口
✓ 配置密钥和对端
❌ 错误: 配置虚拟 IP 失败
  原因: RTNETLINK answers: File exists

提示: 虚拟 IP 10.0.0.1 可能已被其他接口使用
检查: ip addr show | grep 10.0.0.1
```

## WireGuard 握手机制

### 客户端模式

客户端模式会主动触发握手：

```
1. 发送 ping 包到对端 VIP
   └─ ping -c 3 -W 1 -I <interface> <peer_vip>

2. 等待握手完成（最多 30 秒）
   ├─ 每秒检查一次握手状态
   └─ wg show <interface> latest-handshakes

3. 握手成功标志
   └─ latest-handshake 时间戳 > 0
```

### 服务器模式

服务器模式被动等待客户端连接：

```
1. 监听指定端口
   └─ listen-port: 51820

2. 等待客户端首次连接
   └─ 客户端发送数据包时触发握手

3. 连接建立后
   └─ 通过 wg show endpoints 查看客户端 IP
```

## 依赖关系处理

### 多层隧道启动顺序

```
物理接口 (eth0)
  ↓
隧道1 (tunnel_hk) - 必须先启动
  ↓
隧道2 (tunnel_layer2) - 然后启动
  ↓
隧道3 (tunnel_layer3) - 最后启动
```

**错误示例**（未启动父接口）:

```bash
$ sudo twnode line start tunnel_layer2

❌ 错误: 父接口 tunnel_hk 未运行
请先启动父接口: sudo twnode line start tunnel_hk
```

**正确做法**:

```bash
# 方法1: 按顺序启动
sudo twnode line start tunnel_hk
sudo twnode line start tunnel_layer2
sudo twnode line start tunnel_layer3

# 方法2: 使用 start-all（自动处理依赖）
sudo twnode line start-all
```

## 验证启动状态

### 检查接口状态

```bash
# 查看接口是否存在
ip link show tunnel_hk

# 应该显示:
# X: tunnel_hk: <POINTOPOINT,NOARP,UP,LOWER_UP> ...
#    ↑ UP 表示已启动
```

### 检查 WireGuard 状态

```bash
sudo wg show tunnel_hk

# 应该显示:
# interface: tunnel_hk
#   public key: xY9zA0...
#   private key: (hidden)
#   listening port: 51820
#
# peer: aB3cD4...
#   endpoint: 103.118.40.121:51820  ← 有 endpoint 表示已连接
#   allowed ips: 10.0.0.2/32
#   latest handshake: 10 seconds ago  ← 最近握手时间
#   transfer: 1.2 KiB received, 892 B sent
```

### 检查路由规则

```bash
# 查看虚拟 IP 路由规则
ip rule show | grep tunnel_hk

# 应该显示:
# 80: from all to 10.0.0.2/32 lookup 80

# 查看保护路由
ip rule show pref 10 | grep <remote_ip>

# 应该显示:
# 10: from all to 103.118.40.121 lookup main
```

### Ping 测试

```bash
# Ping 对端虚拟 IP
ping -c 3 10.0.0.2

# 应该能收到响应:
# 64 bytes from 10.0.0.2: icmp_seq=1 ttl=64 time=25.3 ms
```

## 自动启动

### 方法1: systemd 服务

创建 systemd 服务文件：

```bash
sudo nano /etc/systemd/system/twnode-tunnels.service
```

内容：

```ini
[Unit]
Description=TrueWord Node Tunnels
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/twnode line start-all
RemainAfterExit=yes
ExecStop=/usr/local/bin/twnode line stop-all

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable twnode-tunnels
sudo systemctl start twnode-tunnels
```

### 方法2: rc.local

编辑 `/etc/rc.local`：

```bash
#!/bin/bash
/usr/local/bin/twnode line start-all
exit 0
```

赋予执行权限：

```bash
sudo chmod +x /etc/rc.local
```

### 方法3: cron @reboot

```bash
crontab -e
```

添加：

```bash
@reboot /usr/local/bin/twnode line start-all
```

## 常见问题

### Q: 启动后无法 ping 通对端？

A: 检查以下几点：

1. **WireGuard 握手是否成功**:
   ```bash
   sudo wg show tunnel_hk latest-handshakes
   # 如果显示 0，说明握手失败
   ```

2. **对端是否也启动了**:
   ```bash
   # 在对端服务器上检查
   sudo twnode line list
   ```

3. **防火墙是否允许**:
   ```bash
   sudo iptables -L -v -n | grep 51820
   ```

### Q: start-all 启动顺序错误怎么办？

A: 手动指定启动顺序：

```bash
# 按依赖关系手动启动
sudo twnode line start tunnel_hk
sudo twnode line start tunnel_layer2
```

或修改配置文件中的父接口关系。

### Q: 启动失败，如何清理？

A: 使用 stop 命令或手动清理：

```bash
# 停止隧道
sudo twnode line stop tunnel_hk

# 或手动清理
sudo ip link del tunnel_hk
```

### Q: 启动时提示 "地址已存在"？

A: 说明虚拟 IP 被其他接口占用：

```bash
# 查找占用的接口
ip addr show | grep 10.0.0.1

# 删除冲突的接口或修改 VIP
```

## 下一步

- [停止隧道](stop.md) - 停止运行的隧道
- [检查连通性](check.md) - 测试隧道状态
- [列出隧道](list.md) - 查看所有隧道

---

**导航**: [← delete](delete.md) | [返回首页](../../index.md) | [stop →](stop.md)
