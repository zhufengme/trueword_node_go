# TrueWord Node 使用示例

本文档提供详细的使用示例和最佳实践。

## 快速开始

### 1. 系统初始化

```bash
# 以root身份运行初始化
sudo ./bin/twnode init
```

输出示例:
```
初始化 TrueWord Node...

检查必需命令...
  ✓ ip
  ✓ iptables
  ✓ ping
  ✓ sysctl

启用IP转发...
  ✓ net.ipv4.ip_forward = 1
  ✓ 配置已保存到 /etc/sysctl.d/99-trueword-node.conf

配置iptables MASQUERADE...
  ✓ 已添加 MASQUERADE 规则

创建配置目录...
  ✓ /etc/trueword_node
  ✓ /var/lib/trueword_node
  ✓ /var/lib/trueword_node/rev
  ✓ /etc/trueword_node/policies

创建配置文件...
  ✓ 配置文件已创建: /etc/trueword_node/config.yaml

============================================================
✓ 初始化完成!
============================================================
```

### 2. 创建第一条隧道(交互式)

```bash
# 不带参数自动进入交互模式
sudo ./bin/twnode line create
```

交互示例:
```
=== 交互式创建隧道 ===

远程IP地址: 203.0.113.10
远程虚拟IP: 10.0.1.2
本地IP地址: 198.51.100.5
本地虚拟IP: 10.0.1.1
隧道名称 (留空自动生成): tun-prod
认证密钥 (不显示): ********
加密密钥 (不显示): ********

=== 确认信息 ===
远程: 203.0.113.10/10.0.1.2
本地: 198.51.100.5/10.0.1.1
隧道名: tun-prod

确认创建? (yes/no): yes

开始创建...
=== 创建 IPsec 连接 ===
生成 SPI one: 0xa3b2c1d4
生成 SPI two: 0xd4c1b2a3
...
```

## 真实场景示例

### 场景1: 连接两个数据中心

**需求**:
- 北京数据中心: 公网IP 1.2.3.4, 内网 10.10.0.0/16
- 上海数据中心: 公网IP 5.6.7.8, 内网 10.20.0.0/16
- 需要两地内网互通

**北京节点 (1.2.3.4)**:
```bash
# 1. 创建隧道
sudo twnode line create
  远程IP地址: 5.6.7.8
  远程虚拟IP: 10.100.0.2
  本地IP地址: 1.2.3.4
  本地虚拟IP: 10.100.0.1
  隧道名称: tun-shanghai
  认证密钥: bj-sh-tunnel-2024
  加密密钥: secure-key-for-bj-sh

# 2. 创建策略组
sudo twnode policy create shanghai-net tun-shanghai

# 3. 添加上海内网网段
sudo twnode policy add shanghai-net 10.20.0.0/16

# 4. 应用策略
sudo twnode policy apply

# 5. 测试连通性
ping 10.100.0.2
ping 10.20.0.1
```

**上海节点 (5.6.7.8)**:
```bash
# 1. 创建隧道(密钥必须相同!)
sudo twnode line create
  远程IP地址: 1.2.3.4
  远程虚拟IP: 10.100.0.1
  本地IP地址: 5.6.7.8
  本地虚拟IP: 10.100.0.2
  隧道名称: tun-beijing
  认证密钥: bj-sh-tunnel-2024
  加密密钥: secure-key-for-bj-sh

# 2. 创建策略组
sudo twnode policy create beijing-net tun-beijing

# 3. 添加北京内网网段
sudo twnode policy add beijing-net 10.10.0.0/16

# 4. 应用策略
sudo twnode policy apply

# 5. 测试连通性
ping 10.100.0.1
ping 10.10.0.1
```

### 场景2: 多隧道负载均衡

**需求**:
- 本地有3条VPN隧道到不同地区
- 特定流量走特定隧道
- 其余流量走本地网络

