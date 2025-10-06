package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"trueword_node/pkg/network"
)

const (
	RevDir           = "/var/lib/trueword_node/rev"
	PeerConfigDir    = "/var/lib/trueword_node/peer_configs"
)

// WireGuardTunnel WireGuard 隧道结构
type WireGuardTunnel struct {
	Name            string
	Mode            string // "server" 或 "client"
	LocalIP         string
	RemoteIP        string
	LocalVIP        string
	RemoteVIP       string
	PrivateKey      string
	PeerPublicKey   string
	ListenPort      int
	PeerListenPort  int
}

// 执行命令并记录 (静默执行,只在出错时显示)
func execCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	command := exec.Command(parts[0], parts[1:]...)
	output, err := command.CombinedOutput()
	if err != nil {
		fmt.Printf("\n❌ 命令执行失败:\n")
		fmt.Printf("   命令: %s\n", cmd)
		fmt.Printf("   错误: %s\n", string(output))
		return fmt.Errorf("命令执行失败: %w", err)
	}
	return nil
}

// 执行命令不报错
func execCommandNoError(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}
	command := exec.Command(parts[0], parts[1:]...)
	command.Run()
}

// 通过 stdin 传递私钥执行 wg set
func execWGSetPrivateKey(interfaceName, privateKey string) error {
	cmd := exec.Command("wg", "set", interfaceName, "private-key", "/dev/stdin")
	cmd.Stdin = strings.NewReader(privateKey)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("\n❌ 设置私钥失败:\n")
		fmt.Printf("   错误: %s\n", string(output))
		return fmt.Errorf("设置私钥失败: %w", err)
	}
	return nil
}

// 记录撤销命令
func recordRevCommands(revFile string, commands []string) error {
	if err := os.MkdirAll(RevDir, 0755); err != nil {
		return fmt.Errorf("创建撤销目录失败: %w", err)
	}

	revPath := filepath.Join(RevDir, revFile)
	content := strings.Join(commands, "\n")
	return os.WriteFile(revPath, []byte(content), 0644)
}

// 执行撤销命令
func executeRevCommands(revFile string) error {
	revPath := filepath.Join(RevDir, revFile)

	data, err := os.ReadFile(revPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	commands := strings.Split(string(data), "\n")
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" {
			execCommandNoError(cmd)
		}
	}

	os.Remove(revPath)
	return nil
}

// CheckWireGuardInstalled 检查 WireGuard 是否安装
func CheckWireGuardInstalled() error {
	// 检查 wg 命令
	if _, err := exec.LookPath("wg"); err != nil {
		return fmt.Errorf("未找到 wg 命令，请安装 WireGuard:\n" +
			"  Ubuntu/Debian: sudo apt install wireguard\n" +
			"  CentOS/RHEL 8+: sudo dnf install wireguard-tools\n" +
			"  Alpine: sudo apk add wireguard-tools")
	}

	// 检查内核模块是否可用
	// 1. 尝试加载模块
	modprobeCmd := exec.Command("modprobe", "wireguard")
	modprobeErr := modprobeCmd.Run()

	// 2. 检查模块是否已加载
	lsmodCmd := exec.Command("lsmod")
	lsmodOutput, _ := lsmodCmd.CombinedOutput()
	moduleLoaded := strings.Contains(string(lsmodOutput), "wireguard")

	// 3. 检查 /sys/module/wireguard（某些内核编译进去，不显示在lsmod）
	_, sysModuleErr := os.Stat("/sys/module/wireguard")

	if modprobeErr != nil && !moduleLoaded && os.IsNotExist(sysModuleErr) {
		return fmt.Errorf("WireGuard 内核模块不可用，请确认:\n" +
			"  1. 内核版本 >= 5.6 (WireGuard 已内置)\n" +
			"  2. 或安装 WireGuard 内核模块:\n" +
			"     Ubuntu/Debian: sudo apt install wireguard\n" +
			"     CentOS/RHEL 8: sudo dnf install kmod-wireguard\n" +
			"  3. 检查内核是否支持: modprobe wireguard && lsmod | grep wireguard")
	}

	// 测试创建临时接口（最终验证）
	testIfaceName := fmt.Sprintf("wgtest%d", os.Getpid())
	testCmd := exec.Command("ip", "link", "add", "name", testIfaceName, "type", "wireguard")
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("无法创建 WireGuard 接口，内核可能不支持 WireGuard:\n" +
			"  错误: %v\n" +
			"  请确认内核版本 >= 5.6 或已安装 WireGuard 内核模块", err)
	}

	// 清理测试接口
	cleanupCmd := exec.Command("ip", "link", "del", testIfaceName)
	cleanupCmd.Run()

	return nil
}

