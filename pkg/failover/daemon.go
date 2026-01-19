package failover

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"trueword_node/pkg/network"
	"trueword_node/pkg/routing"
)

// FailoverDaemon 故障转移守护进程
type FailoverDaemon struct {
	config               *FailoverConfig
	configFile           string
	logger               *Logger
	healthChecker        *HealthChecker
	stateManager         *StateManager
	failoverMutex        sync.Mutex
	stopChan             chan struct{}
	tickers              map[string]*time.Ticker
	currentExits         map[string]string // monitor_name -> current_exit
	confirmationCounters map[string]int    // monitor_name -> 当前确认次数
}

// NewFailoverDaemon 创建守护进程
func NewFailoverDaemon(configFile string, debugMode bool) (*FailoverDaemon, error) {
	// 加载配置
	config, err := LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("加载配置文件失败: %v", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 创建日志管理器
	logger, err := NewLogger(config.Daemon.LogFile, debugMode)
	if err != nil {
		return nil, fmt.Errorf("创建日志管理器失败: %v", err)
	}

	// 创建健康检查器
	healthChecker := NewHealthChecker(logger)

	// 创建状态管理器
	stateManager := NewStateManager()

	daemon := &FailoverDaemon{
		config:               config,
		configFile:           configFile,
		logger:               logger,
		healthChecker:        healthChecker,
		stateManager:         stateManager,
		stopChan:             make(chan struct{}),
		tickers:              make(map[string]*time.Ticker),
		currentExits:         make(map[string]string),
		confirmationCounters: make(map[string]int),
	}

	return daemon, nil
}

// Run 运行守护进程
func (d *FailoverDaemon) Run() error {
	d.logger.Info("故障转移守护进程启动")
	d.logger.Info("配置文件: %s", d.configFile)
	d.logger.Info("监控任务: %d 个", len(d.config.Monitors))

	// 初始化当前出口
	for i := range d.config.Monitors {
		monitor := &d.config.Monitors[i]
		currentExit, err := d.getCurrentExit(monitor)
		if err != nil {
			d.logger.Warn("获取监控任务 %s 当前出口失败: %v", monitor.Name, err)
		} else {
			d.currentExits[monitor.Name] = currentExit
			d.logger.Info("监控任务 %s 当前出口: %s", monitor.Name, currentExit)
		}
	}

	// 为每个monitor启动独立的检测循环
	for i := range d.config.Monitors {
		monitor := &d.config.Monitors[i]
		d.startMonitor(monitor)
	}

	// 注册信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

	// 主循环
	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				// 重载配置
				d.logger.Info("收到 SIGHUP 信号，重新加载配置...")
				if err := d.reloadConfig(); err != nil {
					d.logger.Error("重载配置失败: %v", err)
				} else {
					d.logger.Info("配置重载成功")
				}

			case syscall.SIGTERM, syscall.SIGINT:
				// 优雅退出
				d.logger.Info("收到退出信号，正在关闭...")
				d.shutdown()
				return nil
			}

		case <-d.stopChan:
			// 停止信号
			return nil
		}
	}
}

// startMonitor 启动监控任务
func (d *FailoverDaemon) startMonitor(monitor *MonitorConfig) {
	interval := monitor.GetCheckInterval(d.config.Daemon.CheckIntervalMs)
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	d.tickers[monitor.Name] = ticker

	d.logger.Info("启动监控任务: %s (间隔: %dms)", monitor.Name, interval)

	go func(m *MonitorConfig, t *time.Ticker) {
		for range t.C {
			d.checkMonitor(m)
		}
	}(monitor, ticker)
}

// stopMonitor 停止监控任务
func (d *FailoverDaemon) stopMonitor(name string) {
	if ticker, exists := d.tickers[name]; exists {
		ticker.Stop()
		delete(d.tickers, name)
	}
}

