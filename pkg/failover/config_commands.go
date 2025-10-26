package failover

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/olekukonko/tablewriter"
)

// InitConfig 初始化配置文件
func InitConfig(interactive bool) error {
	configFile := DefaultConfigFile

	// 检查是否已存在
	if _, err := os.Stat(configFile); err == nil {
		fmt.Println("⚠ 配置文件已存在:", configFile)
		if !interactive {
			return fmt.Errorf("配置文件已存在")
		}

		fmt.Print("是否覆盖? (yes/no): ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "yes" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	// 确保目录存在
	dir := "/etc/trueword_node"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 生成默认配置
	content := generateDefaultConfig()
	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	fmt.Println("✓ 已创建配置文件:", configFile)
	fmt.Println()
	fmt.Println("请编辑配置文件，然后运行:")
	fmt.Println("  twnode policy failover validate-config")
	fmt.Println("  sudo systemctl start twnode-failover")

	return nil
}

// generateDefaultConfig 生成默认配置
func generateDefaultConfig() string {
	return `# TrueWord Node Failover 守护进程配置

# ============================================
# 全局配置
# ============================================
daemon:
  # 检测间隔（毫秒）
  # 范围: 100-60000
  # 推荐: 500（毫秒级响应），1000（稳定优先）
  check_interval_ms: 500

  # 评分差值阈值（避免频繁切换）
  # 范围: 0-100
  # 说明: 新出口评分必须比当前出口高出此阈值才会切换
  # 推荐: 5.0（平衡稳定性和响应性）
  # 设为 0 表示任何评分提升都会切换
  score_threshold: 5.0

  # 切换确认次数（避免网络抖动导致频繁切换）
  # 范围: 1-10
  # 说明: 需要连续N次检测都确认需要切换，才真正执行
  # 推荐: 3（500ms × 3 = 1.5秒确认时间）
  # 设为 1 表示立即切换（v1.4.1 之前的行为）
  # switch_confirmation_count: 3

  # 日志文件路径（留空则不保存日志）
  # log_file: /var/log/twnode-failover.log
  log_file: ""

# ============================================
# 监控任务列表
# ============================================
# 使用 'twnode policy failover add-monitor' 命令添加监控任务
# 或手动编辑此配置文件

monitors: []

# 监控任务示例:
#
# - name: "monitor-cn-routes"
#   type: "policy_group"          # 类型: policy_group 或 default_route
#   target: "cn_routes"            # 策略组名称
#
#   # 检测目标IP（按顺序尝试，最多3个）
#   check_targets:
#     - "114.114.114.114"          # 国内DNS（首选）
#     - "223.5.5.5"                # 阿里DNS（备选）
#     - "119.29.29.29"             # 腾讯DNS（备选）
#
#   # 候选出口接口（至少2个）
#   candidate_exits:
#     - "tun_cn1"
#     - "tun_cn2"
#     - "eth0"
#
#   # 可选: 覆盖全局配置
#   # check_interval_ms: 300
#   # score_threshold: 10.0
#   # switch_confirmation_count: 5  # 关键业务可设置更保守的值
#
# - name: "monitor-default"
#   type: "default_route"
#   target: "default"              # 默认路由固定使用 "default"
#
#   check_targets:
#     - "8.8.8.8"                  # Google DNS
#     - "1.1.1.1"                  # Cloudflare DNS
#
#   candidate_exits:
#     - "tun_hk"
#     - "tun_us"
#     - "tun_sg"
`
}

// ValidateConfig 验证配置文件
func ValidateConfig() error {
	config, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		return err
	}

	if err := config.Validate(); err != nil {
		fmt.Printf("✗ %v\n", err)
		return err
	}

	fmt.Println("✓ 配置文件格式正确")
	fmt.Println("✓ 所有监控任务配置有效")
	fmt.Println("✓ 全局配置参数在有效范围内")

	return nil
}

// ShowConfig 显示全局配置
func ShowConfig() error {
	config, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		return err
	}

	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║  Failover 守护进程全局配置              ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("检测间隔: %dms\n", config.Daemon.CheckIntervalMs)
	fmt.Printf("评分阈值: %.1f\n", config.Daemon.ScoreThreshold)

	// 显示切换确认次数（获取实际值，包括默认值）
	confirmationCount := config.Daemon.SwitchConfirmationCount
	if confirmationCount == 0 {
		confirmationCount = 1 // 默认值
	}
	fmt.Printf("切换确认次数: %d 次\n", confirmationCount)

	// 仅在配置了废弃字段时显示（向后兼容）
	if config.Daemon.FailureThreshold > 0 {
		fmt.Printf("失败阈值: %d 次 [已废弃]\n", config.Daemon.FailureThreshold)
	}
	if config.Daemon.RecoveryThreshold > 0 {
		fmt.Printf("恢复阈值: %d 次 [已废弃]\n", config.Daemon.RecoveryThreshold)
	}
	if config.Daemon.LogFile != "" {
		fmt.Printf("日志文件: %s\n", config.Daemon.LogFile)
	} else {
		fmt.Println("日志文件: 未配置（不保存日志）")
	}
	fmt.Printf("配置文件: %s\n", DefaultConfigFile)
	fmt.Printf("监控任务数: %d\n", len(config.Monitors))

	return nil
}