// Create 创建 WireGuard 隧道
func (wg *WireGuardTunnel) Create() error {
	// 检查并清理可能冲突的 WireGuard 配置
	if err := checkAndCleanWireGuardConflicts(wg.Name); err != nil {
		return err
	}

	// 清理旧配置（如果存在）
	revFile := fmt.Sprintf("%s.rev", wg.Name)
	executeRevCommands(revFile)

	// 再次检查并强制删除（防止之前创建失败但接口残留）
	if interfaceExists(wg.Name) {
		fmt.Printf("   ⚠️  接口 %s 已存在，正在清理...\n", wg.Name)
		execCommandNoError(fmt.Sprintf("ip link set dev %s down", wg.Name))
		execCommandNoError(fmt.Sprintf("ip link del dev %s", wg.Name))
	}

	// 记录撤销命令
	revCommands := []string{
		fmt.Sprintf("ip link set dev %s down", wg.Name),
		fmt.Sprintf("ip link del dev %s", wg.Name),
		fmt.Sprintf("ip route del %s/32 dev %s table 80", wg.RemoteVIP, wg.Name),
	}
	recordRevCommands(revFile, revCommands)

	// 1. 创建 WireGuard 接口
	cmd := fmt.Sprintf("ip link add name %s type wireguard", wg.Name)
	if err := execCommand(cmd); err != nil {
		return err
	}

	// 2. 设置私钥 (通过 stdin，避免命令行泄露)
	if err := execWGSetPrivateKey(wg.Name, wg.PrivateKey); err != nil {
		return err
	}

	// 3. 设置监听端口（如果指定了端口）
	if wg.ListenPort > 0 {
		cmd = fmt.Sprintf("wg set %s listen-port %d", wg.Name, wg.ListenPort)
		if err := execCommand(cmd); err != nil {
			return err
		}
	}
	// 如果端口为 0，WireGuard 会自动分配一个随机端口

	// 4. 添加对端配置
	// allowed-ips 设置为 0.0.0.0/0，不使用 WireGuard 内置路由
	// 路由完全由本软件的策略路由系统控制
	var peerCmd string
	if wg.Mode == "client" {
		// 客户端模式：配置 endpoint 和 persistent-keepalive
		peerCmd = fmt.Sprintf("wg set %s peer %s endpoint %s:%d allowed-ips 0.0.0.0/0 persistent-keepalive 25",
			wg.Name, wg.PeerPublicKey, wg.RemoteIP, wg.PeerListenPort)
	} else {
		// 服务端模式：不配置 endpoint（等待客户端连接），不需要 persistent-keepalive
		// 仅配置 peer 公钥和 allowed-ips
		peerCmd = fmt.Sprintf("wg set %s peer %s allowed-ips 0.0.0.0/0",
			wg.Name, wg.PeerPublicKey)
	}
	if err := execCommand(peerCmd); err != nil {
		return err
	}

	// 5. 配置本地虚拟 IP
	cmd = fmt.Sprintf("ip addr add %s/32 dev %s", wg.LocalVIP, wg.Name)
	if err := execCommand(cmd); err != nil {
		return err
	}

	// 6. 启动接口
	cmd = fmt.Sprintf("ip link set dev %s up", wg.Name)
	if err := execCommand(cmd); err != nil {
		return err
	}

	// 7. 确保路由规则存在 (表80用于虚拟IP路由)
	checkCmd := exec.Command("bash", "-c", "ip rule list | grep -q ^80:")
	if err := checkCmd.Run(); err != nil {
		cmd = "ip rule add from all lookup 80 pref 80"
		if err := execCommand(cmd); err != nil {
			return err
		}
	}

	// 8. 添加对端 VIP 路由到表80
	cmd = fmt.Sprintf("ip route add %s/32 dev %s table 80", wg.RemoteVIP, wg.Name)
	if err := execCommand(cmd); err != nil {
		return err
	}

	fmt.Printf("   ✓ WireGuard隧道已创建\n")

	// WireGuard 握手说明
	if wg.Mode == "client" {
		fmt.Printf("\n   ⏳ 客户端模式：正在触发握手...\n")
		// 客户端模式：主动触发握手
		time.Sleep(1 * time.Second) // 等待接口就绪

		// 主动触发握手：使用 wg 命令检查状态并发送 ping
		fmt.Printf("   ⏳ 正在建立加密连接（首次握手可能需要5-10秒）...\n")

		// 先主动触发握手（发送多个 ping 包）
		triggerWireGuardHandshake(wg.RemoteVIP, wg.Name)

		// 等待握手完成
		if waitForWireGuardHandshake(wg.Name, 15) { // 15秒超时
			// 握手成功，测试连通性
			if pingHost(wg.RemoteVIP, 3) {
				fmt.Printf("   ✓ 隧道连接成功 (%s <-> %s)\n", wg.LocalVIP, wg.RemoteVIP)
				return nil
			}
		}

		// 握手失败或超时
		fmt.Printf("   ⚠️  握手超时，但隧道已创建\n")
		fmt.Printf("      提示：WireGuard 握手会在有流量时自动建立\n")
		fmt.Printf("      - 检查对端是否已启动: %s\n", wg.RemoteIP)
		fmt.Printf("      - 检查防火墙是否允许 UDP %d\n", wg.PeerListenPort)
		fmt.Printf("      - 如仍无法连接，尝试: twnode line stop %s && twnode line start %s\n", wg.Name, wg.Name)
		return nil
	} else {
		// 服务端模式：被动等待客户端连接
		fmt.Printf("\n   ⏳ 服务端模式：等待客户端连接...\n")
		time.Sleep(2 * time.Second)

		// 尝试 ping（可能对端还未连接）
		if pingHost(wg.RemoteVIP, 5) {
			fmt.Printf("   ✓ 隧道连接成功 (%s <-> %s)\n", wg.LocalVIP, wg.RemoteVIP)
			return nil
		} else {
			fmt.Printf("   ⚠️  等待客户端建立连接\n")
			fmt.Printf("      服务端已就绪，监听端口: %d\n", wg.ListenPort)
			fmt.Printf("      等待远程节点 %s 连接...\n", wg.RemoteIP)
			fmt.Printf("      稍后可使用 'twnode line check %s' 测试连通性\n", wg.Name)
			return nil
		}
	}
}

