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

---

## 守护进程模式（自动故障转移）

**版本要求**: v1.4+

### 概述

Failover 守护进程提供毫秒级的自动故障转移，监控隧道健康状态并在检测到故障时立即切换。

**核心特性**：
- **毫秒级检测间隔**（100ms - 60秒可配置）
- **快速响应**（默认1.5秒内完成故障转移）
- **多目标容错**（最多3个检测目标IP，顺序尝试）
- **智能防抖**（连续N次失败/成功才触发切换）
- **多任务监控**（同时监控多个策略组/默认路由）
- **配置热重载**（无需重启进程）
- **状态持久化**（运行时状态可查询）

### 快速开始

#### 1. 初始化配置文件

```bash
twnode policy failover init-config
```

生成默认配置文件：`/etc/trueword_node/failover_daemon.yaml`

#### 2. 编辑配置文件

```bash
sudo nano /etc/trueword_node/failover_daemon.yaml
```

示例配置：

```yaml
daemon:
  check_interval_ms: 500      # 检测间隔 500ms
  failure_threshold: 3         # 连续失败3次判定为Down
  recovery_threshold: 3        # 连续成功3次判定为Up
  log_file: /var/log/twnode-failover.log  # 日志文件（留空则不保存）

monitors:
  - name: "monitor-cn-routes"
    type: "policy_group"       # 监控策略组
    target: "cn_routes"        # 策略组名称

    check_targets:             # 检测目标（按顺序尝试）
      - "114.114.114.114"      # 首选
      - "223.5.5.5"            # 备选1
      - "119.29.29.29"         # 备选2

    candidate_exits:           # 候选出口
      - "tun_cn1"
      - "tun_cn2"
      - "eth0"

  - name: "monitor-default"
    type: "default_route"      # 监控默认路由
    target: "default"

    check_targets:
      - "8.8.8.8"
      - "1.1.1.1"

    candidate_exits:
      - "tun_hk"
      - "tun_us"
```

#### 3. 验证配置

```bash
twnode policy failover validate-config
```

#### 4. 配置 systemd 服务

创建 `/etc/systemd/system/twnode-failover.service`：

```ini
[Unit]
Description=TrueWord Node Failover Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/twnode policy failover daemon
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s

# 安全加固
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW

[Install]
WantedBy=multi-user.target
```

#### 5. 启动守护进程

```bash
# 重载 systemd 配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start twnode-failover

# 查看状态
sudo systemctl status twnode-failover

# 查看日志
sudo journalctl -u twnode-failover -f

# 开机自启
sudo systemctl enable twnode-failover
```

### 配置管理命令

#### 查看配置

```bash
# 显示全局配置
twnode policy failover show-config

# 列出所有监控任务
twnode policy failover list-monitors

# 显示监控任务详情
twnode policy failover show-monitor monitor-cn-routes
```

#### 修改配置

```bash
# 修改全局配置
twnode policy failover set-config --interval 500 --failure-threshold 3

# 添加监控任务（交互式）
twnode policy failover add-monitor

# 添加监控任务（命令行）
twnode policy failover add-monitor my-monitor \
  --type policy_group \
  --target cn_routes \
  --check-targets 114.114.114.114,223.5.5.5 \
  --exits tun01,tun02,eth0

# 删除监控任务
twnode policy failover remove-monitor my-monitor
```

#### 运行时管理

```bash
# 重载配置（无需重启）
sudo systemctl reload twnode-failover
# 或
twnode policy failover reload

# 查看运行状态
twnode policy failover status

# 停止服务
sudo systemctl stop twnode-failover

# 重启服务
sudo systemctl restart twnode-failover
```

### 检测机制详解

#### 快速 Ping

- 每次检测发送 **3个 ICMP包**，间隔 0.1秒
- 总耗时约 **300ms**
- 任意1个包成功即判定本次检测成功
- 使用临时路由（优先级5）确保从指定接口发出

#### 多目标容错

配置文件中的 `check_targets` 按顺序尝试：

```yaml
check_targets:
  - "114.114.114.114"  # 首选：国内DNS
  - "223.5.5.5"        # 备选1：阿里DNS
  - "119.29.29.29"     # 备选2：腾讯DNS
```

检测流程：
1. 尝试 ping 114.114.114.114
2. 成功 → 本次检测成功，跳过后续目标
3. 失败 → 尝试下一个目标
4. 所有目标都失败 → 本次检测失败

#### 状态机和防抖

```
初始状态：unknown

检测成功3次（连续） → up
检测失败3次（连续） → down

up → 检测失败3次 → down → 触发故障转移
down → 检测成功3次 → up → 触发故障转移

单次失败不会触发切换（防止误判）
```

#### 时间线示例

配置：检测间隔 500ms，失败阈值 3次

```
0.0s   检测1 (300ms) ✓ → 成功计数 1/3
0.5s   检测2 (300ms) ✓ → 成功计数 2/3
1.0s   检测3 (300ms) ✓ → 成功计数 3/3 → 判定为 UP

1.5s   检测4 (300ms) ✗ → 失败计数 1/3
2.0s   检测5 (300ms) ✗ → 失败计数 2/3
2.5s   检测6 (300ms) ✗ → 失败计数 3/3 → 判定为 DOWN
       → 触发故障转移（耗时 < 100ms）
```

**故障检测到切换完成：约 1.5秒**

### 日志和调试

#### 查看日志

```bash
# 日志文件（如果配置了）
sudo tail -f /var/log/twnode-failover.log

# systemd 日志
sudo journalctl -u twnode-failover -f

# 查看最近事件
sudo twnode policy failover status
```