// SetConfig 修改全局配置
func SetConfig(interval, failThreshold, recvThreshold, switchConfirmCount int, scoreThreshold float64, logFile string) error {
	config, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		return err
	}

	if interval > 0 {
		config.Daemon.CheckIntervalMs = interval
	}
	// 废弃参数：仍然支持以保持向后兼容
	if failThreshold > 0 {
		config.Daemon.FailureThreshold = failThreshold
	}
	if recvThreshold > 0 {
		config.Daemon.RecoveryThreshold = recvThreshold
	}
	if scoreThreshold > 0 {
		config.Daemon.ScoreThreshold = scoreThreshold
	}
	if switchConfirmCount > 0 {
		config.Daemon.SwitchConfirmationCount = switchConfirmCount
	}
	if logFile != "" {
		config.Daemon.LogFile = logFile
	}

	// 验证新配置
	if err := config.Validate(); err != nil {
		return err
	}

	// 保存配置
	if err := SaveConfig(DefaultConfigFile, config); err != nil {
		return err
	}

	fmt.Println("✓ 全局配置已更新")
	fmt.Println("提示: 如果守护进程正在运行，请执行以下命令重载配置:")
	fmt.Println("  sudo systemctl reload twnode-failover")

	return nil
}

// ListMonitors 列出所有监控任务
func ListMonitors() error {
	config, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		return err
	}

	if len(config.Monitors) == 0 {
		fmt.Println("暂无监控任务")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("任务名称", "类型", "目标", "候选出口", "检测间隔")

	for _, monitor := range config.Monitors {
		monitorType := map[string]string{
			"policy_group":   "策略组",
			"default_route":  "默认路由",
		}[monitor.Type]

		exits := strings.Join(monitor.CandidateExits, ",")
		interval := fmt.Sprintf("%dms", monitor.GetCheckInterval(config.Daemon.CheckIntervalMs))

		table.Append(
			monitor.Name,
			monitorType,
			monitor.Target,
			exits,
			interval,
		)
	}

	table.Render()
	return nil
}

// AddMonitorInteractive 交互式添加监控任务
func AddMonitorInteractive() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("添加监控任务")
	fmt.Println()

	// 任务名称
	fmt.Print("请输入任务名称: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("任务名称不能为空")
	}

	// 类型
	fmt.Println("选择类型:")
	fmt.Println("  [1] 策略组")
	fmt.Println("  [2] 默认路由")
	fmt.Print("请选择 (1-2): ")
	typeChoice, _ := reader.ReadString('\n')
	typeChoice = strings.TrimSpace(typeChoice)

	var monitorType, target string
	if typeChoice == "1" {
		monitorType = "policy_group"
		fmt.Print("输入目标策略组名称: ")
		target, _ = reader.ReadString('\n')
		target = strings.TrimSpace(target)
	} else if typeChoice == "2" {
		monitorType = "default_route"
		target = "default"
	} else {
		return fmt.Errorf("无效的选择")
	}

	// 检测目标IP
	fmt.Print("输入检测目标IP（最多3个，逗号分隔）: ")
	targetsStr, _ := reader.ReadString('\n')
	targetsStr = strings.TrimSpace(targetsStr)
	checkTargets := strings.Split(targetsStr, ",")
	for i := range checkTargets {
		checkTargets[i] = strings.TrimSpace(checkTargets[i])
	}

	// 候选出口
	fmt.Print("输入候选出口（逗号分隔）: ")
	exitsStr, _ := reader.ReadString('\n')
	exitsStr = strings.TrimSpace(exitsStr)
	candidateExits := strings.Split(exitsStr, ",")
	for i := range candidateExits {
		candidateExits[i] = strings.TrimSpace(candidateExits[i])
	}

	// 可选参数
	fmt.Print("检测间隔（毫秒，默认使用全局配置，直接回车跳过）: ")
	intervalStr, _ := reader.ReadString('\n')
	intervalStr = strings.TrimSpace(intervalStr)
	var interval int
	if intervalStr != "" {
		interval, _ = strconv.Atoi(intervalStr)
	}

	fmt.Print("失败阈值（次数，默认使用全局配置，直接回车跳过）[已废弃]: ")
	failStr, _ := reader.ReadString('\n')
	failStr = strings.TrimSpace(failStr)
	var failThreshold int
	if failStr != "" {
		failThreshold, _ = strconv.Atoi(failStr)
	}

	fmt.Print("恢复阈值（次数，默认使用全局配置，直接回车跳过）[已废弃]: ")
	recvStr, _ := reader.ReadString('\n')
	recvStr = strings.TrimSpace(recvStr)
	var recvThreshold int
	if recvStr != "" {
		recvThreshold, _ = strconv.Atoi(recvStr)
	}

	fmt.Print("评分差值阈值（0-100，默认使用全局配置，直接回车跳过）: ")
	scoreStr, _ := reader.ReadString('\n')
	scoreStr = strings.TrimSpace(scoreStr)
	var scoreThreshold float64
	if scoreStr != "" {
		scoreThreshold, _ = strconv.ParseFloat(scoreStr, 64)
	}

	fmt.Print("切换确认次数（1-10，默认使用全局配置，直接回车跳过）: ")
	confirmStr, _ := reader.ReadString('\n')
	confirmStr = strings.TrimSpace(confirmStr)
	var switchConfirmCount int
	if confirmStr != "" {
		switchConfirmCount, _ = strconv.Atoi(confirmStr)
	}

	// 添加监控任务
	monitor := MonitorConfig{
		Name:                    name,
		Type:                    monitorType,
		Target:                  target,
		CheckTargets:            checkTargets,
		CandidateExits:          candidateExits,
		CheckIntervalMs:         interval,
		FailureThreshold:        failThreshold,
		RecoveryThreshold:       recvThreshold,
		ScoreThreshold:          scoreThreshold,
		SwitchConfirmationCount: switchConfirmCount,
	}

	return AddMonitor(monitor)
}