// getCurrentExit 获取当前出口
func (d *FailoverDaemon) getCurrentExit(monitor *MonitorConfig) (string, error) {
	if monitor.Type == "default_route" {
		// 默认路由：从系统实际读取
		currentExit, err := d.getActualDefaultRoute(monitor)
		if err != nil {
			d.logger.Warn("无法从系统读取默认路由: %v，使用候选列表第一个作为初始值", err)
			// 降级：使用候选列表第一个
			if len(monitor.CandidateExits) > 0 {
				return monitor.CandidateExits[0], nil
			}
			return "", fmt.Errorf("候选出口列表为空")
		}
		d.logger.Debug("从系统读取到当前默认路由出口: %s", currentExit)
		return currentExit, nil
	} else {
		// 策略组：从配置文件加载
		pm := routing.NewPolicyManager()
		if err := pm.LoadGroup(monitor.Target); err != nil {
			return "", err
		}
		group := pm.GetGroup(monitor.Target)
		if group == nil {
			return "", fmt.Errorf("策略组不存在")
		}
		return group.Exit, nil
	}
}

// getActualDefaultRoute 从系统读取实际的默认路由出口
func (d *FailoverDaemon) getActualDefaultRoute(monitor *MonitorConfig) (string, error) {
	// 读取优先级 900 的路由规则
	cmd := exec.Command("sh", "-c", "ip rule show pref 900 | head -1")
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return "", fmt.Errorf("未找到默认路由规则（pref 900）")
	}

	// 解析规则，提取路由表编号（应该是 900）
	// 格式：900:	from all lookup 900
	tableID := 900

	// 读取路由表 900 中的默认路由
	cmd = exec.Command("sh", "-c", fmt.Sprintf("ip route show table %d | grep '^default'", tableID))
	output, err = cmd.Output()
	if err != nil || len(output) == 0 {
		return "", fmt.Errorf("路由表 %d 中未找到默认路由", tableID)
	}

	// 检查是否有多条默认路由，如果有则自动清理
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 1 {
		d.logger.Warn("⚠ 检测到路由表 %d 中存在多条默认路由（%d 条），自动清理多余路由", tableID, len(lines))
		d.logger.Warn("保留第一条: %s", strings.TrimSpace(lines[0]))

		// 自动清理多余的默认路由（保留第一条）
		for i := 1; i < len(lines); i++ {
			routeLine := strings.TrimSpace(lines[i])
			d.logger.Warn("删除第 %d 条: %s", i+1, routeLine)

			// 提取完整路由信息用于删除命令
			routeParts := strings.Fields(routeLine)
			if len(routeParts) >= 2 {
				// 构建删除命令：ip route del <路由参数> table 900
				delCmd := fmt.Sprintf("ip route del %s table %d", strings.Join(routeParts[1:], " "), tableID)
				cmd := exec.Command("sh", "-c", delCmd)
				if err := cmd.Run(); err != nil {
					d.logger.Error("删除多余默认路由失败: %v (命令: %s)", err, delCmd)
				} else {
					d.logger.Info("✓ 已删除多余默认路由: %s", routeLine)
				}
			}
		}
	}

	// 解析默认路由，提取出口接口（使用第一条）
	// 格式1: default dev tun_hk
	// 格式2: default via 192.168.1.1 dev eth0
	line := strings.TrimSpace(lines[0])
	d.logger.Debug("读取到默认路由: %s", line)
	parts := strings.Fields(line)

	var exitIface string
	for i, part := range parts {
		if part == "dev" && i+1 < len(parts) {
			exitIface = parts[i+1]
			break
		}
	}

	if exitIface == "" {
		return "", fmt.Errorf("无法解析默认路由出口接口")
	}

	d.logger.Debug("解析得到出口接口: %s", exitIface)

	// 验证出口是否在候选列表中
	for _, candidate := range monitor.CandidateExits {
		if candidate == exitIface {
			return exitIface, nil
		}
	}

	// 如果不在候选列表中，警告但仍返回
	d.logger.Warn("当前默认路由出口 %s 不在候选列表中: %v", exitIface, monitor.CandidateExits)
	return exitIface, nil
}

