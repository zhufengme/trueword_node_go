# policy failover - 故障转移

## 概述

`policy failover` 命令根据连通性检查结果，自动选择最佳出口接口并切换策略路由或默认路由。

## 语法

```bash
# 策略组故障转移
sudo twnode policy failover <策略组名> <候选接口列表>

# 默认路由故障转移
sudo twnode policy failover default <候选接口列表>

# 指定测试 IP（可选）
sudo twnode policy failover <策略组名> <候选接口列表> --check-ip <测试IP>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<策略组名>` | 要切换的策略组名称（或 `default`） | 是 |
| `<候选接口列表>` | 候选出口接口，逗号分隔 | 是 |
| `--check-ip` | 测试 IP 地址（默认使用历史结果） | 否 |

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

### 选择最佳接口

```
1. 对所有候选接口计算评分
2. 选择评分最高的接口
3. 如果评分相同，优先选择当前接口（避免频繁切换）
4. 切换策略组出口到最佳接口
5. 重新应用策略
```

## 示例

### 示例1: 策略组故障转移（使用历史结果）

```bash
$ sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us,tunnel_backup

【故障转移】策略组: asia_traffic
候选接口: tunnel_hk, tunnel_us, tunnel_backup

使用历史检查结果...

【接口评分】
tunnel_hk:
  丢包率: 0%, 延迟: 15.3ms
  基础评分: 96.9
  成本惩罚: 0
  最终评分: 96.9 ✓ 最佳

tunnel_us:
  丢包率: 5%, 延迟: 80.5ms
  基础评分: 74.9
  成本惩罚: 0
  最终评分: 74.9

tunnel_backup:
  丢包率: 0%, 延迟: 120ms
  基础评分: 84.0
  成本惩罚: 5.0 (cost=10)
  最终评分: 79.0

选择最佳接口: tunnel_hk (评分: 96.9)
当前出口: tunnel_hk

✓ 接口未变化，无需切换
```

### 示例2: 需要切换接口

```bash
$ sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us

【故障转移】策略组: asia_traffic
候选接口: tunnel_hk, tunnel_us

使用历史检查结果...

【接口评分】
tunnel_hk:
  丢包率: 80%, 延迟: 500ms
  基础评分: 12.0
  成本惩罚: 0
  最终评分: 12.0

tunnel_us:
  丢包率: 5%, 延迟: 80ms
  基础评分: 74.9
  成本惩罚: 0
  最终评分: 74.9 ✓ 最佳

选择最佳接口: tunnel_us (评分: 74.9)
当前出口: tunnel_hk

⚠️ 需要切换接口: tunnel_hk → tunnel_us

【切换策略组】
✓ 更新配置文件
✓ 重新应用策略

✓ 故障转移完成
```

### 示例3: 重新检查（指定测试 IP）

```bash
$ sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us --check-ip 8.8.8.8

【故障转移】策略组: asia_traffic
候选接口: tunnel_hk, tunnel_us
测试 IP: 8.8.8.8

重新检查接口连通性...

检查 tunnel_hk...
✓ 丢包率: 0%, 延迟: 16.2ms, 评分: 96.4

检查 tunnel_us...
✓ 丢包率: 0%, 延迟: 85ms, 评分: 77.0

【接口评分】
tunnel_hk: 96.4 ✓ 最佳
tunnel_us: 77.0

选择最佳接口: tunnel_hk (评分: 96.4)
当前出口: tunnel_hk

✓ 接口未变化，无需切换
```

### 示例4: 默认路由故障转移

```bash
$ sudo twnode policy failover default tunnel_hk,tunnel_us

【故障转移】默认路由
候选接口: tunnel_hk, tunnel_us

使用历史检查结果...

【接口评分】
tunnel_hk: 96.9 ✓ 最佳
tunnel_us: 74.9

选择最佳接口: tunnel_hk (评分: 96.9)
当前默认路由: tunnel_us

⚠️ 需要切换接口: tunnel_us → tunnel_hk

【切换默认路由】
✓ 更新配置文件
✓ 重新应用默认路由

✓ 故障转移完成
```

## 检查结果来源

### 使用历史结果（默认）

从 `/var/lib/trueword_node/check_results.json` 读取：

```bash
sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us
```

**前提条件**: 之前执行过 `line check` 命令。

### 重新检查（指定 --check-ip）

实时检查所有候选接口：

```bash
sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us --check-ip 8.8.8.8
```

**测试时间**: 候选接口数量 × 4秒（每个接口测试约 4 秒）

## 自动化故障转移

### Cron 定时任务

```bash
crontab -e
```

添加：