// Remove 删除 WireGuard 隧道
func RemoveTunnel(tunnelName string) error {
	fmt.Printf("删除隧道: %s\n", tunnelName)

	// 执行撤销命令清理网络配置
	revFile := fmt.Sprintf("%s.rev", tunnelName)
	if err := executeRevCommands(revFile); err != nil {
		return fmt.Errorf("❌ 清理网络配置失败: %w", err)
	}
	fmt.Printf("  ✓ 网络配置已清理\n")

	// 删除隧道配置文件
	if err := network.DeleteTunnelConfig(tunnelName); err != nil {
		return fmt.Errorf("❌ 删除配置文件失败: %w", err)
	}
	fmt.Printf("  ✓ 配置文件已删除\n")

	fmt.Printf("✓ 隧道 %s 删除完成\n", tunnelName)
	return nil
}

// 检查接口是否存在
func interfaceExists(name string) bool {
	_, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", name))
	return err == nil
}

// Ping检查
func pingHost(host string, timeout int) bool {
	cmd := exec.Command("ping", "-c", "3", "-W", fmt.Sprintf("%d", timeout), host)
	err := cmd.Run()
	return err == nil
}

// pingHostWithRetry Ping检查（带重试，用于WireGuard握手）
func pingHostWithRetry(host string, count int, totalTimeout int) bool {
	// WireGuard 握手可能需要时间，使用较长的包间隔和总超时
	// 发送 count 个包，包间隔 2 秒，总超时 totalTimeout 秒
	startTime := time.Now()

	for time.Since(startTime).Seconds() < float64(totalTimeout) {
		cmd := exec.Command("ping", "-c", "1", "-W", "2", host)
		if err := cmd.Run(); err == nil {
			// 第一个包通了，再发几个确认
			cmd = exec.Command("ping", "-c", fmt.Sprintf("%d", count-1), "-W", "2", host)
			return cmd.Run() == nil
		}
		// 等待1秒后重试（给WireGuard时间建立握手）
		time.Sleep(1 * time.Second)
	}

	return false
}

// triggerWireGuardHandshake 主动触发 WireGuard 握手
func triggerWireGuardHandshake(remoteVIP, interfaceName string) {
	// 发送多个 ping 包主动触发握手
	// WireGuard 握手由数据包触发，多发几个增加成功率
	for i := 0; i < 5; i++ {
		cmd := exec.Command("ping", "-c", "1", "-W", "1", "-I", interfaceName, remoteVIP)
		cmd.Run() // 忽略错误，只是触发握手
		time.Sleep(500 * time.Millisecond)
	}
}

