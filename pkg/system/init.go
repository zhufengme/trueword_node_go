package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"trueword_node/pkg/config"
	"trueword_node/pkg/network"
)

// 检查命令是否存在
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// 检查内核模块是否加载
func isModuleLoaded(module string) bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), module)
}

// 检查内核参数
func checkSysctl(param string) (string, error) {
	cmd := exec.Command("sysctl", "-n", param)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// 设置内核参数
func setSysctl(param, value string) error {
	cmd := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", param, value))
	return cmd.Run()
}

// 检查iptables规则是否存在
func iptablesRuleExists(table, chain, rule string) bool {
	cmd := exec.Command("iptables", "-t", table, "-C", chain)
	if rule != "" {
		ruleArgs := strings.Fields(rule)
		cmd.Args = append(cmd.Args, ruleArgs...)
	}
	return cmd.Run() == nil
}

// 添加iptables规则
func addIptablesRule(table, chain, rule string) error {
	cmd := exec.Command("iptables", "-t", table, "-A", chain)
	if rule != "" {
		ruleArgs := strings.Fields(rule)
		cmd.Args = append(cmd.Args, ruleArgs...)
	}
	return cmd.Run()
}

// CheckEnvironment 检查运行环境
func CheckEnvironment() error {
	fmt.Println("检查运行环境...")

	// 1. 检查是否是root
	if os.Geteuid() != 0 {
		return fmt.Errorf("必须以root权限运行")
	}
	fmt.Println("  ✓ Root权限")

	// 2. 检查必需的命令
	requiredCommands := []string{"ip", "iptables", "ping"}
	for _, cmd := range requiredCommands {
		if !commandExists(cmd) {
			return fmt.Errorf("缺少必需的命令: %s", cmd)
		}
	}
	fmt.Println("  ✓ 必需命令已安装")

	// 3. 检查内核模块
	requiredModules := []string{"ip_gre", "xfrm4_tunnel", "esp4"}
	for _, module := range requiredModules {
		if !isModuleLoaded(module) {
			fmt.Printf("  ⚠ 内核模块 %s 未加载，尝试加载...\n", module)
			cmd := exec.Command("modprobe", module)
			if err := cmd.Run(); err != nil {
				fmt.Printf("    警告: 无法加载模块 %s: %v\n", module, err)
			}
		}
	}
	fmt.Println("  ✓ 内核模块")

	// 4. 检查IP转发
	ipForward, err := checkSysctl("net.ipv4.ip_forward")
	if err != nil {
		return fmt.Errorf("无法检查IP转发状态: %w", err)
	}
	if ipForward != "1" {
		fmt.Println("  ⚠ IP转发未启用")
		return fmt.Errorf("IP转发未启用，请先运行初始化命令")
	}
	fmt.Println("  ✓ IP转发已启用")

	// 5. 检查iptables MASQUERADE规则
	if !iptablesRuleExists("nat", "POSTROUTING", "-j MASQUERADE") {
		fmt.Println("  ⚠ iptables MASQUERADE规则未配置")
		return fmt.Errorf("iptables MASQUERADE规则未配置，请先运行初始化命令")
	}
	fmt.Println("  ✓ iptables MASQUERADE已配置")

	fmt.Println("\n✓ 环境检查通过")
	return nil
}

// Initialize 初始化系统
func Initialize() error {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║             TrueWord Node 系统初始化                       ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. 检查是否是root
	if os.Geteuid() != 0 {
		return fmt.Errorf("❌ 必须以root权限运行")
	}

	// 2. 检查必需的命令
	fmt.Println("【检查系统环境】")
	requiredCommands := []string{"ip", "iptables", "ping", "sysctl"}
	for _, cmd := range requiredCommands {
		if !commandExists(cmd) {
			return fmt.Errorf("❌ 缺少必需的命令: %s，请先安装", cmd)
		}
	}
	fmt.Printf("  ✓ 系统命令检查通过\n")

	fmt.Println()

	// 3. 启用IP转发
	fmt.Println("【配置网络参数】")
	if err := setSysctl("net.ipv4.ip_forward", "1"); err != nil {
		return fmt.Errorf("❌ 启用IP转发失败: %w", err)
	}
	fmt.Println("  ✓ IP转发已启用")

	// 永久保存
	sysctlConf := "/etc/sysctl.d/99-trueword-node.conf"
	content := "# TrueWord Node Configuration\nnet.ipv4.ip_forward = 1\n"
	if err := os.WriteFile(sysctlConf, []byte(content), 0644); err != nil {
		fmt.Printf("  ⚠️  警告: 无法持久化配置: %v\n", err)
	}

	// 4. 配置iptables MASQUERADE
	if !iptablesRuleExists("nat", "POSTROUTING", "-j MASQUERADE") {
		if err := addIptablesRule("nat", "POSTROUTING", "-j MASQUERADE"); err != nil {
			return fmt.Errorf("❌ 添加iptables规则失败: %w", err)
		}
	}
	fmt.Println("  ✓ iptables MASQUERADE已配置")
	fmt.Println()

	// 5. 检查是否存在旧配置，如果存在则警告
	fmt.Println("【初始化配置目录】")
	dirs := []string{
		config.ConfigDir,
		"/var/lib/trueword_node",
		"/etc/trueword_node/policies",
	}

	// 检查是否存在旧配置
	hasOldConfig := false
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			hasOldConfig = true
			break
		}
	}

	// 如果存在旧配置，警告并要求确认
	if hasOldConfig {
		fmt.Println()
		fmt.Println("╔═════════════════════════════════════════════════════════════╗")
		fmt.Println("║                         ⚠️  警告 ⚠️                          ║")
		fmt.Println("╚═════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Println("  检测到已存在的配置文件!")
		fmt.Println()
		fmt.Println("  初始化操作将会:")
		fmt.Println("    • 删除所有隧道配置")
		fmt.Println("    • 删除物理接口配置")
		fmt.Println("    • 删除所有策略配置")
		fmt.Println("    • 清空撤销记录")
		fmt.Println()
		fmt.Println("  ⚠️  此操作不可恢复!")
		fmt.Println()
		fmt.Print("  确认要清空所有配置并重新初始化? (yes/no): ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "yes" {
			fmt.Println()
			fmt.Println("  ✗ 初始化已取消")
			fmt.Println()
			return fmt.Errorf("用户取消初始化")
		}

		fmt.Println()
		fmt.Println("  开始清除旧配置...")
	}

	// 清除旧配置
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			if err := os.RemoveAll(dir); err != nil {
				fmt.Printf("  ⚠️  清除旧配置目录 %s 失败: %v\n", dir, err)
			}
		}
	}

	// 重新创建目录
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("❌ 创建目录 %s 失败: %w", dir, err)
		}
	}

	// 创建rev子目录
	if err := os.MkdirAll("/var/lib/trueword_node/rev", 0755); err != nil {
		return fmt.Errorf("❌ 创建撤销目录失败: %w", err)
	}

	fmt.Println("  ✓ 配置目录已清除并重建")

	// 6. 创建默认配置文件
	cfg := config.CreateDefault()
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("❌ 保存配置文件失败: %w", err)
	}
	fmt.Println("  ✓ 配置文件已创建")

	fmt.Println()

	// 7. 扫描物理网络接口
	fmt.Println("【扫描物理网络接口】")
	interfaces, err := network.ScanPhysicalInterfaces()
	if err != nil {
		return fmt.Errorf("❌ 扫描网络接口失败: %w", err)
	}

	if len(interfaces) == 0 {
		fmt.Println("  ⚠️  未找到可用的物理网络接口")
	} else {
		fmt.Printf("  找到 %d 个物理接口\n\n", len(interfaces))

		reader := bufio.NewReader(os.Stdin)
		var selectedInterfaces []network.PhysicalInterface

		for i, iface := range interfaces {
			fmt.Printf("  [%d] %s\n", i+1, iface.Name)
			fmt.Printf("      IP地址: %s\n", iface.IP)
			if iface.Gateway != "" {
				fmt.Printf("      网关:   %s\n", iface.Gateway)
			} else {
				fmt.Printf("      网关:   (未检测到)\n")
			}

			fmt.Print("      添加此接口? [Y/n]: ")
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "" || response == "y" || response == "yes" {
				// 如果未检测到网关，询问用户
				if iface.Gateway == "" {
					fmt.Print("      输入网关(留空=P2P): ")
					gateway, _ := reader.ReadString('\n')
					gateway = strings.TrimSpace(gateway)
					if gateway != "" {
						iface.Gateway = gateway
					}
				}

				selectedInterfaces = append(selectedInterfaces, iface)
				fmt.Printf("      ✓ 已添加\n")
			} else {
				fmt.Printf("      - 已跳过\n")
			}
			fmt.Println()
		}

		if len(selectedInterfaces) == 0 {
			fmt.Println("  ⚠️  未选择任何接口，将无法创建隧道")
		} else {
			// 保存接口配置
			ifaceConfig := &network.InterfaceConfig{
				Interfaces: selectedInterfaces,
			}

			if err := network.SaveInterfaceConfig(ifaceConfig); err != nil {
				return fmt.Errorf("❌ 保存接口配置失败: %w", err)
			}

			fmt.Printf("  ✓ 已保存 %d 个物理接口配置\n", len(selectedInterfaces))
		}
	}

	// 完成提示
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║             ✓ 初始化完成!                                  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("接下来:")
	fmt.Println("  • 查看物理接口:  twnode interface list")
	fmt.Println("  • 创建隧道:      twnode line create")
	fmt.Println("  • 查看帮助:      twnode --help")
	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Println("  • 物理接口: /etc/trueword_node/interfaces/physical.yaml")
	fmt.Println("  • 全局配置: /etc/trueword_node/config.yaml")
	fmt.Println()

	return nil
}

