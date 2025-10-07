# 故障转移配置教程

本教程详细介绍如何使用 TrueWord Node 的故障转移功能，实现高可用的网络隧道架构。

## 教程目标

完成本教程后，你将学会：

- 理解故障转移的工作原理
- 配置自动故障转移
- 使用连通性检查和评分系统
- 利用 Cost 机制优化选择
- 设置定时任务实现自动化

## 前置条件

- 已完成系统初始化（`twnode init`）
- 至少创建了 2 个隧道
- 了解策略路由基础

## 故障转移原理

### 评分算法

TrueWord Node 使用综合评分算法选择最佳出口：

```
基础评分（0-100 分）:
  - 丢包率评分（0-60 分，权重 60%）
  - 延迟评分（0-40 分，权重 40%）

成本惩罚:
  - 惩罚分数 = Cost × 0.5

最终评分 = 基础评分 - 成本惩罚

选择规则: 最终评分最高的出口
```

### 丢包率评分计算

```
丢包率评分 = (100 - 丢包率百分比) × 0.6

示例:
  0% 丢包  → 60 分
  5% 丢包  → 57 分
  50% 丢包 → 30 分
  100% 丢包 → 0 分
```

### 延迟评分计算

```
延迟评分 = max(0, 40 - (延迟ms / 10))

示例:
  10ms  → 39 分
  50ms  → 35 分
  100ms → 30 分
  200ms → 20 分
  400ms+ → 0 分
```

### 综合示例

```
隧道 A:
  丢包率: 0%    → 60 分
  延迟: 20ms   → 38 分
  Cost: 0      → 惩罚 0 分
  最终评分: 98 分

隧道 B:
  丢包率: 5%   → 57 分
  延迟: 100ms  → 30 分
  Cost: 5      → 惩罚 2.5 分
  最终评分: 84.5 分

隧道 C:
  丢包率: 10%  → 54 分
  延迟: 200ms  → 20 分
  Cost: 10     → 惩罚 5 分
  最终评分: 69 分

选择隧道 A（最高分）
```

## 场景1: 策略组故障转移

### 前提

已创建三个隧道和一个策略组：

```bash
# 查看隧道
sudo twnode line list
```

输出：
```
+-------------+---------------+--------------+-----------------+----------------+-------------+
| 隧道名称    | 父接口        | 类型         | 对端IP          | 本地VIP        | 状态        |
+-------------+---------------+--------------+-----------------+----------------+-------------+
| tunnel_hk   | eth0          | WireGuard    | 203.0.113.10    | 10.0.0.1       | ✓ Active    |
| tunnel_us   | eth0          | WireGuard    | 198.51.100.20   | 10.0.1.1       | ✓ Active    |
| tunnel_de   | eth0          | WireGuard    | 192.0.2.30      | 10.0.2.1       | ✓ Active    |
+-------------+---------------+--------------+-----------------+----------------+-------------+
```

```bash
# 创建策略组
sudo twnode policy create vpn_traffic tunnel_hk --priority 100
sudo twnode policy add-cidr vpn_traffic 192.168.0.0/16
sudo twnode policy apply vpn_traffic
```

### 执行故障转移

#### 基本用法

```bash
# 为策略组 vpn_traffic 执行故障转移
# 候选出口: tunnel_hk, tunnel_us, tunnel_de
sudo twnode policy failover vpn_traffic tunnel_hk,tunnel_us,tunnel_de \
  --check-ip 8.8.8.8
```

#### 输出示例（主隧道正常）

```
【策略组故障转移】
策略组: vpn_traffic
候选出口: tunnel_hk, tunnel_us, tunnel_de
检查 IP: 8.8.8.8

检查连通性...
  tunnel_hk → 8.8.8.8
    ✓ Ping 成功
    丢包率: 0.0%, 延迟: 12.5 ms, Cost: 0
    评分: 96.5

  tunnel_us → 8.8.8.8
    ✓ Ping 成功
    丢包率: 0.0%, 延迟: 45.0 ms, Cost: 5
    评分: 89.5 (基础 92.0 - 成本惩罚 2.5)

  tunnel_de → 8.8.8.8
    ✓ Ping 成功
    丢包率: 5.0%, 延迟: 80.0 ms, Cost: 10
    评分: 76.0 (基础 81.0 - 成本惩罚 5.0)

最佳出口: tunnel_hk (评分 96.5)
当前出口: tunnel_hk

ℹ 出口未变化，无需切换
```