// AddMonitor 添加监控任务
func AddMonitor(monitor MonitorConfig) error {
	config, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		return err
	}

	if err := config.AddMonitor(monitor); err != nil {
		return err
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return err
	}

	// 保存配置
	if err := SaveConfig(DefaultConfigFile, config); err != nil {
		return err
	}

	fmt.Printf("✓ 监控任务 '%s' 已添加\n", monitor.Name)
	fmt.Println("提示: 如果守护进程正在运行，请执行以下命令重载配置:")
	fmt.Println("  sudo systemctl reload twnode-failover")

	return nil
}

// RemoveMonitor 删除监控任务
func RemoveMonitor(name string, force bool) error {
	config, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		return err
	}

	// 确认删除
	if !force {
		fmt.Printf("⚠ 确认删除监控任务 '%s'? (yes/no): ", name)
		var answer string
		fmt.Scanln(&answer)
		if answer != "yes" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	if err := config.RemoveMonitor(name); err != nil {
		return err
	}

	// 保存配置
	if err := SaveConfig(DefaultConfigFile, config); err != nil {
		return err
	}

	fmt.Printf("✓ 监控任务 '%s' 已删除\n", name)
	fmt.Println("提示: 如果守护进程正在运行，请执行以下命令重载配置:")
	fmt.Println("  sudo systemctl reload twnode-failover")

	return nil
}

