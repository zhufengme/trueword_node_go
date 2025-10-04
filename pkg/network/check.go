package network

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"trueword_node/pkg/config"
)

const (
	// 临时策略路由优先级（用于测试）
	TestPolicyPriority = 5
	// 检查结果缓存文件
	CheckResultFile = "/var/lib/trueword_node/check_results.json"
)

// CheckResult 检查结果
type CheckResult struct {
	TunnelName   string    `json:"tunnel_name"`
	Status       string    `json:"status"`      // "UP", "DOWN", "IDLE"
	Latency      float64   `json:"latency"`     // 延迟(毫秒)
	PacketLoss   float64   `json:"packet_loss"` // 丢包率(百分比)
	TargetIP     string    `json:"target_ip"`   // 成功响应的目标IP
	CheckTime    time.Time `json:"check_time"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// AllCheckResults 所有隧道的检查结果
type AllCheckResults struct {
	Results    map[string]*CheckResult `json:"results"` // key: tunnel_name
	LastUpdate time.Time               `json:"last_update"`
}

// ping解析正则
var (
	// ping统计行: "5 packets transmitted, 5 received, 0% packet loss, time 4005ms"
	pingStatsRegex = regexp.MustCompile(`(\d+) packets transmitted, (\d+) received, (\d+)% packet loss`)
	// RTT行: "rtt min/avg/max/mdev = 10.123/15.456/20.789/5.012 ms"
	rttRegex = regexp.MustCompile(`rtt min/avg/max/mdev = ([\d.]+)/([\d.]+)/([\d.]+)/([\d.]+) ms`)
)

// parsePingOutput 解析ping输出
func parsePingOutput(output string) (avgLatency float64, packetLoss float64, err error) {
	// 解析丢包率
	statsMatch := pingStatsRegex.FindStringSubmatch(output)
	if statsMatch == nil {
		return 0, 0, fmt.Errorf("无法解析ping统计信息")
	}

	transmitted, _ := strconv.Atoi(statsMatch[1])
	received, _ := strconv.Atoi(statsMatch[2])
	loss, _ := strconv.Atoi(statsMatch[3])

	if transmitted == 0 {
		return 0, 100, nil
	}

	packetLoss = float64(loss)

	// 如果全部丢包，不解析RTT
	if received == 0 {
		return 0, packetLoss, nil
	}

	// 解析平均延迟
	rttMatch := rttRegex.FindStringSubmatch(output)
	if rttMatch == nil {
		return 0, packetLoss, fmt.Errorf("无法解析RTT信息")
	}

	avgLatency, _ = strconv.ParseFloat(rttMatch[2], 64)

	return avgLatency, packetLoss, nil
}

// addTestPolicyRoute 添加临时测试策略路由
func addTestPolicyRoute(targetIP, exitInterface string) error {
	// 添加策略路由规则: ip rule add to <targetIP> lookup <table> pref <prio>
	// 使用一个临时路由表（例如表5）
	tableID := TestPolicyPriority

	// 添加路由规则
	cmd := exec.Command("ip", "rule", "add", "to", targetIP, "lookup", strconv.Itoa(tableID), "pref", strconv.Itoa(TestPolicyPriority))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("添加测试路由规则失败: %w", err)
	}

	// 检查是否是物理接口（通过查找配置）
	ifaceConfig, err := LoadInterfaceConfig()
	var gateway string
	isPhysical := false

	if err == nil {
		for _, iface := range ifaceConfig.Interfaces {
			if iface.Name == exitInterface {
				isPhysical = true
				gateway = iface.Gateway
				break
			}
		}
	}

	// 添加路由
	var routeCmd *exec.Cmd
	if isPhysical && gateway != "" {
		// 物理接口：通过网关路由
		// ip route add <targetIP> via <gateway> dev <interface> table <tableID>
		routeCmd = exec.Command("ip", "route", "add", targetIP, "via", gateway, "dev", exitInterface, "table", strconv.Itoa(tableID))
	} else {
		// 隧道或无网关的P2P连接：直接通过设备路由
		// ip route add <targetIP> dev <exitInterface> table <tableID>
		routeCmd = exec.Command("ip", "route", "add", targetIP, "dev", exitInterface, "table", strconv.Itoa(tableID))
	}

	if err := routeCmd.Run(); err != nil {
		// 清理规则
		exec.Command("ip", "rule", "del", "pref", strconv.Itoa(TestPolicyPriority)).Run()
		return fmt.Errorf("添加测试路由失败: %w", err)
	}

	return nil
}

// removeTestPolicyRoute 删除临时测试策略路由
func removeTestPolicyRoute(targetIP string) {
	tableID := TestPolicyPriority

	// 删除路由
	exec.Command("ip", "route", "del", targetIP, "table", strconv.Itoa(tableID)).Run()

	// 删除规则
	exec.Command("ip", "rule", "del", "pref", strconv.Itoa(TestPolicyPriority)).Run()
}

// pingWithRoute 使用指定出口进行ping测试
func pingWithRoute(targetIP, exitInterface string, count int, timeout int) (avgLatency float64, packetLoss float64, err error) {
	// 添加临时策略路由
	if err := addTestPolicyRoute(targetIP, exitInterface); err != nil {
		return 0, 0, err
	}
	defer removeTestPolicyRoute(targetIP)

	// 执行ping
	cmd := exec.Command("ping", "-c", strconv.Itoa(count), "-W", strconv.Itoa(timeout), targetIP)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// ping失败，尝试解析输出
		if len(output) > 0 {
			_, loss, parseErr := parsePingOutput(string(output))
			if parseErr == nil && loss == 100 {
				// 全部丢包
				return 0, 100, nil
			}
		}
		return 0, 0, fmt.Errorf("ping失败: %w", err)
	}

	// 解析输出
	return parsePingOutput(string(output))
}

// CheckTunnel 检查单个隧道的连通性
// targetIPs: 目标IP列表，按顺序测试，成功即返回
func CheckTunnel(tunnelName string, targetIPs []string) *CheckResult {
	result := &CheckResult{
		TunnelName: tunnelName,
		Status:     "IDLE",
		CheckTime:  time.Now(),
	}

	// 加载隧道配置
	_, err := LoadTunnelConfig(tunnelName)
	if err != nil {
		result.Status = "IDLE"
		result.ErrorMessage = fmt.Sprintf("加载隧道配置失败: %v", err)
		return result
	}

	// 检查隧道接口是否存在
	if _, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", tunnelName)); os.IsNotExist(err) {
		result.Status = "IDLE"
		result.ErrorMessage = "隧道未启动"
		return result
	}

	// 隧道已启动，进行连通性测试
	// 逐个测试目标IP，找到第一个能通的IP
	var lastResult struct {
		targetIP   string
		latency    float64
		packetLoss float64
		err        error
	}

	for _, targetIP := range targetIPs {
		avgLatency, packetLoss, err := pingWithRoute(targetIP, tunnelName, 20, 2)

		// 记录最后一次测试结果
		lastResult.targetIP = targetIP
		lastResult.latency = avgLatency
		lastResult.packetLoss = packetLoss
		lastResult.err = err

		if err != nil {
			// ping命令执行失败，尝试下一个IP
			continue
		}

		// ping命令成功执行
		result.TargetIP = targetIP
		result.Latency = avgLatency
		result.PacketLoss = packetLoss

		// 如果丢包率 < 100%，说明这个IP是通的，直接返回UP
		if packetLoss < 100 {
			result.Status = "UP"
			return result
		}

		// 丢包率 = 100%，继续尝试下一个IP
		// 但保留这个结果作为备选
	}

	// 所有IP都测试完了
	if lastResult.err != nil {
		// 所有IP都执行失败
		result.Status = "DOWN"
		result.ErrorMessage = "所有目标IP均无法执行ping测试"
	} else {
		// 至少有一个IP能执行ping，但都是100%丢包
		result.Status = "DOWN"
		result.TargetIP = lastResult.targetIP
		result.Latency = lastResult.latency
		result.PacketLoss = lastResult.packetLoss
	}

	return result
}

// LoadCheckResults 加载检查结果缓存
func LoadCheckResults() (*AllCheckResults, error) {
	data, err := os.ReadFile(CheckResultFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空结果
			return &AllCheckResults{
				Results: make(map[string]*CheckResult),
			}, nil
		}
		return nil, err
	}

	var results AllCheckResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	if results.Results == nil {
		results.Results = make(map[string]*CheckResult)
	}

	return &results, nil
}

// SaveCheckResults 保存检查结果缓存
func SaveCheckResults(results *AllCheckResults) error {
	results.LastUpdate = time.Now()

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll("/var/lib/trueword_node", 0755); err != nil {
		return err
	}

	return os.WriteFile(CheckResultFile, data, 0644)
}

// CheckInterface 检查单个物理接口的连通性
func CheckInterface(interfaceName string, targetIPs []string) *CheckResult {
	result := &CheckResult{
		TunnelName: interfaceName,
		Status:     "IDLE",
		CheckTime:  time.Now(),
	}

	// 检查接口是否存在
	if _, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", interfaceName)); os.IsNotExist(err) {
		result.Status = "IDLE"
		result.ErrorMessage = "接口不存在"
		return result
	}

	// 物理接口总是"启动"状态，直接进行连通性测试
	// 逐个测试目标IP，找到第一个能通的IP
	var lastResult struct {
		targetIP   string
		latency    float64
		packetLoss float64
		err        error
	}

	for _, targetIP := range targetIPs {
		avgLatency, packetLoss, err := pingWithRoute(targetIP, interfaceName, 20, 2)

		// 记录最后一次测试结果
		lastResult.targetIP = targetIP
		lastResult.latency = avgLatency
		lastResult.packetLoss = packetLoss
		lastResult.err = err

		if err != nil {
			// ping命令执行失败，尝试下一个IP
			continue
		}

		// ping命令成功执行
		result.TargetIP = targetIP
		result.Latency = avgLatency
		result.PacketLoss = packetLoss

		// 如果丢包率 < 100%，说明这个IP是通的，直接返回UP
		if packetLoss < 100 {
			result.Status = "UP"
			return result
		}

		// 丢包率 = 100%，继续尝试下一个IP
		// 但保留这个结果作为备选
	}

	// 所有IP都测试完了
	if lastResult.err != nil {
		// 所有IP都执行失败
		result.Status = "DOWN"
		result.ErrorMessage = "所有目标IP均无法执行ping测试"
	} else {
		// 至少有一个IP能执行ping，但都是100%丢包
		result.Status = "DOWN"
		result.TargetIP = lastResult.targetIP
		result.Latency = lastResult.latency
		result.PacketLoss = lastResult.packetLoss
	}

	return result
}

// CheckAllTunnels 检查所有出口（包括物理接口和隧道）
func CheckAllTunnels(targetIPs []string) error {
	fmt.Println("开始检查所有出口...")
	fmt.Println()

	// 加载或创建检查结果
	allResults, err := LoadCheckResults()
	if err != nil {
		return fmt.Errorf("加载检查结果失败: %w", err)
	}

	totalCount := 0

	// 1. 检查物理接口
	ifaceConfig, err := LoadInterfaceConfig()
	if err == nil && len(ifaceConfig.Interfaces) > 0 {
		fmt.Println("【物理接口】")
		for _, iface := range ifaceConfig.Interfaces {
			fmt.Printf("检查接口: %s ... ", iface.Name)

			result := CheckInterface(iface.Name, targetIPs)
			allResults.Results[iface.Name] = result
			totalCount++

			// 输出结果
			switch result.Status {
			case "UP":
				fmt.Printf("✓ UP (延迟: %.2fms, 丢包: %.0f%%)\n", result.Latency, result.PacketLoss)
			case "DOWN":
				fmt.Printf("✗ DOWN (延迟: %.2fms, 丢包: %.0f%%)\n", result.Latency, result.PacketLoss)
			case "IDLE":
				fmt.Printf("- IDLE (未启动)\n")
			default:
				fmt.Printf("? 未知\n")
			}
		}
		fmt.Println()
	}

	// 2. 检查隧道
	tunnelDir := config.ConfigDir + "/tunnels"
	entries, err := os.ReadDir(tunnelDir)
	if err == nil && len(entries) > 0 {
		hasValidTunnel := false
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".yaml") {
				hasValidTunnel = true
				break
			}
		}

		if hasValidTunnel {
			fmt.Println("【隧道】")
			for _, entry := range entries {
				if !strings.HasSuffix(entry.Name(), ".yaml") {
					continue
				}

				tunnelName := strings.TrimSuffix(entry.Name(), ".yaml")
				totalCount++

				fmt.Printf("检查隧道: %s ... ", tunnelName)

				result := CheckTunnel(tunnelName, targetIPs)
				allResults.Results[tunnelName] = result

				// 输出结果
				switch result.Status {
				case "UP":
					fmt.Printf("✓ UP (延迟: %.2fms, 丢包: %.0f%%)\n", result.Latency, result.PacketLoss)
				case "DOWN":
					fmt.Printf("✗ DOWN (延迟: %.2fms, 丢包: %.0f%%)\n", result.Latency, result.PacketLoss)
				case "IDLE":
					fmt.Printf("- IDLE (未启动)\n")
				default:
					fmt.Printf("? 未知\n")
				}
			}
		}
	}

	if totalCount == 0 {
		fmt.Println("未找到任何出口")
		return nil
	}

	// 保存结果
	if err := SaveCheckResults(allResults); err != nil {
		return fmt.Errorf("保存检查结果失败: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ 检查完成，共检查 %d 个出口\n", totalCount)

	return nil
}