#### 输出示例（主隧道故障，自动切换）

```
【策略组故障转移】
策略组: vpn_traffic
候选出口: tunnel_hk, tunnel_us, tunnel_de
检查 IP: 8.8.8.8

检查连通性...
  tunnel_hk → 8.8.8.8
    ❌ Ping 失败 (100% 丢包)
    评分: 0.0

  tunnel_us → 8.8.8.8
    ✓ Ping 成功
    丢包率: 0.0%, 延迟: 45.0 ms, Cost: 5
    评分: 89.5

  tunnel_de → 8.8.8.8
    ✓ Ping 成功
    丢包率: 5.0%, 延迟: 80.0 ms, Cost: 10
    评分: 76.0

最佳出口: tunnel_us (评分 89.5)
当前出口: tunnel_hk

⚠️ 出口需要切换: tunnel_hk → tunnel_us

【执行切换】
撤销策略组 vpn_traffic...
✓ 已撤销 1 条路由规则

更新出口接口为 tunnel_us...
✓ 配置已更新

重新应用策略组...
✓ 192.168.0.0/16 → tunnel_us (pref 100)

✓ 故障转移完成: tunnel_hk → tunnel_us
```

### 使用历史检查结果

如果之前执行过 `line check`，可以复用历史结果，无需重新检查：

```bash
# 先执行连通性检查
sudo twnode line check tunnel_hk 8.8.8.8
sudo twnode line check tunnel_us 8.8.8.8
sudo twnode line check tunnel_de 8.8.8.8

# 执行故障转移（不指定 --check-ip，使用历史结果）
sudo twnode policy failover vpn_traffic tunnel_hk,tunnel_us,tunnel_de
```

输出：
```
【策略组故障转移】
策略组: vpn_traffic
候选出口: tunnel_hk, tunnel_us, tunnel_de

使用历史检查结果...
  tunnel_hk: 评分 96.5 (丢包率 0.0%, 延迟 12.5ms, Cost 0)
  tunnel_us: 评分 89.5 (丢包率 0.0%, 延迟 45.0ms, Cost 5)
  tunnel_de: 评分 76.0 (丢包率 5.0%, 延迟 80.0ms, Cost 10)

最佳出口: tunnel_hk (评分 96.5)
当前出口: tunnel_hk

ℹ 出口未变化，无需切换
```

## 场景2: 默认路由故障转移

### 前提

已设置默认路由：

```bash
# 设置默认路由
sudo twnode policy set-default tunnel_hk
```

### 执行故障转移

```bash
# 为默认路由执行故障转移
sudo twnode policy failover --default tunnel_hk,tunnel_us,tunnel_de \
  --check-ip 8.8.8.8
```

输出：
```
【默认路由故障转移】
候选出口: tunnel_hk, tunnel_us, tunnel_de
检查 IP: 8.8.8.8

检查连通性...
  tunnel_hk → 8.8.8.8: 评分 96.5
  tunnel_us → 8.8.8.8: 评分 89.5
  tunnel_de → 8.8.8.8: 评分 76.0

最佳出口: tunnel_hk (评分 96.5)
当前出口: tunnel_hk

ℹ 出口未变化，无需切换
```

### 默认路由切换示例

当主隧道故障时：

```
【默认路由故障转移】
候选出口: tunnel_hk, tunnel_us, tunnel_de

检查连通性...
  tunnel_hk → 8.8.8.8: 评分 0.0 (100% 丢包)
  tunnel_us → 8.8.8.8: 评分 89.5
  tunnel_de → 8.8.8.8: 评分 76.0

最佳出口: tunnel_us (评分 89.5)
当前出口: tunnel_hk

⚠️ 出口需要切换: tunnel_hk → tunnel_us

【执行切换】
撤销默认路由...
✓ 已删除规则: 0.0.0.0/0 → tunnel_hk (pref 900)

设置新默认路由...
✓ 默认路由规则已添加: 0.0.0.0/0 → tunnel_us (pref 900)

✓ 故障转移完成: tunnel_hk → tunnel_us
```

