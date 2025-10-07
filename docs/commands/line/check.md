# line check - 连通性检查

## 概述

`line check` 命令用于检查隧道的连通性、丢包率和延迟，并生成综合评分。结果保存到本地，供故障转移使用。

## 语法

```bash
sudo twnode line check <隧道名> <测试IP>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<隧道名>` | 要检查的隧道名称 | 是 |
| `<测试IP>` | 测试目标 IP 地址 | 是 |

## 测试方法

### Ping 测试

使用 `ping` 命令通过指定隧道发送测试包：

```bash
ping -c 20 -i 0.2 -W 1 -I <隧道接口> <测试IP>
```

**参数说明**:
- `-c 20` - 发送 20 个包（提供 5% 丢包率精度）
- `-i 0.2` - 包间隔 0.2 秒（加速测试）
- `-W 1` - 超时 1 秒
- `-I <隧道接口>` - 指定出口接口

**测试时间**: 约 4 秒

### 临时测试路由

为确保测试流量通过指定隧道，会临时添加优先级 5 的测试路由：

```bash
# 添加临时路由（优先级最高）
ip rule add to <测试IP> lookup 5 pref 5
ip route add <测试IP> dev <隧道接口> table 5

# 执行测试
ping ...

# 清理临时路由（defer 自动清理）
ip route del <测试IP> table 5
ip rule del to <测试IP> pref 5
```

**优先级 5 的作用**:
- 最高优先级，确保测试流量不受用户策略干扰
- 临时存在（约 4 秒），不会影响系统路由
- 测试完成后自动清理

## 评分算法

### 基础评分

```
丢包率评分 = (1 - 丢包率) × 60        # 0-60 分，权重 60%
延迟评分 = max(0, (1 - 延迟/200)) × 40  # 0-40 分，权重 40%
基础评分 = 丢包率评分 + 延迟评分        # 0-100 分
```

### 成本惩罚

```
成本惩罚 = Cost × 0.5
最终评分 = 基础评分 - 成本惩罚
```

**Cost 字段**: 在策略组配置中设置，用于手动调整优先级。

### 状态判断

| 评分范围 | 状态 | 说明 |
|---------|------|------|
| >= 80 | 良好 (good) | 连接稳定，质量优秀 |
| 60-79 | 降级 (degraded) | 可用但质量下降 |
| < 60 | 差 (bad) | 连接质量差或不可用 |

## 示例

### 示例1: 正常连接

```bash
$ sudo twnode line check tunnel_hk 8.8.8.8

检查隧道连通性...

【连通性检查结果】
接口名称: tunnel_hk
测试地址: 8.8.8.8
丢包率: 0%
平均延迟: 15.3 ms
评分: 96.2 分
状态: ✓ 良好

结果已保存到 /var/lib/trueword_node/check_results.json
```

**评分计算**:
```
丢包率评分 = (1 - 0) × 60 = 60
延迟评分 = (1 - 15.3/200) × 40 = 36.9
基础评分 = 60 + 36.9 = 96.9
成本惩罚 = 0 × 0.5 = 0
最终评分 = 96.9 - 0 = 96.9 ≈ 96.2
```

### 示例2: 有丢包

```bash
$ sudo twnode line check tunnel_us 8.8.8.8

检查隧道连通性...

【连通性检查结果】
接口名称: tunnel_us
测试地址: 8.8.8.8
丢包率: 15%
平均延迟: 80.5 ms
评分: 67.9 分
状态: ⚠ 降级

结果已保存到 /var/lib/trueword_node/check_results.json
```

**评分计算**:
```
丢包率评分 = (1 - 0.15) × 60 = 51
延迟评分 = (1 - 80.5/200) × 40 = 23.9
基础评分 = 51 + 23.9 = 74.9
成本惩罚 = 0 × 0.5 = 0
最终评分 = 74.9 - 0 = 74.9 ≈ 67.9
```

### 示例3: 连接失败

```bash
$ sudo twnode line check tunnel_down 8.8.8.8

检查隧道连通性...

【连通性检查结果】
接口名称: tunnel_down
测试地址: 8.8.8.8
丢包率: 100%
平均延迟: 0 ms
评分: 0 分
状态: ✗ 差

结果已保存到 /var/lib/trueword_node/check_results.json
```

### 示例4: 带成本惩罚

假设策略组配置中 `cost: 10`:

```bash
$ sudo twnode line check tunnel_backup 8.8.8.8

检查隧道连通性...

【连通性检查结果】
接口名称: tunnel_backup
测试地址: 8.8.8.8
丢包率: 5%
平均延迟: 50 ms
评分: 72.0 分
状态: ⚠ 降级

结果已保存到 /var/lib/trueword_node/check_results.json
```