```bash
# 1. 创建3条隧道
sudo twnode line create 1.1.1.1/10.0.1.2 本地IP/10.0.1.1 tun-us \
  --auth-key "us-key" --enc-key "us-enc"

sudo twnode line create 2.2.2.2/10.0.2.2 本地IP/10.0.2.1 tun-eu \
  --auth-key "eu-key" --enc-key "eu-enc"

sudo twnode line create 3.3.3.3/10.0.3.2 本地IP/10.0.3.1 tun-asia \
  --auth-key "asia-key" --enc-key "asia-enc"

# 2. 创建策略组(优先级自动分配)
sudo twnode policy create us-services tun-us
sudo twnode policy create eu-services tun-eu
sudo twnode policy create asia-services tun-asia

# 3. 准备CIDR列表文件
cat > /tmp/us-ips.txt << 'EOF'
# 美国服务IP段
192.0.2.0/24
198.51.100.0/24
EOF

cat > /tmp/eu-ips.txt << 'EOF'
# 欧洲服务IP段
203.0.113.0/24
EOF

cat > /tmp/asia-ips.txt << 'EOF'
# 亚洲服务IP段
192.168.100.0/24
192.168.101.0/24
EOF

# 4. 批量导入
sudo twnode policy import us-services /tmp/us-ips.txt
sudo twnode policy import eu-services /tmp/eu-ips.txt
sudo twnode policy import asia-services /tmp/asia-ips.txt

# 5. 设置默认路由(走本地网络)
sudo twnode policy default eth0

# 6. 应用所有策略
sudo twnode policy apply

# 7. 查看配置
sudo twnode policy list
sudo twnode status
```

### 场景3: 使用默认路由(0.0.0.0/0)

**需求**:
- 特定网段走VPN隧道
- 其他所有流量作为兜底，可以选择走本地网络或VPN
- 快速切换兜底出口

```bash
# 1. 创建策略组
sudo twnode policy create vpn-routes tun-vpn
sudo twnode policy add vpn-routes 192.168.0.0/16

# 2. 设置默认路由(0.0.0.0/0)作为兜底 - 走本地网络
sudo twnode policy default eth0
sudo twnode policy apply

输出:
开始应用策略路由...

检查出口状态...
  ✓ vpn-routes: 接口 tun-vpn 正常
  ✓ 默认出口: 接口 eth0 正常

应用策略组...
  ✓ vpn-routes: 1个CIDR -> tun-vpn (优先级 100)

应用默认路由(0.0.0.0/0)...
  ✓ 默认路由(0.0.0.0/0) -> eth0 (优先级 900)

✓ 策略路由应用完成

# 3. 测试路由
ip route get 192.168.1.1  # 应走 tun-vpn
ip route get 8.8.8.8      # 应走 eth0

# 4. 切换兜底出口 - 让所有其他流量走VPN
sudo twnode policy default tun-vpn

输出:
默认路由(0.0.0.0/0): eth0 -> tun-vpn

开始应用策略路由...
...
✓ 默认路由已应用

# 5. 再次测试
ip route get 8.8.8.8      # 现在应走 tun-vpn

# 6. 取消默认路由，使用系统路由表
sudo twnode policy unset-default
sudo twnode policy apply

输出:
⚠ 未设置默认路由，将使用系统路由表
```

**说明**:
- 默认路由(0.0.0.0/0)是策略路由的兜底规则
- 不设置时，未匹配的流量走系统默认路由表
- 设置后，可以控制所有未被策略组匹配的流量

### 场景4: 故障排查

#### 问题1: 隧道创建成功但无法ping通

```bash
# 1. 检查系统状态
sudo twnode status

# 2. 检查隧道是否UP
ip link show tun-test

# 3. 检查IPsec连接
sudo ip xfrm state list
sudo ip xfrm policy list

# 4. 检查路由
ip route show table all | grep tun-test

# 5. 尝试ping对端真实IP(不是VIP)
ping 对端公网IP

# 6. 检查防火墙
sudo iptables -L -n -v
```

#### 问题2: 策略路由不生效

```bash
# 1. 检查策略规则
ip rule list

# 2. 检查路由表
ip route show table 100  # 根据实际优先级

# 3. 检查接口状态
ip link show | grep tun

# 4. 重新应用策略
sudo twnode policy revoke
sudo twnode policy apply

# 5. 测试路由
ip route get 目标IP
```

## 命令组合技巧

### 批量操作

```bash
# 批量删除所有隧道
for tun in $(ip tunnel show | grep "tun-" | cut -d: -f1); do
    sudo twnode tunnel remove $tun
done

# 备份策略组配置
tar czf policies-backup.tar.gz /etc/trueword_node/policies/

# 恢复策略组配置
sudo tar xzf policies-backup.tar.gz -C /
```

### 监控和日志