## 场景3: 自动化故障转移

### 使用 Cron 定时任务

#### 1. 创建故障转移脚本

```bash
# 创建脚本目录
sudo mkdir -p /usr/local/bin/twnode-scripts

# 创建脚本
sudo nano /usr/local/bin/twnode-scripts/auto-failover.sh
```

脚本内容：

```bash
#!/bin/bash

# 自动故障转移脚本
# 每 5 分钟执行一次

LOG_FILE="/var/log/twnode-failover.log"

# 记录日志函数
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

log "开始故障转移检查..."

# 策略组故障转移
/usr/local/bin/twnode policy failover vpn_traffic \
    tunnel_hk,tunnel_us,tunnel_de \
    --check-ip 8.8.8.8 >> "$LOG_FILE" 2>&1

if [ $? -eq 0 ]; then
    log "策略组故障转移检查完成"
else
    log "策略组故障转移检查失败"
fi

# 默认路由故障转移（如果配置了）
/usr/local/bin/twnode policy failover --default \
    tunnel_hk,tunnel_us,tunnel_de \
    --check-ip 8.8.8.8 >> "$LOG_FILE" 2>&1

if [ $? -eq 0 ]; then
    log "默认路由故障转移检查完成"
else
    log "默认路由故障转移检查失败"
fi

log "故障转移检查结束"
log "----------------------------------------"
```

#### 2. 设置执行权限

```bash
sudo chmod +x /usr/local/bin/twnode-scripts/auto-failover.sh
```

#### 3. 添加 Cron 任务

```bash
# 编辑 root 的 crontab
sudo crontab -e
```

添加：

```cron
# 每 5 分钟执行一次故障转移检查
*/5 * * * * /usr/local/bin/twnode-scripts/auto-failover.sh

# 或者每分钟执行（高可用场景）
* * * * * /usr/local/bin/twnode-scripts/auto-failover.sh
```

#### 4. 查看日志

```bash
# 实时查看日志
sudo tail -f /var/log/twnode-failover.log

# 查看最近的日志
sudo tail -n 50 /var/log/twnode-failover.log
```

日志示例：

```
[2025-01-15 10:00:01] 开始故障转移检查...
[2025-01-15 10:00:01] 策略组故障转移检查完成
[2025-01-15 10:00:02] 默认路由故障转移检查完成
[2025-01-15 10:00:02] 故障转移检查结束
[2025-01-15 10:00:02] ----------------------------------------
[2025-01-15 10:05:01] 开始故障转移检查...
[2025-01-15 10:05:02] ⚠️ 出口需要切换: tunnel_hk → tunnel_us
[2025-01-15 10:05:03] ✓ 故障转移完成: tunnel_hk → tunnel_us
[2025-01-15 10:05:03] 策略组故障转移检查完成
[2025-01-15 10:05:04] 默认路由故障转移检查完成
[2025-01-15 10:05:04] 故障转移检查结束
[2025-01-15 10:05:04] ----------------------------------------
```

## 场景4: 使用 Cost 优化选择

### Cost 的作用

Cost 值影响故障转移的选择，即使连通性相同，低 Cost 的出口也会被优先选择。

### 配置示例

#### 1. 创建策略组并设置 Cost

```bash
# 主隧道（香港，低延迟，Cost 0）
sudo twnode policy create main_traffic tunnel_hk --priority 100

# 备用隧道1（美国，中等延迟，Cost 5）
sudo twnode policy create backup1_traffic tunnel_us --priority 200

# 备用隧道2（德国，高延迟，Cost 10）
sudo twnode policy create backup2_traffic tunnel_de --priority 300
```

#### 2. 编辑配置文件设置 Cost

```bash
# 编辑美国隧道策略组配置
sudo nano /etc/trueword_node/policies/backup1_traffic.json
```

修改 `cost` 字段：

```json
{
  "name": "backup1_traffic",
  "exit_interface": "tunnel_us",
  "priority": 200,
  "from_source": "",
  "cidrs": ["0.0.0.0/0"],
  "cost": 5
}
```