**评分计算**:
```
丢包率评分 = (1 - 0.05) × 60 = 57
延迟评分 = (1 - 50/200) × 40 = 30
基础评分 = 57 + 30 = 87
成本惩罚 = 10 × 0.5 = 5
最终评分 = 87 - 5 = 82 → 72.0（示例）
```

## 检查结果保存

### 结果文件

检查结果保存到 `/var/lib/trueword_node/check_results.json`：

```json
{
  "tunnel_hk": {
    "interface": "tunnel_hk",
    "check_ip": "8.8.8.8",
    "packet_loss": 0,
    "avg_latency": 15.3,
    "score": 96.2,
    "status": "good",
    "timestamp": "2025-01-15T10:30:00Z"
  },
  "tunnel_us": {
    "interface": "tunnel_us",
    "check_ip": "8.8.8.8",
    "packet_loss": 15,
    "avg_latency": 80.5,
    "score": 67.9,
    "status": "degraded",
    "timestamp": "2025-01-15T10:30:05Z"
  }
}
```

### 结果用途

检查结果用于：
1. **手动分析** - 查看隧道质量
2. **故障转移** - `policy failover` 根据评分选择最佳出口
3. **监控告警** - 脚本监控隧道状态

## 批量检查

检查多个隧道：

```bash
# 方法1: 循环检查
for tunnel in tunnel_hk tunnel_us tunnel_backup; do
    sudo twnode line check $tunnel 8.8.8.8
done

# 方法2: 使用 list 和 awk
for tunnel in $(sudo twnode line list | awk 'NR>3 && NF {print $1}' | grep -v "^+"); do
    sudo twnode line check $tunnel 8.8.8.8
done

# 方法3: 并行检查（更快）
for tunnel in tunnel_hk tunnel_us tunnel_backup; do
    sudo twnode line check $tunnel 8.8.8.8 &
done
wait
```

## 定时检查

### Cron 定时任务

```bash
crontab -e
```

添加：

```bash
# 每 10 分钟检查所有隧道
*/10 * * * * /usr/local/bin/twnode line check tunnel_hk 8.8.8.8 >/dev/null 2>&1
*/10 * * * * /usr/local/bin/twnode line check tunnel_us 8.8.8.8 >/dev/null 2>&1

# 或使用脚本批量检查
*/10 * * * * /path/to/check_all_tunnels.sh
```

### 检查脚本示例

```bash
#!/bin/bash
# /path/to/check_all_tunnels.sh

TUNNELS="tunnel_hk tunnel_us tunnel_backup"
CHECK_IP="8.8.8.8"

for tunnel in $TUNNELS; do
    /usr/local/bin/twnode line check $tunnel $CHECK_IP
done

# 可选：根据评分发送告警
RESULTS="/var/lib/trueword_node/check_results.json"
if command -v jq &> /dev/null; then
    for tunnel in $TUNNELS; do
        score=$(jq -r ".\"$tunnel\".score" $RESULTS)
        if (( $(echo "$score < 60" | bc -l) )); then
            echo "警告: 隧道 $tunnel 评分过低: $score"
            # 发送告警（如邮件、webhook等）
        fi
    done
fi
```

## 常见问题

### Q: 测试会影响正常流量吗？

A: 影响极小。测试期间会添加优先级 5 的临时路由（约 4 秒），仅影响访问测试 IP 的流量。

### Q: 测试 IP 应该选什么？

A: 建议选择：
- **公共 DNS**: `8.8.8.8` (Google), `1.1.1.1` (Cloudflare)
- **目标服务器**: 实际要访问的服务器 IP
- **稳定主机**: 延迟低、稳定性好的主机

避免使用：
- 不稳定的主机
- 限速的主机
- 禁 ping 的主机

### Q: 丢包率精度是多少？

A: 5%。因为发送 20 个包，每个包代表 5%。

如需更高精度，修改源码中的包数量（如 100 个包 = 1% 精度）。

### Q: 评分公式可以自定义吗？

A: 可以修改源码 `pkg/network/check.go` 中的 `calculateScore()` 函数。

### Q: 检查失败怎么办？

A: 可能原因：
1. **隧道未启动**: 使用 `line list` 检查状态
2. **网络不通**: 检查对端是否在线
3. **防火墙阻止**: 检查 iptables 规则
4. **路由配置错误**: 检查路由规则

手动测试：

```bash
# 手动 ping 测试
ping -c 3 -I tunnel_hk 8.8.8.8

# 检查路由
ip route get 8.8.8.8 from <local_vip>
```

## 下一步

- [故障转移](../policy/failover.md) - 根据检查结果自动切换
- [策略路由](../policy/index.md) - 配置策略路由
- [列出隧道](list.md) - 查看隧道状态

---

**导航**: [← list](list.md) | [返回首页](../../index.md) | [show-peer →](show-peer.md)
