package failover

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"trueword_node/pkg/network"
)

// CheckResult 检查结果
type CheckResult struct {
	Interface  string  // 接口名
	TargetIP   string  // 使用的目标IP
	Success    bool    // 是否成功（至少1个包通）
	Latency    float64 // 平均延迟（ms）
	PacketLoss float64 // 丢包率（%）
}

// HealthChecker 健康检查器
type HealthChecker struct {
	logger     *Logger
	globalLock sync.Mutex // 全局锁，保证测试路由的原子性
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(logger *Logger) *HealthChecker {
	return &HealthChecker{
		logger:     logger,
		globalLock: sync.Mutex{},
	}
}

// CheckInterface 检测接口健康状态
// 返回: 检查结果（包含延迟、丢包率）
func (hc *HealthChecker) CheckInterface(iface string, targets []string) *CheckResult {
	// 顺序尝试每个目标IP
	for i, target := range targets {
		result := hc.quickPing(iface, target)

		if result.Success {
			if i > 0 {
				// 使用了备选目标
				hc.logger.Debug("检测 %s → %s: 成功 (使用备选目标 #%d) [延迟: %.1fms, 丢包: %.0f%%]",
					iface, target, i+1, result.Latency, result.PacketLoss)
			} else {
				hc.logger.Debug("检测 %s → %s: 成功 [延迟: %.1fms, 丢包: %.0f%%]",
					iface, target, result.Latency, result.PacketLoss)
			}
			return result
		} else {
			hc.logger.Debug("检测 %s → %s: 失败", iface, target)
		}
	}

	// 所有目标都失败
	hc.logger.Debug("检测 %s: 失败 (所有目标不可达)", iface)
	return &CheckResult{
		Interface:  iface,
		Success:    false,
		Latency:    0,
		PacketLoss: 100.0,
	}
}

// quickPing 快速ping检测（3个包，约300ms）
// 返回: 检查结果（延迟、丢包率）
func (hc *HealthChecker) quickPing(iface, target string) *CheckResult {
	result := &CheckResult{
		Interface:  iface,
		TargetIP:   target,
		Success:    false,
		Latency:    0,
		PacketLoss: 100.0,
	}

	// 全局锁：因为 pref 5 必须全局唯一，所有检查必须串行化
	hc.globalLock.Lock()
	defer hc.globalLock.Unlock()

	// 先清理所有 pref 5 的残留规则（防止之前崩溃留下的）
	hc.cleanupPref5Rules()

	// 添加临时路由规则和路由
	table := hc.getRouteTable(iface)
	if err := hc.addTestRoute(target, iface, table); err != nil {
		hc.logger.Debug("添加临时路由失败: %v", err)
		return result
	}

	// 确保删除（即使 panic 也要删除）
	defer hc.removeTestRoute(target, iface, table)

	// 执行快速ping: 3个包，间隔0.1秒，超时1秒
	cmd := exec.Command("ping",
		"-c", "3",   // 发送3个包
		"-i", "0.1", // 间隔0.1秒
		"-W", "1",   // 超时1秒
		target,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// ping命令返回非0，可能部分失败或全部失败
		// 继续解析输出，看是否有包成功
	}

	// 解析结果：提取延迟和丢包率
	hc.parsePingResult(output, result)

	return result
}

// cleanupPref5Rules 清理所有 pref 5 的规则（防止残留）
func (hc *HealthChecker) cleanupPref5Rules() {
	// 循环删除所有 pref 5 的规则，最多尝试10次
	for i := 0; i < 10; i++ {
		// 不指定具体的 to，删除任何 pref 5 的规则
		cmd := exec.Command("ip", "rule", "del", "pref", "5")
		err := cmd.Run()
		if err != nil {
			// 删除失败，说明已经没有 pref 5 的规则了
			break
		}
		hc.logger.Debug("清理残留规则: pref 5 (第%d次)", i+1)
	}
}

// addTestRoute 添加临时测试路由规则和路由
func (hc *HealthChecker) addTestRoute(target, iface, table string) error {
	// 步骤1: 添加路由规则 - ip rule add to <target> lookup <table> pref 5
	cmdRule := exec.Command("ip", "rule", "add", "to", target, "lookup", table, "pref", "5")
	if output, err := cmdRule.CombinedOutput(); err != nil {
		return fmt.Errorf("添加路由规则失败: %v, output: %s", err, output)
	}

	// 步骤2: 在指定路由表中添加到目标的路由
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

	// 先删除可能存在的残留路由（忽略错误）
	exec.Command("ip", "route", "del", target, "table", table).Run()

	// 添加路由
	var cmdRoute *exec.Cmd
	if isPhysical && gateway != "" {
		// 物理接口：通过网关路由
		// ip route add <target> via <gateway> dev <iface> table <table>
		cmdRoute = exec.Command("ip", "route", "add", target, "via", gateway, "dev", iface, "table", table)
	} else {
		// 隧道或无网关的P2P连接：直接通过设备路由
		// ip route add <target> dev <iface> table <table>
		cmdRoute = exec.Command("ip", "route", "add", target, "dev", iface, "table", table)
	}

	if output, err := cmdRoute.CombinedOutput(); err != nil {
		// 如果路由添加失败，需要清理已添加的规则
		exec.Command("ip", "rule", "del", "to", target, "pref", "5").Run()
		return fmt.Errorf("添加路由失败: %v, output: %s", err, output)
	}

	return nil
}

// removeTestRoute 删除临时测试路由规则和路由
func (hc *HealthChecker) removeTestRoute(target, iface, table string) {
	// 步骤1: 删除路由 - ip route del <target> table <table>
	cmdRoute := exec.Command("ip", "route", "del", target, "table", table)
	cmdRoute.Run() // 忽略错误

	// 步骤2: 删除路由规则 - ip rule del to <target> pref 5
	cmdRule := exec.Command("ip", "rule", "del", "to", target, "pref", "5")
	cmdRule.Run() // 忽略错误
}

// getRouteTable 获取接口对应的路由表
func (hc *HealthChecker) getRouteTable(iface string) string {
	// 检查是否是隧道接口
	if strings.HasPrefix(iface, "tun") || strings.HasPrefix(iface, "wg") {
		return "80" // 虚拟IP路由表
	}
	return "main" // 物理接口使用主路由表
}

// parsePingResult 解析ping输出，提取延迟和丢包率
func (hc *HealthChecker) parsePingResult(output []byte, result *CheckResult) {
	outputStr := string(output)

	// 示例输出：
	// 3 packets transmitted, 2 received, 33% packet loss, time 200ms
	// rtt min/avg/max/mdev = 10.5/15.3/20.1/4.8 ms

	// 1. 提取丢包率
	// 匹配 "33% packet loss" 或 "33.3% packet loss"
	lossRe := regexp.MustCompile(`(\d+(?:\.\d+)?)% packet loss`)
	lossMatches := lossRe.FindStringSubmatch(outputStr)
	if len(lossMatches) >= 2 {
		if loss, err := strconv.ParseFloat(lossMatches[1], 64); err == nil {
			result.PacketLoss = loss
		}
	}

	// 2. 提取平均延迟
	// 匹配 "rtt min/avg/max/mdev = 10.5/15.3/20.1/4.8 ms"
	latencyRe := regexp.MustCompile(`rtt min/avg/max/mdev = [\d.]+/([\d.]+)/[\d.]+/[\d.]+ ms`)
	latencyMatches := latencyRe.FindStringSubmatch(outputStr)
	if len(latencyMatches) >= 2 {
		if latency, err := strconv.ParseFloat(latencyMatches[1], 64); err == nil {
			result.Latency = latency
		}
	}

	// 3. 判断成功：丢包率 < 100%（至少1个包通）
	if result.PacketLoss < 100.0 {
		result.Success = true
	}
}
