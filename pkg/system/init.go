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
	fmt.Println("初始化 TrueWord Node...")

	// 1. 检查是否是root
	if os.Geteuid() != 0 {
		return fmt.Errorf("必须以root权限运行")
	}

	// 2. 检查必需的命令
	fmt.Println("\n检查必需命令...")
	requiredCommands := []string{"ip", "iptables", "ping", "sysctl"}
	for _, cmd := range requiredCommands {
		if !commandExists(cmd) {
			return fmt.Errorf("缺少必需的命令: %s，请先安装", cmd)
		}
		fmt.Printf("  ✓ %s\n", cmd)
	}

	// 3. 启用IP转发
	fmt.Println("\n启用IP转发...")
	if err := setSysctl("net.ipv4.ip_forward", "1"); err != nil {
		return fmt.Errorf("启用IP转发失败: %w", err)
	}
	fmt.Println("  ✓ net.ipv4.ip_forward = 1")

	// 永久保存
	sysctlConf := "/etc/sysctl.d/99-trueword-node.conf"
	content := "# TrueWord Node Configuration\nnet.ipv4.ip_forward = 1\n"
	if err := os.WriteFile(sysctlConf, []byte(content), 0644); err != nil {
		fmt.Printf("  ⚠ 警告: 无法保存到 %s: %v\n", sysctlConf, err)
	} else {
		fmt.Printf("  ✓ 配置已保存到 %s\n", sysctlConf)
	}

	// 4. 配置iptables MASQUERADE
	fmt.Println("\n配置iptables MASQUERADE...")
	if iptablesRuleExists("nat", "POSTROUTING", "-j MASQUERADE") {
		fmt.Println("  ✓ MASQUERADE规则已存在")
	} else {
		if err := addIptablesRule("nat", "POSTROUTING", "-j MASQUERADE"); err != nil {
			return fmt.Errorf("添加iptables规则失败: %w", err)
		}
		fmt.Println("  ✓ 已添加 MASQUERADE 规则")
	}

	// 5. 创建配置目录
	fmt.Println("\n创建配置目录...")
	dirs := []string{
		config.ConfigDir,
		"/var/lib/trueword_node",
		"/var/lib/trueword_node/rev",
		"/etc/trueword_node/policies",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
		fmt.Printf("  ✓ %s\n", dir)
	}

	// 6. 创建默认配置文件
	fmt.Println("\n创建配置文件...")
	cfg := config.CreateDefault()
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}
	fmt.Printf("  ✓ 配置文件已创建: %s\n", config.ConfigDir+"/"+config.ConfigFile)

	// 7. 扫描物理网络接口
	fmt.Println("\n扫描物理网络接口...")
	interfaces, err := network.ScanPhysicalInterfaces()
	if err != nil {
		return fmt.Errorf("扫描网络接口失败: %w", err)
	}

	if len(interfaces) == 0 {
		fmt.Println("  ⚠ 未找到可用的物理网络接口")
	} else {
		fmt.Printf("  找到 %d 个物理接口:\n", len(interfaces))

		reader := bufio.NewReader(os.Stdin)
		var selectedInterfaces []network.PhysicalInterface

		for _, iface := range interfaces {
			fmt.Printf("\n  接口: %s\n", iface.Name)
			fmt.Printf("    IP地址: %s\n", iface.IP)
			if iface.Gateway != "" {
				fmt.Printf("    网关: %s\n", iface.Gateway)
			} else {
				fmt.Printf("    网关: (未检测到，可能是P2P连接)\n")
			}

			fmt.Print("    是否添加此接口? [Y/n]: ")
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "" || response == "y" || response == "yes" {
				// 如果未检测到网关，询问用户
				if iface.Gateway == "" {
					fmt.Print("    请输入网关地址(留空表示P2P连接): ")
					gateway, _ := reader.ReadString('\n')
					gateway = strings.TrimSpace(gateway)
					if gateway != "" {
						iface.Gateway = gateway
					}
				}

				selectedInterfaces = append(selectedInterfaces, iface)
				fmt.Printf("    ✓ 已添加 %s\n", iface.Name)
			} else {
				fmt.Printf("    ✗ 跳过 %s\n", iface.Name)
			}
		}

		if len(selectedInterfaces) == 0 {
			fmt.Println("\n  ⚠ 未选择任何接口，将无法创建隧道")
		} else {
			// 保存接口配置
			ifaceConfig := &network.InterfaceConfig{
				Interfaces: selectedInterfaces,
			}

			if err := network.SaveInterfaceConfig(ifaceConfig); err != nil {
				return fmt.Errorf("保存接口配置失败: %w", err)
			}

			fmt.Printf("\n  ✓ 已保存 %d 个物理接口配置\n", len(selectedInterfaces))
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✓ 初始化完成!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\n提示:")
	fmt.Println("  1. 物理接口配置: /etc/trueword_node/interfaces/physical.yaml")
	fmt.Println("  2. 现在可以创建隧道了，每条隧道使用独立的密钥")
	fmt.Println("  3. 配置文件位置: /etc/trueword_node/config.yaml")
	fmt.Println("  4. iptables规则不会自动保存，重启后需要重新配置")
	fmt.Println("     建议使用 iptables-save 保存规则")
	fmt.Println()

	return nil
}

// ShowStatus 显示系统状态
func ShowStatus() error {
	fmt.Println("系统状态:")
	fmt.Println(strings.Repeat("=", 60))

	// IP转发
	ipForward, _ := checkSysctl("net.ipv4.ip_forward")
	fmt.Printf("\nIP转发: %s\n", map[string]string{"1": "已启用 ✓", "0": "未启用 ✗"}[ipForward])

	// iptables MASQUERADE
	masqueradeExists := iptablesRuleExists("nat", "POSTROUTING", "-j MASQUERADE")
	fmt.Printf("iptables MASQUERADE: %s\n", map[bool]string{true: "已配置 ✓", false: "未配置 ✗"}[masqueradeExists])

	// 隧道列表
	fmt.Println("\n活动隧道:")
	cmd := exec.Command("ip", "tunnel", "show")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		count := 0
		for _, line := range lines {
			if line != "" && !strings.Contains(line, "remote any") {
				parts := strings.Split(line, ":")
				if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
					fmt.Printf("  - %s\n", strings.TrimSpace(line))
					count++
				}
			}
		}
		if count == 0 {
			fmt.Println("  无")
		}
	}

	// XFRM状态
	fmt.Println("\nIPsec连接:")
	cmd = exec.Command("ip", "xfrm", "state", "list")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "src") {
				fmt.Printf("  %s\n", line)
			}
		}
	} else {
		fmt.Println("  无")
	}

	// 策略路由规则
	fmt.Println("\n策略路由规则:")
	cmd = exec.Command("ip", "rule", "list")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if line != "" {
				// 只显示我们管理的规则（优先级10-999）
				if strings.Contains(line, "lookup") {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						prio := strings.TrimSuffix(parts[0], ":")
						// 简单检查是否在我们的范围内
						if prio != "0" && prio != "32766" && prio != "32767" {
							fmt.Printf("  %s\n", line)
						}
					}
				}
			}
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	return nil
}