// ShowStatus 显示系统状态
func ShowStatus() error {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    TrueWord Node 状态                      ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 加载检查结果
	checkResults, err := network.LoadCheckResults()
	if err != nil {
		checkResults = &network.AllCheckResults{
			Results: make(map[string]*network.CheckResult),
		}
	}

	// 加载配置获取默认路由设置
	cfg, _ := config.Load()
	defaultExit := ""
	if cfg != nil {
		defaultExit = cfg.Routing.DefaultExit
	}

	// 显示出口状态表格（包括物理接口和隧道）
	fmt.Println("【出口状态】")
	fmt.Println()

	// 创建表格
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"接口名", "类型", "启用状态", "运行状态", "延迟(ms)", "丢包率", "目标IP"})
	t.SetStyle(table.StyleLight)

	// 1. 添加物理接口
	ifaceConfig, err := network.LoadInterfaceConfig()
	if err == nil && len(ifaceConfig.Interfaces) > 0 {
		for _, iface := range ifaceConfig.Interfaces {
			enabledStatus := "启用"
			if !iface.Enabled {
				enabledStatus = "禁用"
			}

			// 检查是否是默认路由出口
			interfaceName := iface.Name
			if defaultExit != "" && iface.Name == defaultExit {
				interfaceName = iface.Name + " *"
			}

			// 从检查结果获取状态
			result, ok := checkResults.Results[iface.Name]
			var runStatus, latency, packetLoss, targetIP string

			if !ok || result == nil {
				runStatus = "未检查"
				latency = "-"
				packetLoss = "-"
				targetIP = "-"
			} else {
				switch result.Status {
				case "UP":
					runStatus = "UP"
					latency = fmt.Sprintf("%.2f", result.Latency)
					packetLoss = fmt.Sprintf("%.0f%%", result.PacketLoss)
					targetIP = result.TargetIP
				case "DOWN":
					runStatus = "DOWN"
					latency = fmt.Sprintf("%.2f", result.Latency)
					packetLoss = fmt.Sprintf("%.0f%%", result.PacketLoss)
					targetIP = result.TargetIP
				case "IDLE":
					runStatus = "IDLE"
					latency = "-"
					packetLoss = "-"
					targetIP = "-"
				default:
					runStatus = "未知"
					latency = "-"
					packetLoss = "-"
					targetIP = "-"
				}
			}

			t.AppendRow(table.Row{
				interfaceName,
				"物理接口",
				enabledStatus,
				runStatus,
				latency,
				packetLoss,
				targetIP,
			})
		}
	}

	// 2. 添加隧道
	tunnelDir := config.ConfigDir + "/tunnels"
	entries, err := os.ReadDir(tunnelDir)
	if err == nil {
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}

			tunnelName := strings.TrimSuffix(entry.Name(), ".yaml")

			// 加载隧道配置获取启用状态
			tunnelConfig, _ := network.LoadTunnelConfig(tunnelName)
			enabledStatus := "禁用"
			if tunnelConfig != nil && tunnelConfig.Enabled {
				enabledStatus = "启用"
			}

			// 检查是否是默认路由出口
			tunnelDisplayName := tunnelName
			if defaultExit != "" && tunnelName == defaultExit {
				tunnelDisplayName = tunnelName + " *"
			}

			result, ok := checkResults.Results[tunnelName]
			var runStatus, latency, packetLoss, targetIP string

			if !ok || result == nil {
				runStatus = "未检查"
				latency = "-"
				packetLoss = "-"
				targetIP = "-"
			} else {
				switch result.Status {
				case "UP":
					runStatus = "UP"
					latency = fmt.Sprintf("%.2f", result.Latency)
					packetLoss = fmt.Sprintf("%.0f%%", result.PacketLoss)
					targetIP = result.TargetIP
				case "DOWN":
					runStatus = "DOWN"
					latency = fmt.Sprintf("%.2f", result.Latency)
					packetLoss = fmt.Sprintf("%.0f%%", result.PacketLoss)
					targetIP = result.TargetIP
				case "IDLE":
					runStatus = "IDLE"
					latency = "-"
					packetLoss = "-"
					targetIP = "-"
				default:
					runStatus = "未知"
					latency = "-"
					packetLoss = "-"
					targetIP = "-"
				}
			}

			t.AppendRow(table.Row{
				tunnelDisplayName,
				"隧道",
				enabledStatus,
				runStatus,
				latency,
				packetLoss,
				targetIP,
			})
		}
	}

	// 渲染表格
	t.Render()

	// 如果设置了默认路由，显示说明
	if defaultExit != "" {
		fmt.Println()
		fmt.Printf("  * 默认路由出口: %s\n", defaultExit)
	}
	fmt.Println()

	// 显示最后检查时间
	if !checkResults.LastUpdate.IsZero() {
		fmt.Printf("  最后检查时间: %s\n", checkResults.LastUpdate.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("  提示: 运行 'twnode line check <ip>' 来检查出口连通性\n")
	}

	// 系统配置
	fmt.Println()
	fmt.Println("【系统配置】")
	fmt.Println()

	// IP转发
	ipForward, _ := checkSysctl("net.ipv4.ip_forward")
	fmt.Printf("  IP转发:             %s\n", map[string]string{"1": "✓ 已启用", "0": "✗ 未启用"}[ipForward])

	// iptables MASQUERADE
	masqueradeExists := iptablesRuleExists("nat", "POSTROUTING", "-j MASQUERADE")
	fmt.Printf("  iptables MASQ:      %s\n", map[bool]string{true: "✓ 已配置", false: "✗ 未配置"}[masqueradeExists])

	// 策略路由数量
	cmd := exec.Command("ip", "rule", "list")
	output, err := cmd.Output()
	policyCount := 0
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if line != "" && strings.Contains(line, "lookup") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					prio := strings.TrimSuffix(parts[0], ":")
					if prio != "0" && prio != "32766" && prio != "32767" {
						policyCount++
					}
				}
			}
		}
	}
	fmt.Printf("  策略路由规则:       %d 条\n", policyCount)

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))

	return nil
}
