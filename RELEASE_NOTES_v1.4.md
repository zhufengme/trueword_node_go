# TrueWord Node v1.4 Release Notes

## 🎉 版本亮点

v1.4 是一个重大功能更新版本，引入了**故障转移守护进程**，为 TrueWord Node 带来了自动化、智能化的故障转移能力。

### 核心特性

- 🎯 **故障转移守护进程** - 基于评分的智能故障转移机制
- 📊 **实时健康监控** - 延迟、丢包率、Cost 综合评分系统
- 🛡️ **防抖动机制** - 评分阈值 + 双重检测确认，避免频繁切换
- 🔄 **状态实时同步** - status 命令显示守护进程实时监控状态

---

## ✨ 新增功能

### 1. 故障转移守护进程

全新的守护进程模块，支持自动监控接口健康状态并执行智能故障转移。

#### 守护进程命令

```bash
# 启动守护进程（后台运行）
twnode policy failover daemon start

# 停止守护进程
twnode policy failover daemon stop

# 查看守护进程状态
twnode policy failover daemon status

# 重载配置（无需重启）
twnode policy failover daemon reload
```

#### 监控任务管理

```bash
# 添加监控任务（交互式）
twnode policy failover add-monitor

# 删除监控任务
twnode policy failover remove-monitor <monitor_name>

# 列出所有监控任务
twnode policy failover list-monitors
```

### 2. 智能评分机制

基于多维度指标的评分系统，自动选择最佳出口：

**评分算法**：
- **丢包率评分**（0-60分，权重更高）
  - 0% 丢包: 60 分
  - ≤5% 丢包: 45 分
  - ≤10% 丢包: 30 分
  - ≤20% 丢包: 15 分
  - >20% 丢包: 0 分

- **延迟评分**（0-40分）
  - <50ms: 40 分
  - <100ms: 35 分
  - <150ms: 30 分
  - <200ms: 25 分
  - <300ms: 15 分
  - ≥300ms: 5 分

- **成本惩罚**
  - 最终评分 = 基础评分 - (Cost × 0.5)

**示例**：
```
出口A: 延迟=50ms, 丢包=0%, Cost=10
  基础评分 = 60 + 40 = 100
  成本惩罚 = 10 × 0.5 = 5
  最终评分 = 100 - 5 = 95

出口B: 延迟=30ms, 丢包=5%, Cost=0
  基础评分 = 45 + 40 = 85
  成本惩罚 = 0
  最终评分 = 85
```

### 3. 防抖动机制

避免因网络瞬时抖动导致的频繁出口切换：

#### (1) 评分差值阈值
- 新出口评分必须**显著高于**当前出口才会触发切换
- 默认阈值：5.0 分（可配置）
- 设为 0 表示任何评分提升都会切换

#### (2) 双重检测确认
- 决策切换时，重新检测当前出口和最佳出口
- 两次检测结果都确认需要切换，才执行
- 有效过滤瞬时网络波动

**配置示例**：
```yaml
daemon:
  score_threshold: 5.0  # 评分差值阈值（全局）

monitors:
  - name: "monitor-default"
    score_threshold: 10.0  # 为特定监控任务覆盖（更保守）
```

### 4. 实时状态同步

`twnode status` 命令现在可以显示守护进程的实时监控状态：

**改进前**：
- 显示 `line check` 命令的历史检查结果
- 可能与当前实际状态不一致

**改进后**：
- 守护进程运行时，优先显示实时监控状态
- 守护进程未运行时，降级显示历史检查结果
- 时间戳显示最新的检查时间

### 5. 配置文件

新增守护进程配置文件：`/etc/trueword_node/failover_daemon.yaml`

