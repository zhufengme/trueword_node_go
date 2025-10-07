# 故障排查

本文档提供常见问题的诊断和解决方案。

## 隧道连接问题

### 无法 Ping 通对端虚拟 IP

**症状**: 隧道已启动，但无法 ping 通对端虚拟 IP。

**诊断步骤**:

1. **检查隧道状态**:
   ```bash
   sudo twnode line list
   ```
   确认隧道状态为 "Active"。

2. **检查接口是否存在**:
   ```bash
   ip link show <隧道名>
   ```
   应该显示接口存在且 UP。

3. **检查 IP 配置**:
   ```bash
   ip addr show <隧道名>
   ```
   确认虚拟 IP 已配置。

4. **检查路由规则**:
   ```bash
   ip route get <对端VIP>
   ```
   确认路由指向隧道接口。

**解决方案**:

- **对端未启动**: 在对端服务器启动隧道
- **防火墙阻止**: 检查并开放相应端口
- **WireGuard 握手失败**: 查看 WireGuard 状态

### WireGuard 握手失败

**症状**: WireGuard 隧道显示 Active，但 `latest handshake` 为 0 或很久之前。

**诊断**:

```bash
sudo wg show <隧道名>

# 输出示例:
interface: tunnel_hk
  public key: xY9zA0...
  private key: (hidden)
  listening port: 51820

peer: aB3cD4...
  endpoint: (none)  ← 问题：没有 endpoint
  allowed ips: 10.0.0.2/32
  latest handshake: 1 minute ago  ← 问题：时间过久或为0
  transfer: 0 B received, 0 B sent  ← 问题：无数据传输
```

**可能原因**:

1. **防火墙阻止**:
   ```bash
   # 检查 iptables
   sudo iptables -L -v -n | grep 51820

   # 开放端口
   sudo iptables -A INPUT -p udp --dport 51820 -j ACCEPT
   ```

2. **密钥不匹配**:
   ```bash
   # 检查配置文件中的密钥
   cat /etc/trueword_node/tunnels/<隧道名>.yaml

   # 重新创建隧道
   sudo twnode line delete <隧道名>
   sudo twnode line create ...
   ```

3. **对端 IP 错误**（客户端模式）:
   ```bash
   # 检查对端 IP
   cat /etc/trueword_node/tunnels/<隧道名>.yaml | grep remote_ip

   # 测试连通性
   ping <对端IP>
   ```

4. **NAT 穿透问题**:
   ```bash
   # 客户端主动发送包触发握手
   ping -c 5 -I <隧道接口> <对端VIP>
   ```

**解决方案**:

```bash
# 1. 重启隧道
sudo twnode line stop <隧道名>
sudo twnode line start <隧道名>

# 2. 检查对端是否在线
ping <对端IP>

# 3. 检查防火墙
sudo iptables -L -v -n

# 4. 查看系统日志
sudo journalctl -u wg-quick@<隧道名> -n 50
```

### GRE 隧道无法通信

**症状**: GRE 隧道显示 UP，但无法 ping 通。

**诊断**:

```bash
# 检查 GRE 接口
ip -d link show <隧道名>

# 检查 IPsec 连接
sudo ip xfrm state
sudo ip xfrm policy

# 检查 GRE Key
ip tunnel show <隧道名>
```

**可能原因**:

1. **IPsec 未建立**（如果启用加密）:
   ```bash
   # 检查 XFRM state
   sudo ip xfrm state | grep <对端IP>

   # 如果没有输出，IPsec 未建立
   ```

2. **GRE Key 不匹配**:
   ```bash
   # 查看本地 GRE Key
   ip tunnel show <隧道名> | grep key

   # 确保两端 GRE Key 相同
   ```

3. **认证/加密密钥不匹配**:
   ```bash
   # 确保两端使用相同的密钥
   cat /etc/trueword_node/tunnels/<隧道名>.yaml
   ```

**解决方案**:

```bash
# 1. 重新创建隧道（确保密钥一致）
sudo twnode line delete <隧道名>
sudo twnode line create ...

# 2. 检查协议是否被阻止
sudo iptables -L -v -n | grep gre
sudo iptables -L -v -n | grep esp

# 3. 允许 GRE 和 ESP
sudo iptables -A INPUT -p gre -j ACCEPT
sudo iptables -A INPUT -p esp -j ACCEPT
```

## 策略路由问题

### 流量未走策略路由

**症状**: 策略已应用，但流量未通过指定出口。

**诊断**:

```bash
# 1. 检查策略组状态
sudo twnode policy list

# 2. 检查路由规则
ip rule show

# 3. 测试路由决策
ip route get <目标IP>

# 4. 检查表 50
ip route show table 50
```

**可能原因**:

1. **策略未应用**:
   ```bash
   sudo twnode policy apply <策略组名>
   ```

2. **优先级问题**:
   ```bash
   # 检查是否被高优先级规则覆盖
   ip rule show | grep <目标IP>
   ```

3. **出口接口未启动**:
   ```bash
   sudo twnode line list
   sudo twnode line start <出口接口>
   ```

4. **CIDR 不匹配**:
   ```bash
   # 检查 CIDR 是否包含目标 IP
   cat /etc/trueword_node/policies/<策略组名>.json
   ```

**解决方案**:

```bash
# 1. 撤销并重新应用
sudo twnode policy revoke <策略组名>
sudo twnode policy apply <策略组名>

# 2. 检查并启动出口接口
sudo twnode line start <出口接口>

# 3. 验证 CIDR
sudo twnode policy list -v
```

### 路由环路

**症状**: 网络不通，或出现异常延迟。

**诊断**:

```bash
# 检查是否有环路
traceroute <目标IP>

# 检查保护路由
ip rule show pref 10
```

**原因**: 隧道对端 IP 没有保护路由，导致握手包也走策略路由。

**解决方案**:

```bash
# 同步保护路由
sudo twnode policy sync-protection

# 或手动添加
sudo ip rule add to <对端IP> lookup main pref 10
```

### 策略冲突

**症状**: 多个策略组，流量走向不符合预期。

**诊断**:

```bash
# 查看所有策略组的优先级
sudo twnode policy list

# 查看路由规则
ip rule show | grep "lookup 50"
```

**解决方案**:

```bash
# 调整策略组优先级
sudo twnode policy set-priority <策略组名> <新优先级>

# 重新应用
sudo twnode policy apply
```

## 系统问题

### 权限错误

**症状**: 命令提示 "Permission denied" 或 "Operation not permitted"。

**原因**: 未使用 root 权限。

**解决方案**:

```bash
# 使用 sudo
sudo twnode <命令>

# 或切换到 root
sudo su
twnode <命令>
```

### 命令未找到

**症状**: `bash: twnode: command not found`

**解决方案**:

```bash
# 检查安装
which twnode

# 如果未安装
cd /path/to/trueword_node_go
make static
sudo make install

# 检查 PATH
echo $PATH
```

### 配置文件丢失

**症状**: 提示 "配置文件不存在"。

**原因**: 配置文件被删除或损坏。

**解决方案**:

```bash
# 1. 重新初始化
sudo twnode init

# 2. 或从备份恢复
sudo tar -xzf twnode_backup_20250115.tar.gz -C /
```

### 内核模块缺失

**症状**: 提示 "modprobe: FATAL: Module ... not found"。

**诊断**:

```bash
# 检查 GRE 模块
lsmod | grep gre

# 检查 WireGuard 模块
lsmod | grep wireguard
```

**解决方案**:

```bash
# 加载 GRE 模块
sudo modprobe ip_gre

# 安装 WireGuard
sudo apt install wireguard  # Ubuntu/Debian
sudo yum install wireguard-tools  # CentOS/RHEL
```

## 性能问题

### 高延迟

**症状**: ping 延迟很高。

**诊断**:

```bash
# 测试隧道延迟
ping -c 10 -I <隧道接口> <对端VIP>

# 测试物理接口延迟
ping -c 10 <对端IP>
```

**可能原因**:

1. **网络拥塞**: 物理链路延迟高
2. **CPU 使用率高**: 加密/解密占用 CPU
3. **MTU 问题**: MTU 设置不当

**解决方案**:

```bash
# 1. 调整 MTU
sudo ip link set <隧道接口> mtu 1420

# 2. 检查 CPU 使用率
top

# 3. 检查网络带宽
iperf3 -c <对端IP>
```

### 丢包

**症状**: ping 丢包率高。

**诊断**:

```bash
# 长时间 ping 测试
ping -c 100 -I <隧道接口> <对端VIP>

# 检查接口统计
ip -s link show <隧道接口>
```

**可能原因**:

1. **物理链路丢包**
2. **防火墙限速**
3. **缓冲区溢出**

**解决方案**:

```bash
# 1. 增加缓冲区
sudo sysctl -w net.core.rmem_max=26214400
sudo sysctl -w net.core.wmem_max=26214400

# 2. 检查防火墙规则
sudo iptables -L -v -n

# 3. 检查物理链路
ping <对端IP>
```

## 日志和调试

### 查看系统日志

```bash
# 查看所有日志
sudo journalctl -xe

# 查看网络相关日志
sudo journalctl -u NetworkManager

# 查看 WireGuard 日志
sudo journalctl | grep wireguard

# 查看内核日志
dmesg | grep -i gre
dmesg | grep -i wireguard
```

### 启用调试模式

```bash
# WireGuard 调试
sudo wg set <隧道接口> peer <对端公钥> persistent-keepalive 25

# IPsec 调试
sudo ip xfrm monitor

# 路由调试
ip route get <目标IP> verbose
```

### 抓包分析

```bash
# 抓取隧道接口的包
sudo tcpdump -i <隧道接口> -w /tmp/tunnel.pcap

# 抓取物理接口的包
sudo tcpdump -i eth0 port 51820 -w /tmp/wireguard.pcap

# 分析抓包文件
wireshark /tmp/tunnel.pcap
```

## 获取帮助

如果以上方法无法解决问题：

1. **检查日志**: `sudo journalctl -xe`
2. **查看配置**: `cat /etc/trueword_node/...`
3. **提交 Issue**: 在 GitHub 上提交详细的问题描述

**Issue 模板**:

```markdown
## 问题描述
简要描述问题

## 环境信息
- OS: Ubuntu 22.04
- 内核版本: uname -r
- twnode 版本: twnode --version

## 重现步骤
1. sudo twnode line create ...
2. sudo twnode line start ...
3. ping ...

## 期望结果
应该能 ping 通

## 实际结果
ping 失败

## 日志输出
粘贴相关日志
```

---

**导航**: [← 保护路由](protection-routes.md) | [返回首页](../index.md)