同样设置德国隧道策略组的 `cost` 为 10。

#### 3. 执行故障转移

```bash
sudo twnode policy failover main_traffic tunnel_hk,tunnel_us,tunnel_de \
  --check-ip 8.8.8.8
```

假设三个隧道连通性相同（都是 0% 丢包，50ms 延迟）：

```
检查连通性...
  tunnel_hk → 8.8.8.8
    丢包率: 0.0%, 延迟: 50.0ms, Cost: 0
    基础评分: 95.0
    成本惩罚: 0.0
    最终评分: 95.0

  tunnel_us → 8.8.8.8
    丢包率: 0.0%, 延迟: 50.0ms, Cost: 5
    基础评分: 95.0
    成本惩罚: 2.5
    最终评分: 92.5

  tunnel_de → 8.8.8.8
    丢包率: 0.0%, 延迟: 50.0ms, Cost: 10
    基础评分: 95.0
    成本惩罚: 5.0
    最终评分: 90.0

最佳出口: tunnel_hk (评分 95.0)
```

### Cost 使用场景

| Cost 值 | 使用场景 |
|---------|---------|
| 0 | 首选隧道（低成本、高带宽） |
| 1-5 | 备用隧道（中等成本） |
| 6-10 | 紧急备用（高成本、低带宽） |
| 11+ | 极端备用（极高成本） |

## 场景5: 多目标检查

### 需求

不同的流量可能访问不同的目标，需要分别检查连通性。

### 配置示例

#### 1. 针对不同目标执行故障转移

```bash
# 对于访问国内服务器的流量（检查国内 IP）
sudo twnode policy failover china_traffic \
    tunnel_hk,tunnel_us,tunnel_de \
    --check-ip 223.5.5.5

# 对于访问国际服务器的流量（检查国际 IP）
sudo twnode policy failover global_traffic \
    tunnel_hk,tunnel_us,tunnel_de \
    --check-ip 8.8.8.8
```

#### 2. 使用脚本自动化

```bash
#!/bin/bash

# 多目标故障转移脚本

# 国内流量故障转移（检查阿里 DNS）
/usr/local/bin/twnode policy failover china_traffic \
    tunnel_hk,tunnel_cn \
    --check-ip 223.5.5.5

# 国际流量故障转移（检查 Google DNS）
/usr/local/bin/twnode policy failover global_traffic \
    tunnel_us,tunnel_de \
    --check-ip 8.8.8.8

# 默认路由故障转移（检查多个目标）
# 这里使用通用的 8.8.8.8
/usr/local/bin/twnode policy failover --default \
    tunnel_hk,tunnel_us,tunnel_de \
    --check-ip 8.8.8.8
```

## 高可用架构设计

### 架构1: 主备模式

```
主隧道: tunnel_hk (Cost 0)
备用隧道: tunnel_us (Cost 5)

策略:
- 主隧道故障时自动切换到备用
- 主隧道恢复后自动切换回主
```

Cron 配置：

```cron
# 每分钟检查一次（快速故障转移）
* * * * * /usr/local/bin/twnode policy failover --default tunnel_hk,tunnel_us --check-ip 8.8.8.8
```

### 架构2: 多活模式

```
隧道 A: tunnel_hk (优先级 100, Cost 0)
隧道 B: tunnel_us (优先级 200, Cost 0)
隧道 C: tunnel_de (优先级 300, Cost 0)

策略:
- 不同流量分配到不同隧道
- 每个隧道独立故障转移
```

Cron 配置：

```cron
# 每 5 分钟检查一次
*/5 * * * * /usr/local/bin/twnode policy failover traffic_a tunnel_hk,tunnel_us --check-ip 8.8.8.8
*/5 * * * * /usr/local/bin/twnode policy failover traffic_b tunnel_us,tunnel_de --check-ip 8.8.8.8
*/5 * * * * /usr/local/bin/twnode policy failover traffic_c tunnel_de,tunnel_hk --check-ip 8.8.8.8
```

### 架构3: 分级备份