// ShowMonitor 显示监控任务详情
func ShowMonitor(name string) error {
	config, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		return err
	}

	monitor := config.GetMonitor(name)
	if monitor == nil {
		return fmt.Errorf("监控任务 '%s' 不存在", name)
	}

	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Printf("║  监控任务: %-28s ║\n", name)
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("【基本信息】")
	typeStr := map[string]string{
		"policy_group":  "策略组",
		"default_route": "默认路由",
	}[monitor.Type]
	fmt.Printf("  类型: %s\n", typeStr)
	fmt.Printf("  目标: %s\n", monitor.Target)
	fmt.Println()

	fmt.Println("【检测配置】")
	fmt.Printf("  检测间隔: %dms", monitor.GetCheckInterval(config.Daemon.CheckIntervalMs))
	if monitor.CheckIntervalMs > 0 {
		fmt.Printf(" (自定义)\n")
	} else {
		fmt.Printf(" (全局配置)\n")
	}
	fmt.Printf("  评分阈值: %.1f", monitor.GetScoreThreshold(config.Daemon.ScoreThreshold))
	if monitor.ScoreThreshold > 0 {
		fmt.Printf(" (自定义)\n")
	} else {
		fmt.Printf(" (全局配置)\n")
	}
	fmt.Printf("  切换确认次数: %d 次", monitor.GetSwitchConfirmationCount(config.Daemon.SwitchConfirmationCount))
	if monitor.SwitchConfirmationCount > 0 {
		fmt.Printf(" (自定义)\n")
	} else {
		fmt.Printf(" (全局配置)\n")
	}
	// 仅在配置了废弃字段时显示
	if monitor.FailureThreshold > 0 || config.Daemon.FailureThreshold > 0 {
		fmt.Printf("  失败阈值: %d 次 [已废弃]", monitor.GetFailureThreshold(config.Daemon.FailureThreshold))
		if monitor.FailureThreshold > 0 {
			fmt.Printf(" (自定义)\n")
		} else {
			fmt.Printf(" (全局配置)\n")
		}
	}
	if monitor.RecoveryThreshold > 0 || config.Daemon.RecoveryThreshold > 0 {
		fmt.Printf("  恢复阈值: %d 次 [已废弃]", monitor.GetRecoveryThreshold(config.Daemon.RecoveryThreshold))
		if monitor.RecoveryThreshold > 0 {
			fmt.Printf(" (自定义)\n")
		} else {
			fmt.Printf(" (全局配置)\n")
		}
	}
	fmt.Println()

	fmt.Println("【检测目标】")
	for i, target := range monitor.CheckTargets {
		if i == 0 {
			fmt.Printf("  %d. %s (首选)\n", i+1, target)
		} else {
			fmt.Printf("  %d. %s (备选)\n", i+1, target)
		}
	}
	fmt.Println()

	fmt.Println("【候选出口】")
	for _, exit := range monitor.CandidateExits {
		fmt.Printf("  - %s\n", exit)
	}

	return nil
}

// ReloadDaemon 重载守护进程配置
func ReloadDaemon() error {
	// 发送 SIGHUP 信号
	err := SendSignal(syscall.SIGHUP)
	if err != nil {
		return err
	}

	pid, _ := GetRunningPID()
	fmt.Printf("✓ 已发送 SIGHUP 信号到守护进程 (PID: %d)\n", pid)
	fmt.Println("✓ 守护进程将重新加载配置")

	return nil
}

// ShowStatus 显示守护进程状态
func ShowStatus() error {
	// 检查守护进程是否运行
	pid, err := GetRunningPID()
	if err != nil {
		fmt.Println("守护进程未运行")
		return nil
	}

	// 加载运行时状态
	state, err := LoadState()
	if err != nil {
		fmt.Printf("运行状态: 运行中 (PID: %d)\n", pid)
		fmt.Println("状态文件不存在或无法读取")
		return nil
	}

	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║  Failover 守护进程状态                  ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	// 运行时长
	uptime := time.Since(state.StartTime)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60
	fmt.Printf("运行状态: 运行中 (PID: %d)\n", pid)
	fmt.Printf("运行时长: %d小时%d分钟\n", hours, minutes)
	fmt.Printf("配置文件: %s\n", DefaultConfigFile)

	// 加载配置
	config, err := LoadConfig(DefaultConfigFile)
	if err == nil {
		fmt.Printf("监控任务: %d 个\n", len(config.Monitors))
		if config.Daemon.LogFile != "" {
			fmt.Printf("日志文件: %s\n", config.Daemon.LogFile)
		}
	}

	fmt.Println()

	// 最近事件
	if len(state.RecentEvents) > 0 {
		fmt.Println("【最近事件】")
		count := len(state.RecentEvents)
		if count > 10 {
			count = 10
		}
		for i := len(state.RecentEvents) - count; i < len(state.RecentEvents); i++ {
			event := state.RecentEvents[i]
			timeStr := event.Timestamp.Format("15:04:05")
			fmt.Printf("  [%s] %s: %s\n", timeStr, event.MonitorName, event.Message)
		}
		fmt.Println()
	}

	// 接口状态
	if len(state.InterfaceStates) > 0 {
		fmt.Println("【接口状态】")
		for name, ifaceState := range state.InterfaceStates {
			if !ifaceState.InitialCheckDone {
				fmt.Printf("  %s: 初始检测中...\n", name)
				continue
			}

			// 判断状态
			var statusStr string
			if ifaceState.PacketLoss >= 100.0 {
				statusStr = "DOWN"
			} else {
				statusStr = "UP"
			}

			// 显示详细信息：延迟、丢包、评分
			fmt.Printf("  %s: %s [延迟: %.1fms, 丢包: %.0f%%, Cost: %d, 评分: %.1f]\n",
				name, statusStr, ifaceState.Latency, ifaceState.PacketLoss,
				ifaceState.Cost, ifaceState.FinalScore)
		}
	}

	return nil
}
