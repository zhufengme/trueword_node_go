package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/spf13/cobra"
	"trueword_node/pkg/config"
	"trueword_node/pkg/ipsec"
	"trueword_node/pkg/routing"
	"trueword_node/pkg/system"
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

	// 输入远程IP
	remoteIP := readInput("远程IP地址: ")
	if remoteIP == "" {
		return fmt.Errorf("远程IP不能为空")
	}

	// 输入远程虚拟IP
	remoteVIP := readInput("远程虚拟IP: ")
	if remoteVIP == "" {
		return fmt.Errorf("远程虚拟IP不能为空")
	}

	// 输入本地IP
	localIP := readInput("本地IP地址: ")
	if localIP == "" {
		return fmt.Errorf("本地IP不能为空")
	}

	// 输入本地虚拟IP
	localVIP := readInput("本地虚拟IP: ")
	if localVIP == "" {
		return fmt.Errorf("本地虚拟IP不能为空")
	}

	// 输入隧道名
	tunnelName := readInput("隧道名称 (留空自动生成): ")
	if tunnelName == "" {
		tunnelName = fmt.Sprintf("tun-%d", rand.Intn(9000)+1000)
		fmt.Printf("自动生成隧道名: %s\n", tunnelName)
	}

	// 输入认证密钥
	authPass := readPassword("认证密钥 (不显示): ")
	if authPass == "" {
		return fmt.Errorf("认证密钥不能为空")
	}

	// 输入加密密钥
	encPass := readPassword("加密密钥 (不显示): ")
	if encPass == "" {
		return fmt.Errorf("加密密钥不能为空")
	}

	// 确认信息
	fmt.Println("\n=== 确认信息 ===")
	fmt.Printf("远程: %s/%s\n", remoteIP, remoteVIP)
	fmt.Printf("本地: %s/%s\n", localIP, localVIP)
	fmt.Printf("隧道名: %s\n", tunnelName)

	confirm := readInput("\n确认创建? (yes/no): ")
	if confirm != "yes" && confirm != "y" {
		fmt.Println("已取消")
		return nil
	}

	// 生成密钥
	authKey, encKey, err := config.GenerateIPsecKeys(authPass, encPass)
	if err != nil {
		return err
	}

	// 创建线路
	fmt.Println("\n开始创建...")
	return ipsec.CreateLine(remoteIP, remoteVIP, localIP, localVIP, tunnelName, authKey, encKey)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	rootCmd := &cobra.Command{
		Use:   "twnode",
		Short: "TrueWord Node - IPsec隧道管理工具",
		Long:  `TrueWord Node 是一个用于管理GRE over IPsec隧道和策略路由的工具`,
	}

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
		Use:   "create [remote_ip/remote_vip] [local_ip/local_vip] [tunnel_name]",
		Short: "创建隧道(自动创建IPsec连接和GRE隧道)",
		Long:  "不带参数时进入交互模式\n带参数时需要指定 --auth-key 和 --enc-key",
		Args:  cobra.RangeArgs(0, 3),
		Run: func(cmd *cobra.Command, args []string) {
			// 如果没有参数，进入交互模式
			if len(args) == 0 {
				if err := interactiveCreateLine(); err != nil {
					fmt.Fprintf(os.Stderr, "创建失败: %v\n", err)
					os.Exit(1)
				}
				return
			}

			// 命令行模式：需要至少2个参数
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "参数不足")
				fmt.Fprintln(os.Stderr, "格式: twnode line create <remote_ip/remote_vip> <local_ip/local_vip> [tunnel_name]")
				fmt.Fprintln(os.Stderr, "或直接运行: twnode line create (进入交互模式)")
				os.Exit(1)
			}

			// 解析参数
			remoteParts := strings.Split(args[0], "/")
			localParts := strings.Split(args[1], "/")

			if len(remoteParts) != 2 || len(localParts) != 2 {
				fmt.Fprintln(os.Stderr, "格式错误")
				fmt.Fprintln(os.Stderr, "格式: twnode line create <remote_ip/remote_vip> <local_ip/local_vip> [tunnel_name]")
				os.Exit(1)
			}

			remoteIP := remoteParts[0]
			remoteVIP := remoteParts[1]
			localIP := localParts[0]
			localVIP := localParts[1]

			// 隧道名
			tunnelName := ""
			if len(args) >= 3 {
				tunnelName = args[2]
			} else {
				tunnelName = fmt.Sprintf("tun-%d", rand.Intn(9000)+1000)
				fmt.Printf("隧道名未指定，自动分配: %s\n", tunnelName)
			}

			// 获取密钥
			authPass, _ := cmd.Flags().GetString("auth-key")
			encPass, _ := cmd.Flags().GetString("enc-key")

			if authPass == "" || encPass == "" {
				fmt.Fprintln(os.Stderr, "错误: 命令行模式必须指定 --auth-key 和 --enc-key")
				fmt.Fprintln(os.Stderr, "或不带参数进入交互模式: twnode line create")
				os.Exit(1)
			}

			// 生成密钥
			authKey, encKey, err := config.GenerateIPsecKeys(authPass, encPass)
			if err != nil {
				fmt.Fprintf(os.Stderr, "生成密钥失败: %v\n", err)
				os.Exit(1)
			}

			// 创建线路
			if err := ipsec.CreateLine(remoteIP, remoteVIP, localIP, localVIP, tunnelName, authKey, encKey); err != nil {
				fmt.Fprintf(os.Stderr, "创建线路失败: %v\n", err)
				os.Exit(1)
			}
		},
	}
	lineCreateCmd.Flags().String("auth-key", "", "认证密钥字符串(命令行模式必需)")
	lineCreateCmd.Flags().String("enc-key", "", "加密密钥字符串(命令行模式必需)")

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

	lineCmd.AddCommand(lineCreateCmd, lineRemoveCmd)

	// 策略路由命令组
	policyCmd := &cobra.Command{
		Use:   "policy",
		Short: "管理策略路由",
	}

	// 创建策略组(自动分配优先级)
	policyCreateCmd := &cobra.Command{
		Use:   "create <group_name> <exit_interface>",
		Short: "创建策略组",
		Long:  "创建策略组，优先级自动分配。同一CIDR只能属于一个策略组",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

			// 加载现有策略组,找到最大优先级
			maxPrio := routing.PrioUserPolicyBase - 1
			entries, err := os.ReadDir(routing.PolicyDir)
			if err == nil {
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".policy") {
						groupName := strings.TrimSuffix(entry.Name(), ".policy")
						if err := pm.LoadGroup(groupName); err == nil {
							// 获取该组优先级(临时访问)
							group := pm.GetGroup(groupName)
							if group != nil && group.Priority > maxPrio {
								maxPrio = group.Priority
							}
						}
					}
				}
			}

			// 分配新优先级
			newPrio := maxPrio + 1
			if newPrio >= routing.PrioDefault {
				fmt.Fprintf(os.Stderr, "错误: 策略组数量已达上限\n")
				os.Exit(1)
			}

			if err := pm.CreateGroup(args[0], args[1], newPrio); err != nil {
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
		Long:  "设置策略路由的默认路由(0.0.0.0/0)，作为兜底路由\n设置后自动应用到内核\n不设置则使用系统默认路由表",
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
				fmt.Printf("默认路由(0.0.0.0/0): (未设置) -> %s\n", args[0])
			} else {
				fmt.Printf("默认路由(0.0.0.0/0): %s -> %s\n", oldExit, args[0])
			}

			// 应用策略
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
					if err := pm.LoadGroup(groupName); err != nil {
						fmt.Fprintf(os.Stderr, "加载策略组 %s 失败: %v\n", groupName, err)
					}
				}
			}

			// 设置新的默认路由
			pm.SetDefaultExit(cfg.Routing.DefaultExit)

			// 应用
			if err := pm.Apply(); err != nil {
				fmt.Fprintf(os.Stderr, "应用策略失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("\n✓ 默认路由已应用")
		},
	}

	// 清除默认路由
	policyUnsetDefaultCmd := &cobra.Command{
		Use:   "unset-default",
		Short: "清除默认路由设置",
		Long:  "清除默认路由(0.0.0.0/0)设置，使用系统默认路由表\n清除后自动应用到内核",
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

			fmt.Printf("默认路由(0.0.0.0/0): %s -> (未设置)\n", oldExit)

			// 应用策略
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
					if err := pm.LoadGroup(groupName); err != nil {
						fmt.Fprintf(os.Stderr, "加载策略组 %s 失败: %v\n", groupName, err)
					}
				}
			}

			// 清除默认路由
			pm.SetDefaultExit(cfg.Routing.DefaultExit)

			// 应用
			if err := pm.Apply(); err != nil {
				fmt.Fprintf(os.Stderr, "应用策略失败: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("\n✓ 默认路由已清除")
		},
	}

	// 应用策略
	policyApplyCmd := &cobra.Command{
		Use:   "apply",
		Short: "应用策略路由到内核",
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

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
		Use:   "revoke",
		Short: "撤销所有策略路由",
		Run: func(cmd *cobra.Command, args []string) {
			pm = routing.NewPolicyManager()

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

	policyCmd.AddCommand(policyCreateCmd, policyAddCmd, policyImportCmd,
		policyListCmd, policyDefaultCmd, policyUnsetDefaultCmd,
		policyApplyCmd, policyRevokeCmd)

	// 添加所有命令
	rootCmd.AddCommand(initCmd, statusCmd, lineCmd, policyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
