package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// setupIptablesPersistence 设置iptables规则持久化
func setupIptablesPersistence() error {
	// 1. 创建iptables脚本
	scriptPath := "/usr/local/bin/twnode-iptables.sh"
	scriptContent := `#!/bin/bash
# TrueWord Node iptables 规则
# 系统启动时自动应用

# 清空可能的重复规则（防止多次运行导致重复）
iptables -t nat -D POSTROUTING -j MASQUERADE 2>/dev/null || true

# 添加 MASQUERADE 规则
iptables -t nat -A POSTROUTING -j MASQUERADE

exit 0
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("创建脚本失败: %w", err)
	}

	// 2. 创建systemd service
	servicePath := "/etc/systemd/system/twnode-iptables.service"
	serviceContent := `[Unit]
Description=TrueWord Node iptables Rules
After=network-pre.target
Before=network.target
DefaultDependencies=no

[Service]
Type=oneshot
ExecStart=/usr/local/bin/twnode-iptables.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("创建systemd service失败: %w", err)
	}

	// 3. 重载systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重载systemd失败: %w", err)
	}

	// 4. 启用service（开机自启）
	cmd = exec.Command("systemctl", "enable", "twnode-iptables.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("启用service失败: %w", err)
	}

	// 5. 立即启动service
	cmd = exec.Command("systemctl", "start", "twnode-iptables.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("启动service失败: %w", err)
	}

	return nil
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
	fmt.Println("  ✓ IP转发已启用（当前会话）")

	// 检查IP转发是否已持久化
	sysctlConf := "/etc/sysctl.d/99-trueword-node.conf"
	if _, err := os.Stat(sysctlConf); err == nil {
		// 配置文件已存在
		fmt.Printf("  ✓ 已持久化（%s）\n", sysctlConf)
	} else {
		// 配置文件不存在，询问是否持久化
		fmt.Println()
		fmt.Println("  ℹ️  IP转发配置是临时的，重启后会失效")
		fmt.Print("  是否持久化到系统配置? (Y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			content := "# TrueWord Node Configuration\nnet.ipv4.ip_forward = 1\n"
			if err := os.WriteFile(sysctlConf, []byte(content), 0644); err != nil {
				fmt.Printf("  ⚠️  持久化失败: %v\n", err)
			} else {
				fmt.Printf("  ✓ 已持久化到 %s\n", sysctlConf)
			}
		} else {
			fmt.Println("  - 已跳过持久化")
		}
	}

	fmt.Println()

	// 4. 配置iptables MASQUERADE
	if !iptablesRuleExists("nat", "POSTROUTING", "-j MASQUERADE") {
		if err := addIptablesRule("nat", "POSTROUTING", "-j MASQUERADE"); err != nil {
			return fmt.Errorf("❌ 添加iptables规则失败: %w", err)
		}
	}
	fmt.Println("  ✓ iptables MASQUERADE已配置（当前会话）")

	// 检查iptables规则是否已持久化
	servicePath := "/etc/systemd/system/twnode-iptables.service"
	if _, err := os.Stat(servicePath); err == nil {
		// Service文件已存在，检查是否已启用
		cmd := exec.Command("systemctl", "is-enabled", "twnode-iptables.service")
		if err := cmd.Run(); err == nil {
			fmt.Println("  ✓ 已持久化（systemd service已启用）")
		} else {
			fmt.Println("  ⚠️  systemd service存在但未启用，建议重新配置")
		}
	} else {
		// Service文件不存在，询问是否持久化
		fmt.Println()
		fmt.Println("  ℹ️  iptables规则是临时的，重启后会失效")
		fmt.Print("  是否通过systemd持久化? (Y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			if err := setupIptablesPersistence(); err != nil {
				fmt.Printf("  ⚠️  持久化失败: %v\n", err)
				fmt.Println("  提示: 您可以手动配置 iptables-persistent 或其他持久化方案")
			} else {
				fmt.Println("  ✓ 已通过systemd持久化iptables规则")
			}
		} else {
			fmt.Println("  - 已跳过持久化")
		}
	}

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

	// 构建并显示接口树
	roots, err := BuildInterfaceTree(checkResults, defaultExit)
	if err != nil {
		return fmt.Errorf("构建接口树失败: %w", err)
	}

	PrintInterfaceTree(roots)

	// 图例说明
	fmt.Println()
	fmt.Println("【图例】")
	fmt.Printf("  \033[92m✓\033[0m 正常  \033[31m✗\033[0m 异常  \033[33m○\033[0m 未启动  \033[36m○\033[0m 未检查  \033[90m⊗\033[0m 已禁用  \033[93m★\033[0m 默认路由出口\n")
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