// checkMonitor 检查监控任务（基于评分机制）
func (d *FailoverDaemon) checkMonitor(monitor *MonitorConfig) {
	d.logger.Debug("【监控任务】%s 开始检查 (候选: %v)", monitor.Name, monitor.CandidateExits)

	// 获取检测间隔（用于自适应包数量）
	checkIntervalMs := monitor.GetCheckInterval(d.config.Daemon.CheckIntervalMs)

	// 获取检测模式
	checkMode := monitor.GetCheckMode(d.config.Daemon.CheckMode)

	// 获取检测目标（ping 模式用 check_targets，dns 模式用 dns_servers）
	var targets []string
	var dnsDomain string
	if checkMode == "dns" {
		targets = monitor.DNSServers
		dnsDomain = monitor.GetDNSQueryDomain(d.config.Daemon.DNSQueryDomain)
	} else {
		targets = monitor.CheckTargets
		dnsDomain = "" // ping 模式不使用
	}

	// 检查所有候选出口
	for _, exit := range monitor.CandidateExits {
		// 执行健康检查（支持 ping 和 dns 模式）
		checkResult := d.healthChecker.CheckInterface(exit, checkMode, targets, dnsDomain, checkIntervalMs)

		// 获取成本
		cost := d.getExitCost(exit)

		// 更新状态（计算评分）
		isFirstCheck := d.stateManager.UpdateState(exit, checkResult, cost)

		if isFirstCheck {
			state := d.stateManager.GetState(exit)
			d.logger.Debug("  接口 %s: 初始检测完成 [延迟: %.1fms, 丢包: %.0f%%, Cost: %d, 评分: %.1f]",
				exit, state.Latency, state.PacketLoss, state.Cost, state.FinalScore)
		}
	}

	// 等待所有接口完成初始检测
	if !d.stateManager.AllInitialChecksDone(monitor.CandidateExits) {
		d.logger.Debug("监控任务 %s 还在初始检测阶段，不触发故障转移", monitor.Name)
		// 保存状态
		if err := d.stateManager.SaveState(d.currentExits); err != nil {
			d.logger.Error("保存状态失败: %v", err)
		}
		return
	}

	// 评估是否需要执行 failover
	d.evaluateFailover(monitor)

	// 保存状态
	if err := d.stateManager.SaveState(d.currentExits); err != nil {
		d.logger.Error("保存状态失败: %v", err)
	}
}