```bash
# 每 10 分钟检查一次
*/10 * * * * /usr/local/bin/twnode line check tunnel_hk 8.8.8.8
*/10 * * * * /usr/local/bin/twnode line check tunnel_us 8.8.8.8

# 每 15 分钟执行故障转移
*/15 * * * * /usr/local/bin/twnode policy failover asia_traffic tunnel_hk,tunnel_us
```

### 故障转移脚本

```bash
#!/bin/bash
# /usr/local/bin/auto_failover.sh

POLICY_GROUP="asia_traffic"
CANDIDATES="tunnel_hk,tunnel_us,tunnel_backup"
CHECK_IP="8.8.8.8"

# 检查所有候选接口
for tunnel in $(echo $CANDIDATES | tr ',' ' '); do
    /usr/local/bin/twnode line check $tunnel $CHECK_IP
done

# 执行故障转移
/usr/local/bin/twnode policy failover $POLICY_GROUP $CANDIDATES

# 可选：发送通知
if [ $? -eq 0 ]; then
    echo "故障转移成功: $POLICY_GROUP"
else
    echo "故障转移失败: $POLICY_GROUP" | mail -s "Failover Alert" admin@example.com
fi
```

添加到 cron：

```bash
*/15 * * * * /usr/local/bin/auto_failover.sh
```

## 最佳实践

### 1. 定期检查 + 定期故障转移

```bash
# 检查频率高于故障转移频率
*/5 * * * * /usr/local/bin/twnode line check tunnel_hk 8.8.8.8
*/5 * * * * /usr/local/bin/twnode line check tunnel_us 8.8.8.8
*/15 * * * * /usr/local/bin/twnode policy failover asia_traffic tunnel_hk,tunnel_us
```

### 2. 使用成本调整优先级

在配置文件中设置 Cost：

```json
{
  "name": "backup_route",
  "exit_interface": "tunnel_backup",
  "priority": 800,
  "cidrs": ["192.168.100.0/24"],
  "cost": 10
}
```

即使 `tunnel_backup` 连通性良好，由于成本惩罚，会优先选择其他接口。

### 3. 避免频繁切换

```bash
# 设置较长的故障转移间隔
*/30 * * * * /usr/local/bin/twnode policy failover asia_traffic tunnel_hk,tunnel_us
```

或在脚本中添加切换阈值：

```bash
# 只有评分差距 > 20 才切换
current_score=...
new_score=...
if (( $(echo "$new_score - $current_score > 20" | bc -l) )); then
    twnode policy failover ...
fi
```

### 4. 监控和告警

记录故障转移日志：

```bash
*/15 * * * * /usr/local/bin/twnode policy failover asia_traffic tunnel_hk,tunnel_us >> /var/log/twnode_failover.log 2>&1
```

检查日志并发送告警：

```bash
#!/bin/bash
# check_failover_log.sh

LOG="/var/log/twnode_failover.log"
LAST_HOUR=$(date -d '1 hour ago' +%Y-%m-%d\ %H:00)

if grep -q "需要切换接口" $LOG | grep "$LAST_HOUR"; then
    echo "检测到故障转移事件" | mail -s "Failover Alert" admin@example.com
fi
```

## 常见问题

### Q: 没有历史检查结果怎么办？

A: 使用 `--check-ip` 参数重新检查：

```bash
sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us --check-ip 8.8.8.8
```

或先执行 check 再 failover：

```bash
sudo twnode line check tunnel_hk 8.8.8.8
sudo twnode line check tunnel_us 8.8.8.8
sudo twnode policy failover asia_traffic tunnel_hk,tunnel_us
```

### Q: 所有候选接口评分都很低怎么办？

A: failover 会选择评分最高的接口，即使评分很低。建议：
1. 检查所有接口是否正常
2. 检查网络连接
3. 添加更多候选接口

### Q: 如何强制切换到特定接口？

A: 直接修改配置并重新 apply：

```bash
# 修改配置文件
sudo nano /etc/trueword_node/policies/asia_traffic.json
# 修改 exit_interface 字段

# 重新应用
sudo twnode policy apply asia_traffic
```

### Q: failover 频率应该设置多少？

A: 建议：
- **检查频率**: 每 5-10 分钟
- **故障转移频率**: 每 15-30 分钟

避免过于频繁的切换导致网络不稳定。

## 下一步

- [连通性检查](../line/check.md) - 检查隧道状态
- [应用策略](apply.md) - 手动应用策略
- [故障转移教程](../../tutorials/failover-setup.md) - 完整配置方案

---

**导航**: [← set-default](set-default.md) | [返回首页](../../index.md) | [policy 命令](index.md)