```bash
# 实时查看隧道流量
watch -n 1 'ip -s link show | grep -A 1 tun-'

# 查看IPsec连接统计
watch -n 2 'sudo ip xfrm state list'

# 测试所有隧道连通性
for tun in $(ip link show | grep tun- | cut -d: -f2 | tr -d ' '); do
    remote_ip=$(ip tunnel show $tun | grep -oP 'remote \K[0-9.]+')
    echo "Testing $tun -> $remote_ip"
    ping -c 1 -W 1 $remote_ip >/dev/null 2>&1 && echo "  ✓ OK" || echo "  ✗ FAIL"
done
```

### 自动化脚本

```bash
#!/bin/bash
# 自动创建多条隧道

TUNNELS=(
    "1.1.1.1/10.0.1.2 本地IP/10.0.1.1 tun-01 auth1 enc1"
    "2.2.2.2/10.0.2.2 本地IP/10.0.2.1 tun-02 auth2 enc2"
    "3.3.3.3/10.0.3.2 本地IP/10.0.3.1 tun-03 auth3 enc3"
)

for tunnel in "${TUNNELS[@]}"; do
    read -r remote local name auth enc <<< "$tunnel"
    echo "Creating $name..."
    sudo twnode line create $remote $local $name \
        --auth-key "$auth" \
        --enc-key "$enc"
    sleep 2
done

echo "All tunnels created!"
```

## 最佳实践

### 1. 密钥管理

```bash
# 使用强密钥
# 不要使用: "123456", "password"
# 推荐使用: 随机生成的长字符串

# 生成随机密钥
AUTH_KEY=$(openssl rand -base64 32)
ENC_KEY=$(openssl rand -base64 32)

echo "Auth Key: $AUTH_KEY"
echo "Enc Key: $ENC_KEY"

# 通过安全通道发送给对端
```

### 2. 策略组织

```bash
# 按用途组织策略组
sudo twnode policy create internal-services tun-internal
sudo twnode policy create external-services tun-external
sudo twnode policy create backup-route tun-backup

# 按地区组织
sudo twnode policy create asia-pacific tun-apac
sudo twnode policy create europe-middle-east tun-emea
sudo twnode policy create americas tun-amer
```

### 3. 定期维护

```bash
# 每周检查隧道状态
sudo twnode status

# 每月备份配置
sudo tar czf /backup/twnode-$(date +%Y%m%d).tar.gz \
    /etc/trueword_node/ \
    /var/lib/trueword_node/

# 测试所有隧道连通性
sudo twnode policy revoke
sudo twnode policy apply
```

### 4. 安全建议

1. **定期更换密钥**: 每3-6个月更换一次隧道密钥
2. **最小权限原则**: 只开放必要的端口和IP
3. **日志审计**: 定期检查系统日志
4. **备份配置**: 保存好所有配置和密钥
5. **测试恢复**: 定期测试配置恢复流程

## 故障恢复

### 完全重置

```bash
# 1. 撤销所有策略
sudo twnode policy revoke

# 2. 删除所有隧道
for tun in $(ip tunnel show | grep "tun-" | cut -d: -f1); do
    sudo twnode tunnel remove $tun
done

# 3. 清除所有IPsec连接
sudo ip xfrm state flush
sudo ip xfrm policy flush

# 4. 删除配置(可选)
sudo rm -rf /etc/trueword_node/policies/*

# 5. 重新初始化
sudo twnode init
```

### 快速恢复

```bash
# 从备份恢复
sudo tar xzf /backup/twnode-backup.tar.gz -C /

# 重新应用策略
sudo twnode policy apply

# 验证
sudo twnode status
```

## 常见错误和解决方案

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| "接口不存在" | 隧道未创建或已删除 | 重新创建隧道 |
| "接口未启动" | 隧道DOWN状态 | `ip link set tun-xxx up` |
| "密钥不匹配" | 两端密钥不同 | 检查并统一密钥 |
| "路由冲突" | 策略重复 | 检查策略组,删除重复项 |
| "无法ping通" | 防火墙或路由问题 | 检查iptables和路由表 |

## 性能优化

```bash
# 调整MTU(如果有性能问题)
sudo ip link set tun-test mtu 1400

# 查看隧道性能
iperf3 -s  # 在对端运行
iperf3 -c 对端VIP  # 在本端测试
```

这些示例应该能帮助您快速上手TrueWord Node!