// evaluateFailover 评估是否需要故障转移（基于评分）
func (d *FailoverDaemon) evaluateFailover(monitor *MonitorConfig) {
	// 获取当前出口
	var currentExit string
	var err error

	// 对于默认路由，每次都从系统实际读取（防止外部修改导致状态不一致）
	if monitor.Type == "default_route" {
		currentExit, err = d.getActualDefaultRoute(monitor)
		if err != nil {
			d.logger.Warn("无法从系统读取默认路由: %v，使用缓存值", err)
			// 降级：使用缓存值
			if cachedExit, exists := d.currentExits[monitor.Name]; exists {
				currentExit = cachedExit
			} else {
				d.logger.Error("获取监控任务 %s 当前出口失败: 无缓存且系统读取失败", monitor.Name)
				return
			}
		} else {
			// 检查是否与缓存不一致
			if cachedExit, exists := d.currentExits[monitor.Name]; exists {
				// 缓存存在时才检查
				if cachedExit != currentExit {
					d.logger.Warn("检测到默认路由已被外部修改: %s → %s", cachedExit, currentExit)
				} else {
					d.logger.Debug("默认路由未变化: %s", currentExit)
				}
			} else {
				// 首次读取，不报警告
				d.logger.Debug("首次读取默认路由: %s", currentExit)
			}
			// 更新缓存
			d.currentExits[monitor.Name] = currentExit
		}
	} else {
		// 策略组：使用缓存值（策略组配置不会被外部修改）
		if cachedExit, exists := d.currentExits[monitor.Name]; exists {
			currentExit = cachedExit
		} else {
			// 第一次评估，从配置文件读取
			currentExit, err = d.getCurrentExit(monitor)
			if err != nil {
				d.logger.Error("获取监控任务 %s 当前出口失败: %v", monitor.Name, err)
				return
			}
			d.currentExits[monitor.Name] = currentExit
		}
	}

	// 获取所有候选出口的状态
	var bestExit string
	var bestScore float64 = -1
	var currentScore float64 = -1

	d.logger.Debug("【评分结果】监控任务: %s", monitor.Name)

	for _, exit := range monitor.CandidateExits {
		state := d.stateManager.GetState(exit)

		// 判断 UP/DOWN 状态
		status := "UP"
		if state.PacketLoss >= 100.0 {
			status = "DOWN"
		}

		d.logger.Debug("  %s: %s [延迟=%.1fms 丢包=%.0f%% Cost=%d 基础分=%.1f 最终分=%.1f]",
			exit, status, state.Latency, state.PacketLoss, state.Cost, state.BaseScore, state.FinalScore)

		// 记录当前出口的评分
		if exit == currentExit {
			currentScore = state.FinalScore
		}

		// 选择最佳出口
		if state.FinalScore > bestScore {
			bestScore = state.FinalScore
			bestExit = exit
		} else if state.FinalScore == bestScore && exit == currentExit {
			// 分数相同，优先保持当前出口（避免频繁切换）
			bestExit = currentExit
		}
	}

	if bestExit == "" {
		d.logger.Warn("监控任务 %s: 所有候选出口均不可用", monitor.Name)
		return
	}

	d.logger.Debug("【决策】当前出口: %s (评分: %.1f), 最佳出口: %s (评分: %.1f)",
		currentExit, currentScore, bestExit, bestScore)

	// 判断是否需要切换
	if bestExit != currentExit {
		scoreDiff := bestScore - currentScore
		scoreThreshold := monitor.GetScoreThreshold(d.config.Daemon.ScoreThreshold)

		// 检查评分差值是否超过阈值
		if scoreDiff < scoreThreshold {
			// 评分差值不足，重置确认计数器
			if d.confirmationCounters[monitor.Name] > 0 {
				d.logger.Info("【确认取消】评分差值不足，重置确认计数器 (之前: %d/%d)",
					d.confirmationCounters[monitor.Name],
					monitor.GetSwitchConfirmationCount(d.config.Daemon.SwitchConfirmationCount))
				d.confirmationCounters[monitor.Name] = 0
			} else {
				d.logger.Debug("【保持不变】评分提升 %.1f 未超过阈值 %.1f，不切换",
					scoreDiff, scoreThreshold)
			}
			return
		}

		// 需要切换：确认计数器 +1
		d.confirmationCounters[monitor.Name]++
		confirmationCount := monitor.GetSwitchConfirmationCount(d.config.Daemon.SwitchConfirmationCount)
		currentConfirmations := d.confirmationCounters[monitor.Name]

		d.logger.Info("【需要切换】监控任务 %s: %s (%.1f) → %s (%.1f), 评分提升: %.1f (阈值: %.1f)",
			monitor.Name, currentExit, currentScore, bestExit, bestScore, scoreDiff, scoreThreshold)

		// 检查是否达到确认次数
		if currentConfirmations >= confirmationCount {
			// 确认完成，执行切换
			d.logger.Info("【确认完成】连续 %d 次确认通过，执行切换", currentConfirmations)
			d.confirmationCounters[monitor.Name] = 0 // 重置计数器
			d.executeFailover(monitor, currentExit, bestExit, currentScore, bestScore)
		} else {
			// 还需要更多确认
			remaining := confirmationCount - currentConfirmations
			d.logger.Info("【确认中】切换确认进度: %d/%d (还需 %d 次确认)",
				currentConfirmations, confirmationCount, remaining)
		}
	} else {
		// 当前出口仍是最佳出口，重置确认计数器
		if d.confirmationCounters[monitor.Name] > 0 {
			d.logger.Info("【确认取消】当前出口恢复为最佳，重置确认计数器 (之前: %d/%d)",
				d.confirmationCounters[monitor.Name],
				monitor.GetSwitchConfirmationCount(d.config.Daemon.SwitchConfirmationCount))
			d.confirmationCounters[monitor.Name] = 0
		} else {
			d.logger.Debug("【保持不变】监控任务 %s: %s 仍是最佳出口 (评分: %.1f)",
				monitor.Name, currentExit, bestScore)
		}
	}
}

