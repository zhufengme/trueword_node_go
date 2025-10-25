# TrueWord Node v1.4.1 Release Notes

## 🐛 重要 Bug 修复

v1.4.1 是一个紧急 bug 修复版本，解决了守护进程无法正确检测物理接口健康状态的严重问题。

---

## 问题描述

**影响版本**：v1.4

**症状**：
- 守护进程检测物理接口（如 `eth0`、`ens33`）时，始终显示 **DOWN** 状态
- 命令行 `twnode policy failover` 命令工作正常，能够正确检测物理接口
- 导致包含物理接口的 failover 策略无法正常工作

**示例**：
```bash
# 守护进程日志（错误）
enp2s0: DOWN [延迟=0.0ms 丢包=100% Cost=0 基础分=0.0 最终分=0.0]

# 命令行模式（正确）
✓ enp2s0 (175.2ms, 0% 丢包)
```

---

## 根本原因

守护进程的健康检查器 (`pkg/failover/health_checker.go`) 与命令行模式 (`pkg/network/check.go`) 在处理物理接口时的路由添加逻辑不一致：

### 守护进程（错误）
```go
// 所有接口都使用相同的路由添加命令
cmdRoute := exec.Command("ip", "route", "add", target, "dev", iface, "table", table)
```

**问题**：物理接口需要通过网关路由，而不是直接通过设备。

### 命令行模式（正确）
```go
// 检查是否是物理接口，读取网关信息
if isPhysical && gateway != "" {
    // 物理接口：通过网关路由
    cmdRoute = exec.Command("ip", "route", "add", targetIP, "via", gateway, "dev", exitInterface, "table", tableID)
} else {
    // 隧道：直接通过设备路由
    cmdRoute = exec.Command("ip", "route", "add", targetIP, "dev", exitInterface, "table", tableID)
}
```

---

## 修复内容

### 文件变更

**`pkg/failover/health_checker.go`**

1. **添加导入**：
   ```go
   import "trueword_node/pkg/network"
   ```

2. **更新 `addTestRoute()` 函数**：
   - 加载物理接口配置以获取网关信息
   - 根据接口类型选择正确的路由添加命令
   - 物理接口使用 `via gateway`，隧道接口直接使用 `dev`

3. **更新 `removeTestRoute()` 函数**：
   - 使用更通用的删除命令（不指定 `dev`）

### 核心逻辑

```go
// 检查是否是物理接口（通过查找配置）
ifaceConfig, err := network.LoadInterfaceConfig()
var gateway string
isPhysical := false

if err == nil {
    for _, physIface := range ifaceConfig.Interfaces {
        if physIface.Name == iface {
            isPhysical = true
            gateway = physIface.Gateway
            break
        }
    }
}

// 添加路由
var cmdRoute *exec.Cmd
if isPhysical && gateway != "" {
    // 物理接口：通过网关路由
    cmdRoute = exec.Command("ip", "route", "add", target, "via", gateway, "dev", iface, "table", table)
} else {
    // 隧道或无网关的P2P连接：直接通过设备路由
    cmdRoute = exec.Command("ip", "route", "add", target, "dev", iface, "table", table)
}
```

---

## 修复效果

### 修复前（v1.4）
```bash
# 守护进程 Debug 日志
2025-10-26 02:30:00 [DEBUG] 检测 enp2s0 → 8.8.8.8: 失败
2025-10-26 02:30:00 [DEBUG]   enp2s0: DOWN [延迟=0.0ms 丢包=100% Cost=0 基础分=0.0 最终分=0.0]
```

### 修复后（v1.4.1）
```bash
# 守护进程 Debug 日志
2025-10-26 02:33:50 [DEBUG] 检测 enp2s0 → 8.8.8.8: 成功 [延迟: 176.4ms, 丢包: 0%]
2025-10-26 02:33:50 [DEBUG]   enp2s0: UP [延迟=176.4ms 丢包=0% Cost=0 基础分=85.0 最终分=85.0]
```

---

## 影响范围

### 受影响的场景
- ✅ 守护进程监控包含物理接口的故障转移策略
- ✅ 守护进程进行物理接口健康检查

### 不受影响的场景
- ✅ 命令行模式的 failover 命令（一直工作正常）
- ✅ 隧道接口的健康检查（守护进程和命令行均正常）
- ✅ 其他所有功能

---

## 升级指南

### 从 v1.4 升级到 v1.4.1

1. **下载并安装新版本**
   ```bash
   # 下载对应平台的包
   wget https://github.com/zhufengme/trueword_node_go/releases/download/v1.4.1/twnode-v1.4.1-linux-amd64.tar.gz

   # 解压
   tar -xzf twnode-v1.4.1-linux-amd64.tar.gz

   # 安装
   sudo cp twnode /usr/local/bin/twnode
   ```

2. **重启守护进程**（如果正在运行）
   ```bash
   sudo twnode policy failover daemon stop
   sudo twnode policy failover daemon start
   ```

3. **验证修复**
   ```bash
   # 查看守护进程状态（应该能看到物理接口正确的 UP/DOWN 状态）
   sudo twnode policy failover daemon status

   # 或查看 debug 日志
   sudo twnode policy failover daemon stop
   sudo twnode policy failover daemon start --debug
   ```

### 兼容性说明

- ✅ 完全向后兼容 v1.4 的配置文件
- ✅ 无需修改现有配置
- ✅ 无破坏性变更

---

## 📦 下载

### Linux 平台

- **x86_64**: [twnode-v1.4.1-linux-amd64.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4.1/twnode-v1.4.1-linux-amd64.tar.gz)
- **ARM64**: [twnode-v1.4.1-linux-arm64.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4.1/twnode-v1.4.1-linux-arm64.tar.gz)
- **i386**: [twnode-v1.4.1-linux-386.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4.1/twnode-v1.4.1-linux-386.tar.gz)
- **ARM**: [twnode-v1.4.1-linux-arm.tar.gz](https://github.com/zhufengme/trueword_node_go/releases/download/v1.4.1/twnode-v1.4.1-linux-arm.tar.gz)

### 安装方法

```bash
# 下载（以 amd64 为例）
wget https://github.com/zhufengme/trueword_node_go/releases/download/v1.4.1/twnode-v1.4.1-linux-amd64.tar.gz

# 解压
tar -xzf twnode-v1.4.1-linux-amd64.tar.gz

# 安装
sudo cp twnode /usr/local/bin/twnode

# 验证
twnode version
```

---

## 📝 完整变更日志

### 修复
- 修复守护进程无法正确检测物理接口健康状态的严重 bug
- 守护进程健康检查器现在与命令行模式使用一致的路由添加逻辑
- 物理接口检测现在能够正确读取网关信息并通过网关路由

### 技术细节
- 更新 `pkg/failover/health_checker.go`
  - 导入 `trueword_node/pkg/network` 包
  - 更新 `addTestRoute()` 函数以支持物理接口网关路由
  - 更新 `removeTestRoute()` 函数使用更通用的删除命令

---

## 🙏 致谢

感谢用户报告此问题！

特别感谢：
- 发现守护进程物理接口检测异常的用户
- 提供详细测试环境和日志的用户

---

## 🔗 相关链接

- 项目主页: https://github.com/zhufengme/trueword_node_go
- 问题反馈: https://github.com/zhufengme/trueword_node_go/issues
- 文档中心: `docs/index.md`
- v1.4 Release Notes: `RELEASE_NOTES_v1.4.md`

---

**发布日期**: 2025-10-26

**版本**: v1.4.1

🤖 Generated with [Claude Code](https://claude.com/claude-code)