```yaml
daemon:
  # 检测间隔（毫秒）
  check_interval_ms: 500

  # 评分差值阈值（避免频繁切换）
  score_threshold: 5.0

  # 日志文件（可选）
  log_file: /var/log/twnode-failover.log

monitors:
  - name: "monitor-default"
    type: "default_route"
    target: "default"

    # 检测目标IP（按顺序尝试）
    check_targets:
      - "8.8.8.8"
      - "1.1.1.1"

    # 候选出口接口（至少2个）
    candidate_exits:
      - "tun_hk"
      - "tun_sg"
      - "tun_us"
```

---

## 🐛 重要 BUG 修复

### 1. 评分算法 BUG（严重）

**问题**：
- 接口完全失败（100% 丢包）时，延迟为 0ms
- 旧算法：延迟 0ms → 延迟评分 40 分
- **导致完全失败的接口仍然有 40 分评分**

**修复**：
```go
// 特殊情况：完全失败（100% 丢包）
if packetLoss >= 100.0 {
    return 0  // 评分直接为 0
}
```

**效果**：
```
# 修复前（错误）
TSN-HKG: 延迟=0.0ms 丢包=100% 基础分=40.0 ❌

# 修复后（正确）
TSN-HKG: DOWN [延迟=0.0ms 丢包=100% 基础分=0.0] ✓
```

### 2. 默认路由读取错误

**问题**：
- `status` 命令从配置文件读取默认路由
- 守护进程切换默认路由时，修改的是系统路由表（table 900）
- **配置文件不会实时更新，导致状态不一致**

**修复**：
- `status` 命令改为从系统实际读取默认路由（`ip route table 900`）
- 守护进程每次评估都从系统实时读取当前出口
- **确保守护进程和 status 命令的状态始终一致**

**效果**：
```bash
# 守护进程切换到 TSN-HKG
$ ip route get 1.1.1.1
1.1.1.1 dev TSN-HKG table 900

# status 命令正确显示
$ twnode status
TSN-HKG ★  ← 一致！
```

### 3. 路由冲突和残留问题

**问题**：
- 健康检查时添加临时路由（pref 5）
- `pref` 值必须全局唯一，并发检查会冲突
- 守护进程非正常退出时，临时路由残留

**修复**：
- **全局锁**：所有健康检查串行执行
- **清理机制**：
  - 每次检查前清理所有 `pref 5` 规则
  - 清理路由表中的残留路由项
  - 使用 `defer` 确保即使 panic 也会清理

**效果**：
- ✓ 彻底解决 "File exists" 错误
- ✓ 不再有残留路由规则
- ✓ 每个接口显示不同的真实延迟

---

## 🔧 技术改进

### 1. 完善的 Debug 日志

守护进程提供详细的决策过程日志：

```
【监控任务】default 开始检查 (候选: [tun_hk tun_sg tun_us])

【评分结果】监控任务: default
  tun_hk: UP [延迟=89.9ms 丢包=0% Cost=0 基础分=95.0 最终分=95.0]
  tun_sg: DOWN [延迟=0.0ms 丢包=100% Cost=0 基础分=0.0 最终分=0.0]
  tun_us: UP [延迟=150ms 丢包=5% Cost=10 基础分=75.0 最终分=70.0]

【决策】当前出口: tun_hk (评分: 95.0), 最佳出口: tun_hk (评分: 95.0)
【保持不变】监控任务 default: tun_hk 仍是最佳出口 (评分: 95.0)
```

### 2. UP/DOWN 状态显示

- 日志和状态输出清晰标记接口状态
- 100% 丢包 → DOWN
- 丢包 < 100% → UP

### 3. 健康检查优化

- 快速 ping：3 个包，间隔 0.1 秒（约 300ms）
- 支持多个检测目标（按顺序尝试，最多 3 个）
- 任何一个目标可达即认为接口 UP

### 4. 配置参数清理

移除无效参数：
- ❌ `failure_threshold`（连续失败次数）
- ❌ `recovery_threshold`（连续成功次数）

这些参数在新的评分机制下不再使用。

---

## 📚 文档更新