// executeFailover 执行故障转移
func (d *FailoverDaemon) executeFailover(monitor *MonitorConfig, oldExit, newExit string, oldScore, newScore float64) {
	// 加锁，确保同一时间只有一个故障转移在执行
	d.failoverMutex.Lock()
	defer d.failoverMutex.Unlock()

	message := fmt.Sprintf("故障转移: %s → %s (评分: %.1f → %.1f)", oldExit, newExit, oldScore, newScore)
	d.logger.Info("【执行】%s", message)

	// 构造模拟的检查结果（供 routing.SelectBestExit 使用）
	checkResults := &network.AllCheckResults{
		Results: make(map[string]*network.CheckResult),
	}

	for _, exit := range monitor.CandidateExits {
		state := d.stateManager.GetState(exit)
		checkResults.Results[exit] = &network.CheckResult{
			TunnelName: exit,
			Status:     map[bool]string{true: "UP", false: "DOWN"}[state.PacketLoss < 100.0],
			Latency:    state.Latency,
			PacketLoss: state.PacketLoss,
			TargetIP:   state.LastTarget,
		}
	}

	// 调用现有的Failover逻辑（静默模式）
	var err error
	if monitor.Type == "default_route" {
		// 默认路由故障转移
		err = d.failoverDefaultRoute(monitor, checkResults)
	} else {
		// 策略组故障转移
		err = d.failoverPolicyGroup(monitor, checkResults)
	}

	if err != nil {
		message := fmt.Sprintf("故障转移失败: %v", err)
		d.logger.Error(message)
		d.stateManager.RecordEvent(monitor.Name, "failover", message)
	} else {
		d.logger.Info("【完成】故障转移成功")
		d.currentExits[monitor.Name] = newExit
		d.stateManager.RecordEvent(monitor.Name, "failover", message)
	}
}

// getExitCost 获取出口成本
func (d *FailoverDaemon) getExitCost(exitName string) int {
	// 先尝试作为隧道加载
	tunnelConfig, err := network.LoadTunnelConfig(exitName)
	if err == nil && tunnelConfig != nil {
		return tunnelConfig.Cost
	}

	// 再尝试作为物理接口加载
	ifaceConfig, err := network.LoadInterfaceConfig()
	if err == nil {
		iface := ifaceConfig.GetInterfaceByName(exitName)
		if iface != nil {
			return iface.Cost
		}
	}

	// 默认成本为0
	return 0
}

// failoverDefaultRoute 执行默认路由故障转移（静默模式）
func (d *FailoverDaemon) failoverDefaultRoute(monitor *MonitorConfig, checkResults *network.AllCheckResults) error {
	// 选择最佳出口
	d.logger.Debug("【Failover】选择最佳出口...")
	bestExit, _, err := routing.SelectBestExit(monitor.CandidateExits, checkResults)
	if err != nil {
		d.logger.Error("选择最佳出口失败: %v", err)
		return err
	}

	d.logger.Debug("【Failover】最佳出口: %s", bestExit.Name)

	// 创建 PolicyManager 并设置默认出口
	pm := routing.NewPolicyManager()
	pm.SetDefaultExit(bestExit.Name)

	// 应用默认路由
	d.logger.Debug("【Failover】应用默认路由到 %s...", bestExit.Name)
	if err := pm.ApplyDefaultRouteOnly(); err != nil {
		d.logger.Error("应用默认路由失败: %v", err)
		return fmt.Errorf("应用默认路由失败: %v", err)
	}

	d.logger.Debug("【Failover】默认路由应用成功")
	return nil
}

