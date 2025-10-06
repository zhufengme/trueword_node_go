package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	// 检查接口是否已存在
	if interfaceExists(wg.Name) {
		return fmt.Errorf("❌ 接口 %s 已存在", wg.Name)
	}

	// 清理旧配置
	revFile := fmt.Sprintf("%s.rev", wg.Name)
	executeRevCommands(revFile)

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

	// 测试连通性
	if pingHost(wg.RemoteVIP, 3) {
		fmt.Printf("   ✓ 隧道连接成功 (%s <-> %s)\n", wg.LocalVIP, wg.RemoteVIP)
		return nil
	} else {
		modeDesc := "服务端"
		if wg.Mode == "client" {
			modeDesc = "客户端"
		}
		fmt.Printf("   ⚠️  隧道已创建但未连接 (%s模式)\n", modeDesc)
		fmt.Printf("      等待远程节点 %s 建立连接...\n", wg.RemoteIP)
		return nil
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

// GeneratePeerCommand 生成对端创建命令
func GeneratePeerCommand(config *network.TunnelConfig, peerPrivateKey, peerPublicKey string) string {
	var sb strings.Builder

	sb.WriteString("╔═══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║  对端配置 (请在远程主机执行)                              ║\n")
	sb.WriteString("╚═══════════════════════════════════════════════════════════╝\n\n")

	// 确定对端模式
	var peerMode string
	if config.WGMode == "server" {
		peerMode = "client"
	} else {
		peerMode = "server"
	}

	// 1. 生成 twnode 命令
	sb.WriteString("【创建命令】\n\n")

	// 构建命令（根据模式决定是否需要远程IP参数）
	var remoteIPArg string
	if peerMode == "server" {
		// 对端是服务端，不需要指定远程IP（交互时会自动设为0.0.0.0）
		sb.WriteString("# 注意：以下命令在交互时选择服务端模式，无需输入远程IP\n")
		remoteIPArg = "# (交互时选择服务端模式)"
	} else {
		// 对端是客户端，需要知道本地IP
		remoteIPArg = config.LocalIP
	}

	sb.WriteString(fmt.Sprintf("twnode line create <父接口> %s %s %s %s \\\n",
		remoteIPArg,         // 对端的 remote_ip
		config.LocalVIP,     // 对端的 remote_vip 是本地的 local_vip
		config.RemoteVIP,    // 对端的 local_vip 是本地的 remote_vip
		config.Name))        // 隧道名

	sb.WriteString("  --type wireguard \\\n")
	sb.WriteString(fmt.Sprintf("  --mode %s \\\n", peerMode))

	// 根据对端模式决定是否需要指定端口
	if peerMode == "server" {
		// 对端是服务端，需要指定监听端口
		peerPort := config.PeerListenPort
		if peerPort == 0 {
			peerPort = 51820 // 建议默认值
		}
		sb.WriteString(fmt.Sprintf("  --listen-port %d \\\n", peerPort))
	}
	// 如果对端是客户端，不需要指定 listen-port（自动分配）

	// 对端需要知道本地的端口
	if config.WGMode == "server" {
		// 本地是服务端，对端（客户端）需要知道本地监听端口
		sb.WriteString(fmt.Sprintf("  --peer-port %d \\\n", config.ListenPort))
	}
	// 如果本地是客户端，对端是服务端，对端不需要知道本地端口（自动分配的）

	sb.WriteString(fmt.Sprintf("  --peer-pubkey %s\n", config.PublicKey))

	sb.WriteString("\n")

	// 2. 提供密钥信息
	sb.WriteString("【对端私钥】(执行命令时需要输入)\n\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", peerPrivateKey))

	// 3. 参数说明
	sb.WriteString("【参数说明】\n\n")
	sb.WriteString("- <父接口>: 请根据对端实际网络接口填写 (如 eth0, tun01 等)\n")
	sb.WriteString(fmt.Sprintf("- 模式: %s (与本地 %s 模式配对)\n", peerMode, config.WGMode))
	if peerMode == "client" {
		sb.WriteString(fmt.Sprintf("- 对端将主动连接本地服务端: %s:%d\n", config.LocalIP, config.ListenPort))
		sb.WriteString("- 对端监听端口将自动分配\n")
		sb.WriteString("- 执行命令时会要求输入远程服务端IP，填写上面显示的本地IP\n")
	} else {
		sb.WriteString("- 对端是服务端模式，交互时选择 [1] 服务端模式\n")
		sb.WriteString("- 对端无需指定远程IP（将等待本地客户端连接）\n")
		peerPort := config.PeerListenPort
		if peerPort == 0 {
			peerPort = 51820
		}
		sb.WriteString(fmt.Sprintf("- 建议对端监听端口: %d (可根据实际情况调整)\n", peerPort))
	}
	sb.WriteString("- 执行命令时会要求输入私钥，请粘贴上面的私钥\n")

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