#### 日志示例

普通模式（仅状态变化和故障转移）：

```
2025-10-25 14:30:15 [INFO] 接口 tun01 状态变化: up → down (连续失败 3 次)
2025-10-25 14:30:15 [INFO] 执行故障转移: cn_routes (tun01 → tun02)
2025-10-25 14:30:15 [INFO] 故障转移成功
```

Debug 模式（详细日志，仅用于调试）：

```bash
# 前台运行 + 详细日志
sudo twnode policy failover daemon --debug
```

输出：

```
2025-10-25 14:30:10.123 [DEBUG] 检测 tun01 → 114.114.114.114: 成功 (RTT: 15ms)
2025-10-25 14:30:10.456 [DEBUG] 检测 tun02 → 114.114.114.114: 失败 (timeout)
2025-10-25 14:30:10.567 [DEBUG] 检测 tun02 → 223.5.5.5: 成功 (RTT: 25ms, 使用备选目标)
2025-10-25 14:30:10.789 [DEBUG] tun01 状态: up (成功 10, 失败 0)
2025-10-25 14:30:10.790 [DEBUG] tun02 状态: up (成功 8, 失败 0)
```

#### 日志轮转

如果配置了日志文件，建议配置 logrotate。

创建 `/etc/logrotate.d/twnode-failover`：

```
/var/log/twnode-failover.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    postrotate
        /bin/kill -HUP $(cat /var/run/twnode-failover.pid 2>/dev/null) 2>/dev/null || true
    endscript
}
```

### 配置建议

#### 保守配置（推荐）

```yaml
daemon:
  check_interval_ms: 500       # 500ms 间隔
  failure_threshold: 3          # 连续失败3次 → Down（需1.5秒）
  recovery_threshold: 3         # 连续成功3次 → Up（需1.5秒）
```

**特点**：平衡响应速度和稳定性

#### 激进配置（快速响应）

```yaml
daemon:
  check_interval_ms: 200       # 200ms 间隔
  failure_threshold: 3          # 连续失败3次 → Down（需0.6秒）
  recovery_threshold: 5         # 连续成功5次 → Up（需1.0秒，避免抖动）
```

**特点**：最快响应，但可能频繁切换

#### 稳定配置（避免抖动）

```yaml
daemon:
  check_interval_ms: 1000      # 1秒间隔
  failure_threshold: 5          # 连续失败5次 → Down（需5秒）
  recovery_threshold: 5         # 连续成功5次 → Up（需5秒）
```

**特点**：最稳定，但响应较慢

### 高级用法

#### 任务局部配置

监控任务可以覆盖全局配置：

```yaml
monitors:
  - name: "critical-service"
    type: "policy_group"
    target: "important_routes"
    check_targets:
      - "8.8.8.8"
    candidate_exits:
      - "tun_primary"
      - "tun_backup"

    # 覆盖全局配置（更激进）
    check_interval_ms: 200       # 200ms 间隔
    failure_threshold: 2         # 仅需2次失败
    recovery_threshold: 5        # 需5次成功（避免误恢复）
```

#### 与手动 failover 共存

守护进程和手动 `failover` 命令可以和平共存：

```bash
# 手动强制切换到特定接口
sudo twnode policy failover cn_routes tun01,tun02,eth0 --check-ip 8.8.8.8
```

守护进程会：
1. 继续监控所有候选接口
2. 如果手动切换的接口是健康的，不会切回
3. 如果手动切换的接口变为 Down，会自动切换到健康接口

## 常见问题

### Q: 守护进程模式和 cron 定时任务哪个好？

A:
- **守护进程模式**：
  - ✅ 毫秒级响应（默认1.5秒故障转移）
  - ✅ 实时监控，自动切换
  - ✅ 无需配置 cron
  - ⚠️ 需要额外的进程管理

- **Cron 定时任务**：
  - ✅ 简单，易于理解
  - ✅ 灵活，可自定义脚本
  - ⚠️ 最快响应时间 = cron 间隔（通常 ≥ 1分钟）
  - ⚠️ 需要手动配置

**建议**：生产环境推荐守护进程模式，测试环境可用 cron。

### Q: 如何确认守护进程正在工作？

A:

```bash
# 查看运行状态
sudo systemctl status twnode-failover

# 查看实时状态
sudo twnode policy failover status

# 查看日志
sudo journalctl -u twnode-failover -f
```

### Q: 修改配置后如何重载？

A:

```bash
# 方式1：通过 systemd
sudo systemctl reload twnode-failover

# 方式2：通过命令
sudo twnode policy failover reload

# 验证配置已重载
sudo twnode policy failover status
```

### Q: 守护进程占用多少资源？

A:
- **CPU**: < 1%（空闲时），< 5%（检测时）
- **内存**: < 10MB
- **网络**: 每秒约 0.5KB（假设监控10个接口）
- **磁盘**: 仅日志文件（如果启用）

### Q: 日志文件会无限增长吗？

A: 不会。配置 logrotate 可自动清理旧日志。如果不需要日志，配置文件中 `log_file` 留空即可。

## 下一步

- [连通性检查](../line/check.md) - 检查隧道状态
- [应用策略](apply.md) - 手动应用策略
- [故障转移守护进程教程](../../tutorials/failover-daemon-setup.md) - 完整配置指南

---

**导航**: [← set-default](set-default.md) | [返回首页](../../index.md) | [policy 命令](index.md)