```
主隧道: tunnel_main (Cost 0)
备用隧道1: tunnel_backup1 (Cost 5)
备用隧道2: tunnel_backup2 (Cost 10)
紧急备用: eth0 (Cost 20, 物理接口)

策略:
- 主隧道故障 → 备用隧道1
- 备用隧道1 故障 → 备用隧道2
- 所有隧道故障 → 物理接口
```

Cron 配置：

```cron
# 每分钟检查一次
* * * * * /usr/local/bin/twnode policy failover --default tunnel_main,tunnel_backup1,tunnel_backup2,eth0 --check-ip 8.8.8.8
```

## 监控和告警

### 监控脚本示例

```bash
#!/bin/bash

# 故障转移监控脚本
# 检测出口切换并发送告警

CURRENT_EXIT_FILE="/var/lib/twnode/current_exit.txt"
LOG_FILE="/var/log/twnode-failover.log"
ALERT_SCRIPT="/usr/local/bin/send-alert.sh"  # 自定义告警脚本

# 执行故障转移
OUTPUT=$(/usr/local/bin/twnode policy failover --default \
    tunnel_hk,tunnel_us,tunnel_de \
    --check-ip 8.8.8.8 2>&1)

# 检查是否发生切换
if echo "$OUTPUT" | grep -q "故障转移完成"; then
    # 提取切换信息
    SWITCH_INFO=$(echo "$OUTPUT" | grep "故障转移完成")

    # 记录日志
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $SWITCH_INFO" >> "$LOG_FILE"

    # 发送告警（如果配置了）
    if [ -x "$ALERT_SCRIPT" ]; then
        $ALERT_SCRIPT "TrueWord Node 故障转移" "$SWITCH_INFO"
    fi
fi
```

### 告警集成（示例）

```bash
#!/bin/bash
# /usr/local/bin/send-alert.sh

TITLE="$1"
MESSAGE="$2"

# 发送邮件告警
echo "$MESSAGE" | mail -s "$TITLE" admin@example.com

# 或发送到 Telegram（需要配置 bot token 和 chat ID）
# curl -s -X POST "https://api.telegram.org/bot$BOT_TOKEN/sendMessage" \
#     -d chat_id=$CHAT_ID \
#     -d text="$TITLE: $MESSAGE"

# 或发送到 Slack（需要配置 webhook URL）
# curl -X POST -H 'Content-type: application/json' \
#     --data "{\"text\":\"$TITLE: $MESSAGE\"}" \
#     $SLACK_WEBHOOK_URL
```

## 常见问题

### Q: 故障转移的检查间隔应该设置多久？

A: 取决于业务需求：
- 高可用场景（金融、游戏）：1 分钟
- 普通场景：5 分钟
- 备份场景：10-30 分钟

### Q: Cost 值应该如何设置？

A: 根据实际成本和优先级：
- 0: 首选隧道
- 1-5: 备用隧道
- 6-10: 紧急备用
- 11+: 极端备用

### Q: 故障转移会中断网络连接吗？

A: 短暂中断（通常 < 1 秒）：
- 切换时需要撤销旧规则、应用新规则
- 使用"先添加后清理"策略最小化中断
- TCP 连接可能需要重新建立

### Q: 如何避免频繁切换（抖动）？

A: 使用 Cost 机制：
- 主隧道 Cost 0
- 备用隧道 Cost 5-10
- 主隧道恢复后，由于 Cost 优势会自动切换回主

### Q: 可以同时对多个策略组执行故障转移吗？

A: 可以，在脚本中逐个执行：

```bash
sudo twnode policy failover group1 tunnel_hk,tunnel_us --check-ip 8.8.8.8
sudo twnode policy failover group2 tunnel_us,tunnel_de --check-ip 8.8.8.8
sudo twnode policy failover --default tunnel_hk,tunnel_us,tunnel_de --check-ip 8.8.8.8
```

## 下一步

- [策略路由实践](policy-routing.md) - 学习策略路由配置
- [嵌套隧道](nested-tunnels.md) - 多层隧道架构
- [连通性检查](../commands/line/check.md) - 详细的检查命令说明

---

**导航**: [← 策略路由实践](policy-routing.md) | [返回首页](../index.md) | [嵌套隧道 →](nested-tunnels.md)
