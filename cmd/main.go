package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/spf13/cobra"
	"trueword_node/pkg/config"
	"trueword_node/pkg/ipsec"
	"trueword_node/pkg/network"
	"trueword_node/pkg/routing"
	"trueword_node/pkg/system"
)

const (
	// 版本信息
	Version = "1.1"
)

var (
	// 全局配置
	cfg *config.Config
	pm  *routing.PolicyManager
)

// 读取用户输入(带提示)
func readInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// 读取密码(不显示)
func readPassword(prompt string) string {
	fmt.Print(prompt)
	password, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return strings.TrimSpace(string(password))
}

// 交互式创建隧道
func interactiveCreateLine() error {
	fmt.Println("=== 交互式创建隧道 ===\n")

	// 1. 列出可用的父接口
	fmt.Println("正在扫描可用的父接口...")
	parents, err := network.ListAvailableParentInterfaces()
	if err != nil {
		return fmt.Errorf("获取父接口失败: %w", err)
	}

	fmt.Println("\n可用的父接口:")
	fmt.Println(strings.Repeat("-", 60))
	for i, parent := range parents {
		fmt.Printf("[%d] %s (%s)\n", i+1, parent.Name, parent.Type)
		fmt.Printf("    IP: %s\n", parent.IP)
		if parent.Gateway != "" {
			fmt.Printf("    网关: %s\n", parent.Gateway)
		}
	}
	fmt.Println(strings.Repeat("-", 60))

	// 2. 选择父接口
	var parentInterface string
	for {
		choice := readInput("\n选择父接口 (输入编号或名称): ")
		if choice == "" {
			return fmt.Errorf("必须选择父接口")
		}

		// 尝试按编号选择
		var selectedIdx int
		if _, err := fmt.Sscanf(choice, "%d", &selectedIdx); err == nil {
			if selectedIdx >= 1 && selectedIdx <= len(parents) {
				parentInterface = parents[selectedIdx-1].Name
				break
			}
		}

		// 尝试按名称选择
		found := false
		for _, parent := range parents {
			if parent.Name == choice {
				parentInterface = choice
				found = true
				break
			}
		}
		if found {
			break
		}

		fmt.Println("无效的选择，请重新输入")
	}

	fmt.Printf("已选择父接口: %s\n", parentInterface)

	// 3. 输入远程IP
	remoteIP := readInput("\n远程IP地址: ")
	if remoteIP == "" {
		return fmt.Errorf("远程IP不能为空")
	}

	// 4. 输入远程虚拟IP
	remoteVIP := readInput("远程虚拟IP: ")
	if remoteVIP == "" {
		return fmt.Errorf("远程虚拟IP不能为空")
	}

	// 5. 输入本地虚拟IP (注意: 不是本地IP!)
	localVIP := readInput("本地虚拟IP: ")
	if localVIP == "" {
		return fmt.Errorf("本地虚拟IP不能为空")
	}

	// 6. 输入隧道名
	tunnelName := readInput("隧道名称 (留空自动生成): ")
	if tunnelName == "" {
		tunnelName = fmt.Sprintf("tun-%d", rand.Intn(9000)+1000)
		fmt.Printf("自动生成隧道名: %s\n", tunnelName)
	}

	// 7. 输入认证密钥
	authPass := readInput("认证密钥: ")
	if authPass == "" {
		return fmt.Errorf("认证密钥不能为空")
	}

	// 8. 输入加密密钥
	encPass := readInput("加密密钥: ")
	if encPass == "" {
		return fmt.Errorf("加密密钥不能为空")
	}

	// 9.5. 输入成本值
	var cost int
	costInput := readInput("成本值 (0-100, 默认0, 直接回车跳过): ")
	if costInput != "" {
		if _, err := fmt.Sscanf(costInput, "%d", &cost); err != nil {
			return fmt.Errorf("成本值必须是数字: %w", err)
		}
		if cost < 0 || cost > 100 {
			return fmt.Errorf("成本值必须在0-100之间")
		}
	}

	// 10. 生成密钥
	authKey, encKey, err := config.GenerateIPsecKeys(authPass, encPass)
	if err != nil {
		return err
	}

	// 11. 确认信息
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("=== 确认信息 ===")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("父接口:      %s\n", parentInterface)
	fmt.Printf("远程IP:      %s\n", remoteIP)
	fmt.Printf("远程虚拟IP:  %s\n", remoteVIP)
	fmt.Printf("本地虚拟IP:  %s\n", localVIP)
	fmt.Printf("隧道名:      %s\n", tunnelName)
	fmt.Printf("成本:        %d\n", cost)
	fmt.Println(strings.Repeat("=", 60))

	confirm := readInput("\n确认创建? (yes/no): ")
	if confirm != "yes" && confirm != "y" {
		fmt.Println("已取消")
		return nil
	}

	// 12. 创建隧道配置
	tunnelConfig := &network.TunnelConfig{
		Name:            tunnelName,
		ParentInterface: parentInterface,
		LocalIP:         "", // 自动从父接口获取
		RemoteIP:        remoteIP,
		LocalVIP:        localVIP,
		RemoteVIP:       remoteVIP,
		AuthKey:         authKey,
		EncKey:          encKey,
		Cost:            cost,
		Enabled:         true,
		UseEncryption:   true, // 默认始终加密
	}

	// 13. 使用TunnelManager创建
	fmt.Println("\n开始创建...")
	tm := ipsec.NewTunnelManager(tunnelConfig)
	return tm.Create()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	rootCmd := &cobra.Command{
		Use:   "twnode",
		Short: "TrueWord Node - IPsec隧道管理工具",
		Long:  `TrueWord Node 是一个用于管理GRE over IPsec隧道和策略路由的工具`,
	}

	// 接口管理命令组
	interfaceCmd := &cobra.Command{
		Use:     "interface",
		Short:   "管理物理网络接口",
		Aliases: []string{"iface", "if"},
	}

	// 列出物理接口
	interfaceListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出已配置的物理接口",
		Run: func(cmd *cobra.Command, args []string) {
			ifaceConfig, err := network.LoadInterfaceConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载接口配置失败: %v\n", err)
				os.Exit(1)
			}

			if len(ifaceConfig.Interfaces) == 0 {
				fmt.Println("未配置物理接口，请先运行: twnode init")
				return
			}

			fmt.Println("物理网络接口:")
			fmt.Println(strings.Repeat("=", 60))
			for _, iface := range ifaceConfig.Interfaces {
				status := "✓"
				if !iface.Enabled {
					status = "✗"
				}
				fmt.Printf("\n%s %s\n", status, iface.Name)
				fmt.Printf("  IP地址: %s\n", iface.IP)
				if iface.Gateway != "" {
					fmt.Printf("  网关: %s\n", iface.Gateway)
				} else {
					fmt.Printf("  网关: (P2P连接)\n")
				}
				fmt.Printf("  状态: %s\n", map[bool]string{true: "已启用", false: "已禁用"}[iface.Enabled])
			}
			fmt.Println(strings.Repeat("=", 60))
		},
	}

	// 扫描接口
	interfaceScanCmd := &cobra.Command{
		Use:   "scan",
		Short: "重新扫描物理接口",
		Run: func(cmd *cobra.Command, args []string) {
			interfaces, err := network.ScanPhysicalInterfaces()
			if err != nil {
				fmt.Fprintf(os.Stderr, "扫描失败: %v\n", err)
				os.Exit(1)
			}

			if len(interfaces) == 0 {
				fmt.Println("未找到可用的物理接口")
				return
			}

			fmt.Printf("找到 %d 个物理接口:\n", len(interfaces))
			for _, iface := range interfaces {
				fmt.Printf("\n  %s\n", iface.Name)
				fmt.Printf("    IP: %s\n", iface.IP)
				if iface.Gateway != "" {
					fmt.Printf("    网关: %s\n", iface.Gateway)
				} else {
					fmt.Printf("    网关: (未检测到)\n")
				}
			}
		},
	}

	// 设置接口成本
	interfaceSetCostCmd := &cobra.Command{
		Use:   "set-cost <interface_name> <cost>",
		Short: "设置物理接口的成本值",
		Long:  "设置物理接口的成本值(0-100)，用于故障转移评分",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			interfaceName := args[0]
			var cost int
			if _, err := fmt.Sscanf(args[1], "%d", &cost); err != nil {
				fmt.Fprintf(os.Stderr, "错误: 成本值必须是数字\n")
				os.Exit(1)
			}

			if cost < 0 || cost > 100 {
				fmt.Fprintf(os.Stderr, "错误: 成本值必须在 0-100 之间\n")
				os.Exit(1)
			}

			// 加载接口配置
			ifaceConfig, err := network.LoadInterfaceConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载接口配置失败: %v\n", err)
				os.Exit(1)
			}

			// 查找接口
			iface := ifaceConfig.GetInterfaceByName(interfaceName)
			if iface == nil {
				fmt.Fprintf(os.Stderr, "错误: 接口 %s 不存在\n", interfaceName)
				os.Exit(1)
			}

			// 更新成本
			iface.Cost = cost

			// 保存配置
			if err := network.SaveInterfaceConfig(ifaceConfig); err != nil {
				fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✓ 接口 %s 的成本已设置为 %d\n", interfaceName, cost)
		},
	}

	interfaceCmd.AddCommand(interfaceListCmd, interfaceScanCmd, interfaceSetCostCmd)

	// 初始化命令
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "初始化系统环境",
		Long:  "检查并配置系统环境，包括IP转发、iptables等",
		Run: func(cmd *cobra.Command, args []string) {
			if err := system.Initialize(); err != nil {
				fmt.Fprintf(os.Stderr, "初始化失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 状态命令
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "显示系统状态",
		Run: func(cmd *cobra.Command, args []string) {
			if err := system.ShowStatus(); err != nil {
				fmt.Fprintf(os.Stderr, "获取状态失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 线路管理命令组
	lineCmd := &cobra.Command{
		Use:   "line",
		Short: "管理隧道(GRE over IPsec)",
	}

	// 创建线路
	lineCreateCmd := &cobra.Command{
		Use:   "create [parent_interface] [remote_ip] [remote_vip] [local_vip] [tunnel_name]",
		Short: "创建隧道(自动创建IPsec连接和GRE隧道)",
		Long:  "不带参数时进入交互模式\n带参数格式: twnode line create <parent_interface> <remote_ip> <remote_vip> <local_vip> [tunnel_name] --auth-key <key> --enc-key <key>",
		Args:  cobra.RangeArgs(0, 5),
		Run: func(cmd *cobra.Command, args []string) {
			// 如果没有参数，进入交互模式
			if len(args) == 0 {
				if err := interactiveCreateLine(); err != nil {
					fmt.Fprintf(os.Stderr, "创建失败: %v\n", err)
					os.Exit(1)
				}
				return
			}

			// 命令行模式：需要至少4个参数
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "参数不足")
				fmt.Fprintln(os.Stderr, "格式: twnode line create <parent_interface> <remote_ip> <remote_vip> <local_vip> [tunnel_name] --auth-key <key> --enc-key <key>")
				fmt.Fprintln(os.Stderr, "或直接运行: twnode line create (进入交互模式)")
				os.Exit(1)
			}

			parentInterface := args[0]
			remoteIP := args[1]
			remoteVIP := args[2]
			localVIP := args[3]

			// 隧道名
			tunnelName := ""
			if len(args) >= 5 {
				tunnelName = args[4]
			} else {
				tunnelName = fmt.Sprintf("tun-%d", rand.Intn(9000)+1000)
				fmt.Printf("隧道名未指定，自动分配: %s\n", tunnelName)
			}

			// 获取密钥
			authPass, _ := cmd.Flags().GetString("auth-key")
			encPass, _ := cmd.Flags().GetString("enc-key")
			cost, _ := cmd.Flags().GetInt("cost")

			if authPass == "" {
				fmt.Fprintln(os.Stderr, "错误: 命令行模式必须指定 --auth-key")
				fmt.Fprintln(os.Stderr, "或不带参数进入交互模式: twnode line create")
				os.Exit(1)
			}

			// 验证成本值范围
			if cost < 0 || cost > 100 {
				fmt.Fprintln(os.Stderr, "错误: cost 必须在 0-100 之间")
				os.Exit(1)
			}

			// 如果未指定加密密钥，使用认证密钥
			if encPass == "" {
				encPass = authPass
			}

			// 生成密钥
			authKey, encKey, err := config.GenerateIPsecKeys(authPass, encPass)
			if err != nil {
				fmt.Fprintf(os.Stderr, "生成密钥失败: %v\n", err)
				os.Exit(1)
			}

			// 创建隧道配置
			tunnelConfig := &network.TunnelConfig{
				Name:            tunnelName,
				ParentInterface: parentInterface,
				LocalIP:         "", // 自动从父接口获取
				RemoteIP:        remoteIP,
				LocalVIP:        localVIP,
				RemoteVIP:       remoteVIP,
				AuthKey:         authKey,
				EncKey:          encKey,
				Cost:            cost,
				Enabled:         true,
				UseEncryption:   true, // 始终加密
			}

			// 使用TunnelManager创建
			tm := ipsec.NewTunnelManager(tunnelConfig)
			if err := tm.Create(); err != nil {
				fmt.Fprintf(os.Stderr, "创建失败: %v\n", err)
				os.Exit(1)
			}
		},
	}
	lineCreateCmd.Flags().String("auth-key", "", "认证密钥字符串(命令行模式必需)")
	lineCreateCmd.Flags().String("enc-key", "", "加密密钥字符串(可选,不指定则使用auth-key)")
	lineCreateCmd.Flags().Int("cost", 0, "成本值(0-100,默认0)")

	// 删除隧道
	lineRemoveCmd := &cobra.Command{
		Use:   "remove <tunnel_name>",
		Short: "删除隧道(自动清理IPsec连接和GRE隧道)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := ipsec.RemoveTunnel(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "删除隧道失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 启动单个隧道
	lineStartCmd := &cobra.Command{
		Use:   "start <tunnel_name>",
		Short: "启动隧道",
		Long:  "启动指定的隧道，建立IPsec和GRE连接",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tunnelName := args[0]

			// 加载隧道配置
			tunnelConfig, err := network.LoadTunnelConfig(tunnelName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载隧道配置失败: %v\n", err)
				os.Exit(1)
			}

			// 启动隧道
			tm := ipsec.NewTunnelManager(tunnelConfig)
			if err := tm.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Println()
			fmt.Printf("✓ 隧道 %s 启动成功\n", tunnelName)
		},
	}

	// 停止单个隧道
	lineStopCmd := &cobra.Command{
		Use:   "stop <tunnel_name>",
		Short: "停止隧道",
		Long:  "停止指定的隧道，保留配置，隧道进入IDLE状态",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tunnelName := args[0]

			// 加载隧道配置
			tunnelConfig, err := network.LoadTunnelConfig(tunnelName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载隧道配置失败: %v\n", err)
				os.Exit(1)
			}

			// 停止隧道
			tm := ipsec.NewTunnelManager(tunnelConfig)
			if err := tm.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "停止失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Println()
			fmt.Printf("✓ 隧道 %s 已停止\n", tunnelName)
		},
	}

	// 启用隧道
	lineEnableCmd := &cobra.Command{
		Use:   "enable <tunnel_name>",
		Short: "启用隧道",
		Long:  "启用隧道，使其包含在start-all操作中。如果隧道未启动则启动",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tunnelName := args[0]

			// 加载隧道配置
			tunnelConfig, err := network.LoadTunnelConfig(tunnelName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载隧道配置失败: %v\n", err)
				os.Exit(1)
			}

			// 如果已经启用，直接返回
			if tunnelConfig.Enabled {
				fmt.Printf("隧道 %s 已经是启用状态\n", tunnelName)

				// 检查是否已启动，未启动则启动
				if _, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", tunnelName)); os.IsNotExist(err) {
					fmt.Printf("隧道未启动，正在启动...\n")
					tm := ipsec.NewTunnelManager(tunnelConfig)
					if err := tm.Start(); err != nil {
						fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
						os.Exit(1)
					}
					fmt.Printf("✓ 隧道 %s 启动成功\n", tunnelName)
				}
				return
			}

			// 设置为启用
			tunnelConfig.Enabled = true
			if err := network.SaveTunnelConfig(tunnelConfig); err != nil {
				fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✓ 隧道 %s 已启用\n", tunnelName)

			// 启动隧道
			fmt.Printf("正在启动隧道...\n")
			tm := ipsec.NewTunnelManager(tunnelConfig)
			if err := tm.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ 隧道 %s 启动成功\n", tunnelName)
		},
	}

	// 禁用隧道
	lineDisableCmd := &cobra.Command{
		Use:   "disable <tunnel_name>",
		Short: "禁用隧道",
		Long:  "禁用隧道，使其不包含在start-all操作中。如果隧道正在运行则停止",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tunnelName := args[0]

			// 加载隧道配置
			tunnelConfig, err := network.LoadTunnelConfig(tunnelName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载隧道配置失败: %v\n", err)
				os.Exit(1)
			}

			// 如果已经禁用，直接返回
			if !tunnelConfig.Enabled {
				fmt.Printf("隧道 %s 已经是禁用状态\n", tunnelName)

				// 检查是否在运行，运行中则停止
				if _, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", tunnelName)); err == nil {
					fmt.Printf("隧道正在运行，正在停止...\n")
					tm := ipsec.NewTunnelManager(tunnelConfig)
					if err := tm.Stop(); err != nil {
						fmt.Fprintf(os.Stderr, "停止失败: %v\n", err)
						os.Exit(1)
					}
					fmt.Printf("✓ 隧道 %s 已停止\n", tunnelName)
				}
				return
			}

			// 设置为禁用
			tunnelConfig.Enabled = false
			if err := network.SaveTunnelConfig(tunnelConfig); err != nil {
				fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✓ 隧道 %s 已禁用\n", tunnelName)

			// 停止隧道
			fmt.Printf("正在停止隧道...\n")
			tm := ipsec.NewTunnelManager(tunnelConfig)
			if err := tm.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "停止失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ 隧道 %s 已停止\n", tunnelName)
		},
	}

	// 检查隧道
	lineCheckCmd := &cobra.Command{
		Use:   "check <ip1>[,ip2,ip3...]",
		Short: "检查所有隧道的连通性",
		Long:  "依次ping指定的IP地址，成功则返回。结果保存到缓存文件供status命令使用。",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// 解析IP列表
			targetIPs := strings.Split(args[0], ",")
			for i, ip := range targetIPs {
				targetIPs[i] = strings.TrimSpace(ip)
			}

			if len(targetIPs) == 0 {
				fmt.Fprintln(os.Stderr, "错误: 必须指定至少一个目标IP")
				os.Exit(1)
			}

			if err := network.CheckAllTunnels(targetIPs); err != nil {
				fmt.Fprintf(os.Stderr, "检查失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 启动所有隧道
	lineStartAllCmd := &cobra.Command{
		Use:   "start-all",
		Short: "启动所有隧道",
		Long:  "批量启动所有已配置的隧道(仅限启用的隧道)",
		Run: func(cmd *cobra.Command, args []string) {
			if err := ipsec.StartAllTunnels(); err != nil {
				fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 停止所有隧道
	lineStopAllCmd := &cobra.Command{
		Use:   "stop-all",
		Short: "停止所有隧道",
		Long:  "批量停止所有已配置的隧道(保留配置，仅停止虚拟隧道)",
		Run: func(cmd *cobra.Command, args []string) {
			if err := ipsec.StopAllTunnels(); err != nil {
				fmt.Fprintf(os.Stderr, "停止失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 设置隧道成本
	lineSetCostCmd := &cobra.Command{
		Use:   "set-cost <tunnel_name> <cost>",
		Short: "设置隧道的成本值",
		Long:  "设置隧道的成本值(0-100)，用于故障转移评分",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			tunnelName := args[0]
			var cost int
			if _, err := fmt.Sscanf(args[1], "%d", &cost); err != nil {
				fmt.Fprintf(os.Stderr, "错误: 成本值必须是数字\n")
				os.Exit(1)
			}

			if cost < 0 || cost > 100 {
				fmt.Fprintf(os.Stderr, "错误: 成本值必须在 0-100 之间\n")
				os.Exit(1)
			}

			// 加载隧道配置
			tunnelConfig, err := network.LoadTunnelConfig(tunnelName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载隧道配置失败: %v\n", err)
				os.Exit(1)
			}

			// 更新成本
			tunnelConfig.Cost = cost

			// 保存配置
			if err := network.SaveTunnelConfig(tunnelConfig); err != nil {
				fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✓ 隧道 %s 的成本已设置为 %d\n", tunnelName, cost)
		},
	}

	lineCmd.AddCommand(lineCreateCmd, lineRemoveCmd, lineStartCmd, lineStopCmd,
		lineEnableCmd, lineDisableCmd, lineCheckCmd, lineStartAllCmd, lineStopAllCmd, lineSetCostCmd)

	// 策略路由命令组
	policyCmd := &cobra.Command{
		Use:   "policy",
		Short: "管理策略路由",
	}

	// 创建策略组(自动分配优先级)
	policyCreateCmd := &cobra.Command{
		Use:   "create <group_name> <exit_interface>",
		Short: "创建策略组",
		Long:  "创建策略组，优先级自动分配或手动指定。出口可以是物理接口、隧道或第三方接口(OpenVPN/WireGuard等)\n可选参数 --from 指定源地址限制（接口名/CIDR/IP，默认all）\n可选参数 --priority 手动指定优先级（100-899，默认自动分配）",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

			// 获取 --from 参数
			fromInput, _ := cmd.Flags().GetString("from")

			// 获取 --priority 参数
			priorityInput, _ := cmd.Flags().GetInt("priority")

			var newPrio int

			// 加载所有现有策略组（用于检查优先级冲突）
			existingGroups := make(map[int]string) // priority -> group_name
			entries, err := os.ReadDir(routing.PolicyDir)
			if err == nil {
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".policy") {
						groupName := strings.TrimSuffix(entry.Name(), ".policy")
						if err := pm.LoadGroup(groupName); err == nil {
							group := pm.GetGroup(groupName)
							if group != nil {
								existingGroups[group.Priority] = groupName
							}
						}
					}
				}
			}

			if priorityInput > 0 {
				// 用户指定了优先级，检查是否合法且不冲突
				if priorityInput < routing.PrioUserPolicyBase || priorityInput >= routing.PrioDefault {
					fmt.Fprintf(os.Stderr, "错误: 优先级必须在 %d-%d 之间\n", routing.PrioUserPolicyBase, routing.PrioDefault-1)
					os.Exit(1)
				}

				// 检查优先级是否已被使用
				if existingGroup, exists := existingGroups[priorityInput]; exists {
					fmt.Fprintf(os.Stderr, "错误: 优先级 %d 已被策略组 '%s' 使用\n", priorityInput, existingGroup)
					os.Exit(1)
				}

				newPrio = priorityInput
				fmt.Printf("使用指定优先级: %d\n", newPrio)
			} else {
				// 自动分配优先级，找到最大优先级
				maxPrio := routing.PrioUserPolicyBase - 1
				for prio := range existingGroups {
					if prio > maxPrio {
						maxPrio = prio
					}
				}

				newPrio = maxPrio + 1
				if newPrio >= routing.PrioDefault {
					fmt.Fprintf(os.Stderr, "错误: 策略组数量已达上限\n")
					os.Exit(1)
				}
				fmt.Printf("自动分配优先级: %d\n", newPrio)
			}

			if err := pm.CreateGroup(args[0], args[1], newPrio, fromInput); err != nil {
				fmt.Fprintf(os.Stderr, "创建策略组失败: %v\n", err)
				os.Exit(1)
			}

			if err := pm.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "保存策略组失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✓ 策略组 %s 创建成功 (优先级: %d, 出口: %s)\n", args[0], newPrio, args[1])
		},
	}

	// 添加 --from 和 --priority 标志
	policyCreateCmd.Flags().String("from", "all", "源地址/源地址段/源接口名（默认all表示所有源）")
	policyCreateCmd.Flags().Int("priority", 0, "手动指定优先级（100-899，默认0表示自动分配）")

	// 添加CIDR
	policyAddCmd := &cobra.Command{
		Use:   "add <group_name> <cidr>",
		Short: "向策略组添加CIDR",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

			if err := pm.LoadGroup(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "加载策略组失败: %v\n", err)
				os.Exit(1)
			}

			if err := pm.AddCIDR(args[0], args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "添加CIDR失败: %v\n", err)
				os.Exit(1)
			}

			if err := pm.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✓ 已添加 %s 到策略组 %s\n", args[1], args[0])
		},
	}

	// 从文件导入
	policyImportCmd := &cobra.Command{
		Use:   "import <group_name> <file_path>",
		Short: "从文件批量导入CIDR到策略组",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

			if err := pm.LoadGroup(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "加载策略组失败: %v\n", err)
				os.Exit(1)
			}

			if err := pm.ImportCIDRsFromFile(args[0], args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
				os.Exit(1)
			}

			if err := pm.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 列出策略组
	policyListCmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有策略组",
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

			// 加载所有策略组
			entries, err := os.ReadDir(routing.PolicyDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "读取策略目录失败: %v\n", err)
				os.Exit(1)
			}

			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".policy") {
					groupName := strings.TrimSuffix(entry.Name(), ".policy")
					pm.LoadGroup(groupName)
				}
			}

			pm.ListGroups()
		},
	}

	// 设置默认路由(0.0.0.0/0)
	policyDefaultCmd := &cobra.Command{
		Use:   "default <exit_interface>",
		Short: "设置/切换默认路由(0.0.0.0/0)出口",
		Long:  "设置策略路由的默认路由(0.0.0.0/0)，作为兜底路由\n设置后自动应用到内核（不影响其他策略组）\n不设置则使用系统默认路由表",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// 加载配置
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
				os.Exit(1)
			}

			// 更新配置
			oldExit := cfg.Routing.DefaultExit
			cfg.Routing.DefaultExit = args[0]

			if err := cfg.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
				os.Exit(1)
			}

			if oldExit == "" {
				fmt.Printf("默认路由(0.0.0.0/0): (未设置) -> %s\n\n", args[0])
			} else {
				fmt.Printf("默认路由(0.0.0.0/0): %s -> %s\n\n", oldExit, args[0])
			}

			// 只应用默认路由（不重新应用所有策略组）
			pm = routing.NewPolicyManager()
			pm.SetDefaultExit(cfg.Routing.DefaultExit)

			if err := pm.ApplyDefaultRouteOnly(); err != nil {
				fmt.Fprintf(os.Stderr, "应用默认路由失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("\n✓ 默认路由已应用")
		},
	}

	// 清除默认路由
	policyUnsetDefaultCmd := &cobra.Command{
		Use:   "unset-default",
		Short: "清除默认路由设置",
		Long:  "清除默认路由(0.0.0.0/0)设置，使用系统默认路由表\n清除后自动撤销内核中的默认路由（不影响其他策略组）",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
				os.Exit(1)
			}

			if cfg.Routing.DefaultExit == "" {
				fmt.Println("默认路由未设置")
				return
			}

			oldExit := cfg.Routing.DefaultExit
			cfg.Routing.DefaultExit = ""

			if err := cfg.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("默认路由(0.0.0.0/0): %s -> (未设置)\n\n", oldExit)

			// 只撤销默认路由（不影响其他策略组）
			pm = routing.NewPolicyManager()

			if err := pm.RevokeDefaultRouteOnly(); err != nil {
				fmt.Fprintf(os.Stderr, "撤销默认路由失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("\n✓ 默认路由已清除")
		},
	}

	// 应用策略
	policyApplyCmd := &cobra.Command{
		Use:   "apply [group_name]",
		Short: "应用策略路由到内核",
		Long: "应用所有策略路由或指定的策略组\n" +
			"示例:\n" +
			"  twnode policy apply           # 应用所有策略组和默认路由\n" +
			"  twnode policy apply vpn_group # 只应用指定的策略组",
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

			// 如果指定了策略组名称，只应用该策略组
			if len(args) == 1 {
				groupName := args[0]

				// 加载指定的策略组
				if err := pm.LoadGroup(groupName); err != nil {
					fmt.Fprintf(os.Stderr, "加载策略组 %s 失败: %v\n", groupName, err)
					os.Exit(1)
				}

				group := pm.GetGroup(groupName)
				if group == nil {
					fmt.Fprintf(os.Stderr, "策略组 %s 不存在\n", groupName)
					os.Exit(1)
				}

				// 验证出口接口
				if !network.IsInterfaceUp(group.Exit) {
					fmt.Fprintf(os.Stderr, "出口接口 %s 不存在或未启动\n", group.Exit)
					os.Exit(1)
				}

				info, err := network.GetInterfaceInfo(group.Exit)
				if err != nil {
					fmt.Fprintf(os.Stderr, "无法获取接口 %s 信息: %v\n", group.Exit, err)
					os.Exit(1)
				}

				if info.Type == network.InterfaceTypeLoopback {
					fmt.Fprintf(os.Stderr, "不能使用回环接口作为出口\n")
					os.Exit(1)
				}

				fmt.Printf("应用策略组: %s\n", groupName)

				// 只应用该策略组
				if err := pm.ApplyGroup(group); err != nil {
					fmt.Fprintf(os.Stderr, "应用策略组失败: %v\n", err)
					os.Exit(1)
				}

				// 刷新路由缓存
				fmt.Println("\n刷新路由缓存...")
				exec.Command("ip", "route", "flush", "cache").Run()

				fmt.Println("\n✓ 策略组应用完成")
				return
			}

			// 否则应用所有策略组
			// 加载配置
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
				os.Exit(1)
			}

			// 加载所有策略组
			entries, err := os.ReadDir(routing.PolicyDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "读取策略目录失败: %v\n", err)
				os.Exit(1)
			}

			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".policy") {
					groupName := strings.TrimSuffix(entry.Name(), ".policy")
					if err := pm.LoadGroup(groupName); err != nil {
						fmt.Fprintf(os.Stderr, "加载策略组 %s 失败: %v\n", groupName, err)
					}
				}
			}

			// 设置默认路由
			if cfg.Routing.DefaultExit != "" {
				pm.SetDefaultExit(cfg.Routing.DefaultExit)
			}

			// 应用
			if err := pm.Apply(); err != nil {
				fmt.Fprintf(os.Stderr, "应用策略失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// 撤销策略
	policyRevokeCmd := &cobra.Command{
		Use:   "revoke [group_name]",
		Short: "撤销策略路由",
		Long: "撤销所有策略路由或指定的策略组\n" +
			"示例:\n" +
			"  twnode policy revoke           # 撤销所有策略组和默认路由\n" +
			"  twnode policy revoke vpn_group # 只撤销指定的策略组",
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

			// 如果指定了策略组名称，只撤销该策略组
			if len(args) == 1 {
				groupName := args[0]

				// 加载指定的策略组
				if err := pm.LoadGroup(groupName); err != nil {
					fmt.Fprintf(os.Stderr, "加载策略组 %s 失败: %v\n", groupName, err)
					os.Exit(1)
				}

				// 撤销指定的策略组
				if err := pm.RevokeGroup(groupName); err != nil {
					fmt.Fprintf(os.Stderr, "撤销策略组失败: %v\n", err)
					os.Exit(1)
				}

				fmt.Println("\n✓ 策略组撤销完成")
				return
			}

			// 否则撤销所有策略
			// 加载配置
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
				os.Exit(1)
			}

			// 加载所有策略组
			entries, err := os.ReadDir(routing.PolicyDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "读取策略目录失败: %v\n", err)
				os.Exit(1)
			}

			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".policy") {
					groupName := strings.TrimSuffix(entry.Name(), ".policy")
					pm.LoadGroup(groupName)
				}
			}

			if cfg.Routing.DefaultExit != "" {
				pm.SetDefaultExit(cfg.Routing.DefaultExit)
			}

			if err := pm.Revoke(); err != nil {
				fmt.Fprintf(os.Stderr, "撤销策略失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// failover命令
	policyFailoverCmd := &cobra.Command{
		Use:   "failover <group_name|default> <candidate1,candidate2,...> [check_ip]",
		Short: "根据连通性检查自动切换到最佳出口",
		Long: "使用上次检查结果（line check）选择最佳出口并应用\n" +
			"示例: twnode policy failover vpn_group eth0,tun01,tun02\n\n" +
			"提供 check_ip 参数时将重新检查所有候选出口：\n" +
			"示例: twnode policy failover vpn_group eth0,tun01,tun02 8.8.8.8",
		Args: cobra.RangeArgs(2, 3),
		Run: func(cmd *cobra.Command, args []string) {
			target := args[0]
			candidatesStr := args[1]

			var checkIP string
			if len(args) >= 3 {
				checkIP = args[2]
				// 验证 IP 地址格式
				if net.ParseIP(checkIP) == nil {
					fmt.Fprintf(os.Stderr, "无效的 IP 地址: %s\n", checkIP)
					os.Exit(1)
				}
			}

			// 解析候选出口列表
			candidates := strings.Split(candidatesStr, ",")
			for i := range candidates {
				candidates[i] = strings.TrimSpace(candidates[i])
			}

			if len(candidates) == 0 {
				fmt.Fprintln(os.Stderr, "候选出口列表不能为空")
				os.Exit(1)
			}

			pm := routing.NewPolicyManager()

			if target == "default" {
				// 切换默认路由
				// 加载配置获取当前默认出口
				cfg, err := config.Load()
				if err == nil && cfg.Routing.DefaultExit != "" {
					pm.SetDefaultExit(cfg.Routing.DefaultExit)
				}

				if err := pm.FailoverDefault(candidates, checkIP); err != nil {
					fmt.Fprintf(os.Stderr, "切换默认路由失败: %v\n", err)
					os.Exit(1)
				}

				// 保存新的默认路由到配置文件
				if cfg == nil {
					cfg = &config.Config{}
				}
				cfg.Routing.DefaultExit = pm.GetDefaultExit()
				if err := cfg.Save(); err != nil {
					fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
					os.Exit(1)
				}
			} else {
				// 切换策略组
				if err := pm.FailoverGroup(target, candidates, checkIP); err != nil {
					fmt.Fprintf(os.Stderr, "切换策略组失败: %v\n", err)
					os.Exit(1)
				}
			}
		},
	}

	// 调整策略组优先级命令
	policySetPriorityCmd := &cobra.Command{
		Use:   "set-priority <group_name> <priority>",
		Short: "调整策略组的优先级",
		Long: "调整策略组的优先级(100-899)，会检查优先级冲突，调整后自动重新应用策略组\n" +
			"示例: twnode policy set-priority vpn_group 150",
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			groupName := args[0]
			var newPriority int
			if _, err := fmt.Sscanf(args[1], "%d", &newPriority); err != nil {
				fmt.Fprintf(os.Stderr, "错误: 优先级必须是数字\n")
				os.Exit(1)
			}

			// 验证优先级范围
			if newPriority < routing.PrioUserPolicyBase || newPriority >= routing.PrioDefault {
				fmt.Fprintf(os.Stderr, "错误: 优先级必须在 %d-%d 之间\n", routing.PrioUserPolicyBase, routing.PrioDefault-1)
				os.Exit(1)
			}

			pm := routing.NewPolicyManager()

			// 加载目标策略组
			if err := pm.LoadGroup(groupName); err != nil {
				fmt.Fprintf(os.Stderr, "加载策略组失败: %v\n", err)
				os.Exit(1)
			}

			group := pm.GetGroup(groupName)
			if group == nil {
				fmt.Fprintf(os.Stderr, "策略组 %s 不存在\n", groupName)
				os.Exit(1)
			}

			oldPriority := group.Priority

			// 如果优先级没变，直接返回
			if oldPriority == newPriority {
				fmt.Printf("策略组 %s 的优先级已经是 %d，无需修改\n", groupName, newPriority)
				return
			}

			// 加载所有现有策略组（检查优先级冲突）
			existingGroups := make(map[int]string) // priority -> group_name
			entries, err := os.ReadDir(routing.PolicyDir)
			if err == nil {
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".policy") {
						gName := strings.TrimSuffix(entry.Name(), ".policy")
						if gName == groupName {
							continue // 跳过当前策略组
						}
						if err := pm.LoadGroup(gName); err == nil {
							g := pm.GetGroup(gName)
							if g != nil {
								existingGroups[g.Priority] = gName
							}
						}
					}
				}
			}

			// 检查新优先级是否已被使用
			if conflictGroup, exists := existingGroups[newPriority]; exists {
				fmt.Fprintf(os.Stderr, "错误: 优先级 %d 已被策略组 '%s' 使用\n", newPriority, conflictGroup)
				os.Exit(1)
			}

			fmt.Printf("调整策略组 '%s' 优先级: %d -> %d\n", groupName, oldPriority, newPriority)

			// 检查策略组是否已应用（通过检查内核中的规则）
			checkCmd := fmt.Sprintf("ip rule show pref %d", oldPriority)
			output, err := exec.Command("sh", "-c", checkCmd).CombinedOutput()
			isApplied := err == nil && len(output) > 0 && strings.Contains(string(output), fmt.Sprintf("%d:", oldPriority))

			if isApplied {
				fmt.Println("策略组已应用，先撤销旧配置...")
				// 撤销旧优先级的规则
				delCmd := fmt.Sprintf("ip rule del pref %d", oldPriority)
				exec.Command("sh", "-c", delCmd).Run()

				// 清空旧路由表
				flushCmd := fmt.Sprintf("ip route flush table %d", oldPriority)
				exec.Command("sh", "-c", flushCmd).Run()
			}

			// 更新优先级
			group.Priority = newPriority

			// 保存配置
			if err := pm.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ 优先级已更新并保存\n")

			// 如果策略组之前已应用，重新应用
			if isApplied {
				fmt.Println("\n重新应用策略组...")
				if err := pm.ApplyGroup(group); err != nil {
					fmt.Fprintf(os.Stderr, "应用策略组失败: %v\n", err)
					os.Exit(1)
				}

				// 刷新路由缓存
				exec.Command("ip", "route", "flush", "cache").Run()

				fmt.Printf("✓ 策略组 '%s' 已重新应用 (优先级: %d)\n", groupName, newPriority)
			} else {
				fmt.Printf("✓ 策略组 '%s' 优先级已调整 (优先级: %d)，运行 'twnode policy apply' 以应用\n", groupName, newPriority)
			}
		},
	}

	// 删除策略组命令
	policyDeleteCmd := &cobra.Command{
		Use:   "delete <group_name>",
		Short: "删除指定的策略组",
		Long: "删除策略组配置文件，如果策略已应用则先自动撤销\n" +
			"示例: twnode policy delete vpn_group",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			groupName := args[0]

			pm := routing.NewPolicyManager()

			// 删除策略组
			if err := pm.DeleteGroup(groupName); err != nil {
				fmt.Fprintf(os.Stderr, "删除策略组失败: %v\n", err)
				os.Exit(1)
			}
		},
	}

	policyCmd.AddCommand(policyCreateCmd, policyAddCmd, policyImportCmd,
		policyListCmd, policyDefaultCmd, policyUnsetDefaultCmd,
		policyApplyCmd, policyRevokeCmd, policyFailoverCmd, policySetPriorityCmd, policyDeleteCmd)

	// 版本命令
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("TrueWord Node v%s\n", Version)
			fmt.Println("GRE over IPsec 隧道管理工具")
		},
	}

	// 添加所有命令
	rootCmd.AddCommand(initCmd, statusCmd, interfaceCmd, lineCmd, policyCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