- 更新 `docs/commands/policy/failover.md`
  - 完整的守护进程命令说明
  - 配置文件详细说明
  - 使用示例和最佳实践

---

## 🔄 升级指南

### 从 v1.3 升级到 v1.4

1. **备份配置**（可选）
   ```bash
   sudo cp -r /etc/trueword_node /etc/trueword_node.backup
   ```

2. **下载并安装新版本**
   ```bash
   # 下载对应平台的包
   wget https://github.com/zhufengme/trueword_node_go/releases/download/v1.4/twnode-v1.4-linux-amd64.tar.gz

   # 解压
   tar -xzf twnode-v1.4-linux-amd64.tar.gz

   # 安装
   sudo cp twnode /usr/local/bin/twnode
   ```

3. **初始化守护进程配置**
   ```bash
   # 守护进程配置文件会在首次运行时自动创建
   sudo twnode policy failover add-monitor
   ```

4. **启动守护进程**
   ```bash
   sudo twnode policy failover daemon start
   ```

5. **验证运行状态**
   ```bash
   sudo twnode policy failover daemon status
   ```

### 兼容性说明

- ✅ 完全向后兼容 v1.3 的配置文件
- ✅ 现有隧道和策略组无需修改
- ✅ 可以选择性启用守护进程功能

---

## 📦 下载

### Linux 平台

- **x86_64**: [twnode-v1.4-linux-amd64.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4/twnode-v1.4-linux-amd64.tar.gz) (2.8M)
- **ARM64**: [twnode-v1.4-linux-arm64.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4/twnode-v1.4-linux-arm64.tar.gz) (2.6M)
- **i386**: [twnode-v1.4-linux-386.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4/twnode-v1.4-linux-386.tar.gz) (2.7M)
- **ARM**: [twnode-v1.4-linux-arm.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4/twnode-v1.4-linux-arm.tar.gz) (2.7M)

### 安装方法

```bash
# 下载（以 amd64 为例）
wget https://github.com/zhufengme/trueword_node_go/releases/download/v1.4/twnode-v1.4-linux-amd64.tar.gz

# 解压
tar -xzf twnode-v1.4-linux-amd64.tar.gz

# 安装
sudo cp twnode /usr/local/bin/twnode

# 验证
twnode --version
```

---

## 🙏 致谢

感谢所有参与测试和反馈的用户！

特别感谢：
- 发现评分算法 BUG 的测试用户
- 提供防抖动机制建议的用户
- 报告状态不一致问题的用户

---

## 📝 完整变更日志

### 新增
- 新增 Failover 守护进程模块 (`pkg/failover/`)
- 新增守护进程管理命令（start/stop/status/reload）
- 新增监控任务管理命令（add/remove/list）
- 新增评分机制（延迟 + 丢包率 + Cost）
- 新增防抖动机制（评分阈值 + 双重检测）
- 新增 UP/DOWN 状态显示
- 新增守护进程配置文件 `/etc/trueword_node/failover_daemon.yaml`

### 修复
- 修复评分算法 BUG（100% 丢包评分 40 → 0）
- 修复默认路由读取逻辑（配置文件 → 系统实时）
- 修复路由冲突问题（全局锁序列化）
- 修复路由残留问题（清理机制 + defer）
- 修复状态显示不一致（守护进程 vs status 命令）

### 优化
- 优化健康检查性能（快速 ping，300ms 完成）
- 优化日志输出（显示完整决策过程）
- 优化状态同步（实时显示守护进程状态）

### 移除
- 移除无效配置参数（failure_threshold, recovery_threshold）

---

## 🔗 相关链接

- 项目主页: https://github.com/zhufengme/trueword_node_go
- 问题反馈: https://github.com/zhufengme/trueword_node_go/issues
- 文档中心: `docs/index.md`

---

**发布日期**: 2025-10-26

**版本**: v1.4

🤖 Generated with [Claude Code](https://claude.com/claude-code)