// failoverPolicyGroup 执行策略组故障转移（静默模式）
func (d *FailoverDaemon) failoverPolicyGroup(monitor *MonitorConfig, checkResults *network.AllCheckResults) error {
	// 选择最佳出口
	d.logger.Debug("【Failover】选择最佳出口...")
	bestExit, _, err := routing.SelectBestExit(monitor.CandidateExits, checkResults)
	if err != nil {
		d.logger.Error("选择最佳出口失败: %v", err)
		return err
	}

	d.logger.Debug("【Failover】最佳出口: %s", bestExit.Name)

	// 加载策略组
	pm := routing.NewPolicyManager()
	if err := pm.LoadGroup(monitor.Target); err != nil {
		d.logger.Error("策略组 %s 不存在: %v", monitor.Target, err)
		return fmt.Errorf("策略组 %s 不存在", monitor.Target)
	}

	group := pm.GetGroup(monitor.Target)
	if group == nil {
		d.logger.Error("策略组 %s 不存在", monitor.Target)
		return fmt.Errorf("策略组 %s 不存在", monitor.Target)
	}

	// 检查是否需要切换
	if group.Exit == bestExit.Name {
		d.logger.Debug("【Failover】策略组 %s 当前出口已是 %s，无需切换", monitor.Target, bestExit.Name)
		return nil // 无需切换
	}

	// 更新出口
	d.logger.Debug("【Failover】更新策略组 %s 出口: %s → %s", monitor.Target, group.Exit, bestExit.Name)
	group.Exit = bestExit.Name

	// 保存配置
	d.logger.Debug("【Failover】保存策略组配置...")
	if err := pm.Save(); err != nil {
		d.logger.Error("保存配置失败: %v", err)
		return fmt.Errorf("保存配置失败: %v", err)
	}

	// 应用策略组
	d.logger.Debug("【Failover】应用策略组 %s...", monitor.Target)
	if err := pm.ApplyGroup(group); err != nil {
		d.logger.Error("应用策略失败: %v", err)
		return fmt.Errorf("应用策略失败: %v", err)
	}

	d.logger.Debug("【Failover】策略组 %s 应用成功", monitor.Target)
	return nil
}

// reloadConfig 重载配置
func (d *FailoverDaemon) reloadConfig() error {
	// 加载新配置
	newConfig, err := LoadConfig(d.configFile)
	if err != nil {
		d.logger.Error("配置文件格式错误，保持旧配置: %v", err)
		return err
	}

	// 验证新配置
	if err := newConfig.Validate(); err != nil {
		d.logger.Error("配置验证失败，保持旧配置: %v", err)
		return err
	}

	// 对比差异
	oldMonitors := d.config.GetMonitorNames()
	newMonitors := newConfig.GetMonitorNames()

	added := difference(newMonitors, oldMonitors)
	removed := difference(oldMonitors, newMonitors)
	updated := intersection(oldMonitors, newMonitors)

	// 停止已删除的monitor
	for _, name := range removed {
		d.stopMonitor(name)
		d.logger.Info("停止监控任务: %s", name)
	}

	// 启动新增的monitor
	for _, name := range added {
		monitor := newConfig.GetMonitor(name)
		d.startMonitor(monitor)
		d.logger.Info("启动监控任务: %s", name)
	}

	// 更新已修改的monitor（重启）
	for _, name := range updated {
		oldMonitor := d.config.GetMonitor(name)
		newMonitor := newConfig.GetMonitor(name)
		if !oldMonitor.Equals(newMonitor) {
			d.stopMonitor(name)
			d.startMonitor(newMonitor)
			d.logger.Info("更新监控任务: %s", name)
		}
	}

	// 更新全局配置
	d.config = newConfig

	// 重置所有状态（避免旧状态干扰）
	d.stateManager.ResetAllStates()

	return nil
}

// shutdown 优雅关闭
func (d *FailoverDaemon) shutdown() {
	d.logger.Info("正在停止所有监控任务...")

	// 停止所有ticker
	for name, ticker := range d.tickers {
		ticker.Stop()
		d.logger.Debug("停止监控任务: %s", name)
	}

	// 保存最终状态
	if err := d.stateManager.SaveState(d.currentExits); err != nil {
		d.logger.Error("保存状态失败: %v", err)
	}

	// 关闭日志
	d.logger.Info("守护进程已停止")
	d.logger.Close()

	close(d.stopChan)
}