// waitForWireGuardHandshake 等待 WireGuard 握手完成
func waitForWireGuardHandshake(interfaceName string, timeout int) bool {
	startTime := time.Now()

	for time.Since(startTime).Seconds() < float64(timeout) {
		// 使用 wg show 检查握手状态
		cmd := exec.Command("wg", "show", interfaceName, "latest-handshakes")
		output, err := cmd.CombinedOutput()
		if err == nil {
			// 输出格式: <公钥> <时间戳>
			// 如果时间戳不为0，说明握手成功
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					timestamp := fields[1]
					// 时间戳大于0说明有握手记录
					if timestamp != "0" {
						return true
					}
				}
			}
		}

		// 每秒检查一次
		time.Sleep(1 * time.Second)
	}

	return false
}

// GeneratePeerCommand 生成对端创建命令（完整命令行，可直接复制执行）
func GeneratePeerCommand(config *network.TunnelConfig, peerPrivateKey, peerPublicKey string) string {
	var sb strings.Builder

	sb.WriteString("╔═══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║  对端配置 (请在远程主机执行以下命令)                      ║\n")
	sb.WriteString("╚═══════════════════════════════════════════════════════════╝\n\n")

	// 确定对端模式
	var peerMode string
	if config.WGMode == "server" {
		peerMode = "client"
	} else {
		peerMode = "server"
	}

	// 1. 生成完整的命令行命令
	sb.WriteString("【对端创建命令】(复制以下完整命令到对端执行)\n\n")

	// 构建远程IP参数
	var remoteIPArg string
	if peerMode == "server" {
		// 对端是服务端，远程IP设为0.0.0.0（占位符，服务端不需要）
		remoteIPArg = "0.0.0.0"
	} else {
		// 对端是客户端，需要连接到本地IP
		remoteIPArg = config.LocalIP
	}

	// 构建完整命令
	sb.WriteString(fmt.Sprintf("twnode line create <父接口> %s %s %s %s \\\n",
		remoteIPArg,         // 对端的 remote_ip
		config.LocalVIP,     // 对端的 remote_vip 是本地的 local_vip
		config.RemoteVIP,    // 对端的 local_vip 是本地的 remote_vip
		config.Name))        // 隧道名

	sb.WriteString("  --type wireguard \\\n")
	sb.WriteString(fmt.Sprintf("  --mode %s \\\n", peerMode))

	// 对端的私钥（作为命令参数）
	sb.WriteString(fmt.Sprintf("  --private-key '%s' \\\n", peerPrivateKey))

	// 对端需要配置本地的公钥
	sb.WriteString(fmt.Sprintf("  --peer-pubkey '%s'", config.PublicKey))

	// 根据对端模式决定端口参数
	if peerMode == "server" {
		// 对端是服务端，需要指定监听端口
		peerPort := config.PeerListenPort
		if peerPort == 0 {
			peerPort = 51820 // 默认值
		}
		sb.WriteString(fmt.Sprintf(" \\\n  --listen-port %d", peerPort))
	} else {
		// 对端是客户端，需要知道本地服务端的端口
		sb.WriteString(fmt.Sprintf(" \\\n  --peer-port %d", config.ListenPort))
	}

	sb.WriteString("\n\n")

	// 2. 参数说明
	sb.WriteString("【参数说明】\n\n")
	sb.WriteString("- <父接口>: 请将 <父接口> 替换为对端实际的网络接口名 (如 eth0, ens33 等)\n")
	sb.WriteString(fmt.Sprintf("- 隧道名: %s (与本地保持一致)\n", config.Name))
	sb.WriteString(fmt.Sprintf("- 模式: %s (与本地 %s 模式互补)\n", peerMode, config.WGMode))

	if peerMode == "client" {
		sb.WriteString(fmt.Sprintf("- 对端将主动连接本地服务端: %s:%d\n", config.LocalIP, config.ListenPort))
		sb.WriteString("- 对端监听端口将自动分配\n")
	} else {
		peerPort := config.PeerListenPort
		if peerPort == 0 {
			peerPort = 51820
		}
		sb.WriteString(fmt.Sprintf("- 对端作为服务端，监听端口: %d\n", peerPort))
		sb.WriteString("- 对端等待本地客户端连接\n")
	}

	sb.WriteString("\n")

	// 3. 密钥信息（仅供查看，已包含在命令中）
	sb.WriteString("【密钥信息】(已包含在上述命令中)\n\n")
	sb.WriteString(fmt.Sprintf("对端私钥: %s\n", peerPrivateKey))
	sb.WriteString(fmt.Sprintf("对端公钥: %s\n", peerPublicKey))
	sb.WriteString(fmt.Sprintf("本地公钥: %s\n", config.PublicKey))

	sb.WriteString("\n")
	sb.WriteString("注意: 命令中已包含所有必需参数，替换 <父接口> 后可直接执行\n")

	return sb.String()
}

// SavePeerConfig 保存对端配置到文件
func SavePeerConfig(tunnelName, content string) error {
	if err := os.MkdirAll(PeerConfigDir, 0755); err != nil {
		return fmt.Errorf("创建对端配置目录失败: %w", err)
	}

	configPath := filepath.Join(PeerConfigDir, tunnelName+".txt")
	return os.WriteFile(configPath, []byte(content), 0644)
}

// checkAndCleanWireGuardConflicts 检查并清理可能冲突的 WireGuard 配置
func checkAndCleanWireGuardConflicts(interfaceName string) error {
	// 1. 检查是否有 wg-quick 服务在运行
	wgQuickService := fmt.Sprintf("wg-quick@%s", interfaceName)
	statusCmd := exec.Command("systemctl", "is-active", wgQuickService)
	if output, _ := statusCmd.CombinedOutput(); strings.TrimSpace(string(output)) == "active" {
		fmt.Printf("\n⚠️  检测到 systemd 服务冲突:\n")
		fmt.Printf("   服务 %s 正在运行\n", wgQuickService)
		fmt.Printf("\n请选择处理方式:\n")
		fmt.Printf("  [1] 停止并禁用该服务 (推荐)\n")
		fmt.Printf("  [2] 取消创建，手动处理\n")

		var choice string
		fmt.Print("\n请选择 (1 或 2): ")
		fmt.Scanln(&choice)

		if choice == "1" {
			fmt.Printf("\n正在停止服务 %s...\n", wgQuickService)
			stopCmd := exec.Command("systemctl", "stop", wgQuickService)
			if err := stopCmd.Run(); err != nil {
				return fmt.Errorf("停止服务失败: %w", err)
			}

			fmt.Printf("正在禁用服务 %s...\n", wgQuickService)
			disableCmd := exec.Command("systemctl", "disable", wgQuickService)
			disableCmd.Run() // 忽略错误（可能本来就没启用）

			fmt.Printf("✓ 服务已停止并禁用\n\n")
		} else {
			return fmt.Errorf("用户取消创建")
		}
	}

	// 2. 检查 /etc/wireguard/ 目录下是否有同名配置文件
	wgConfigPath := fmt.Sprintf("/etc/wireguard/%s.conf", interfaceName)
	if _, err := os.Stat(wgConfigPath); err == nil {
		fmt.Printf("\n⚠️  检测到 WireGuard 配置文件冲突:\n")
		fmt.Printf("   文件 %s 已存在\n", wgConfigPath)
		fmt.Printf("\n请选择处理方式:\n")
		fmt.Printf("  [1] 备份并删除该配置 (推荐)\n")
		fmt.Printf("  [2] 取消创建，手动处理\n")

		var choice string
		fmt.Print("\n请选择 (1 或 2): ")
		fmt.Scanln(&choice)

		if choice == "1" {
			// 备份配置文件
			backupPath := fmt.Sprintf("%s.backup.%d", wgConfigPath, time.Now().Unix())
			fmt.Printf("\n正在备份配置文件到: %s\n", backupPath)

			data, err := os.ReadFile(wgConfigPath)
			if err != nil {
				return fmt.Errorf("读取配置文件失败: %w", err)
			}

			if err := os.WriteFile(backupPath, data, 0600); err != nil {
				return fmt.Errorf("备份配置文件失败: %w", err)
			}

			// 删除原配置
			fmt.Printf("正在删除原配置文件...\n")
			if err := os.Remove(wgConfigPath); err != nil {
				return fmt.Errorf("删除配置文件失败: %w", err)
			}

			fmt.Printf("✓ 配置文件已备份并删除\n\n")
		} else {
			return fmt.Errorf("用户取消创建")
		}
	}

	// 3. 检查是否有其他 WireGuard 接口使用相同端口
	if interfaceExists(interfaceName) {
		// 尝试获取接口的配置信息
		wgShowCmd := exec.Command("wg", "show", interfaceName)
		if output, err := wgShowCmd.CombinedOutput(); err == nil && len(output) > 0 {
			fmt.Printf("\n⚠️  检测到接口 %s 已配置 WireGuard:\n", interfaceName)
			fmt.Printf("%s\n", string(output))
			fmt.Printf("\n将自动清理该接口并重新创建\n")
		}
	}

	return nil
}
