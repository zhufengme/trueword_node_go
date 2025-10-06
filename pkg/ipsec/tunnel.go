package ipsec

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"trueword_node/pkg/network"
)

const (
	RevDir = "/var/lib/trueword_node/rev"
)

type Tunnel struct {
	Name            string
	LocalIP         string
	RemoteIP        string
	LocalVirtualIP  string
	RemoteVirtualIP string
	GREKey          uint32 // GRE密钥
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
		// 出错时显示命令和错误输出
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

// 生成SPI
func generateSPI(ip1, ip2 string) string {
	data := ip1 + ip2
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])[:8]
}

// 生成GRE Key (从auth密钥字符串生成)
func generateGREKey(authKey string) uint32 {
	// 去掉0x前缀
	authKey = strings.TrimPrefix(authKey, "0x")

	// 计算所有字符的ASCII值之和
	var sum uint32
	for _, c := range authKey {
		sum += uint32(c)
	}

	return sum
}

// 获取较大和较小的IP（用于保证两端生成相同的SPI）
func sortIPs(ip1, ip2 string) (string, string) {
	ipA := net.ParseIP(ip1)
	ipB := net.ParseIP(ip2)

	if ipA == nil || ipB == nil {
		return ip1, ip2
	}

	// 比较IP
	for i := 0; i < len(ipA); i++ {
		if ipA[i] > ipB[i] {
			return ip1, ip2
		} else if ipA[i] < ipB[i] {
			return ip2, ip1
		}
	}
	return ip1, ip2
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

// 创建IPsec连接
func CreateIPsec(localIP, remoteIP, authKey, encKey string) error {
	// 检查本地IP是否存在
	localExists, err := isIPLocal(localIP)
	if err != nil {
		return err
	}

	remoteExists, err := isIPLocal(remoteIP)
	if err != nil {
		return err
	}

	if !localExists && !remoteExists {
		return fmt.Errorf("❌ 本地或远程IP必须存在于本地接口")
	}

	// 确定本地和远程IP
	var actualLocalIP, actualRemoteIP string
	if localExists {
		actualLocalIP = localIP
		actualRemoteIP = remoteIP
	} else {
		actualLocalIP = remoteIP
		actualRemoteIP = localIP
	}

	// 生成SPI
	ipOne, ipTwo := sortIPs(localIP, remoteIP)
	spiOne := generateSPI(ipOne, ipTwo)
	spiTwo := generateSPI(ipTwo, ipOne)

	// 撤销文件名
	revFile := fmt.Sprintf("%s-%s.rev", ipOne, ipTwo)

	// 先清理旧配置
	executeRevCommands(revFile)

	// 记录撤销命令
	revCommands := []string{
		fmt.Sprintf("ip xfrm policy del src %s dst %s dir out", actualLocalIP, actualRemoteIP),
		fmt.Sprintf("ip xfrm policy del src %s dst %s dir in", actualRemoteIP, actualLocalIP),
		fmt.Sprintf("ip xfrm state del src %s dst %s proto esp spi 0x%s", ipOne, ipTwo, spiOne),
		fmt.Sprintf("ip xfrm state del src %s dst %s proto esp spi 0x%s", ipTwo, ipOne, spiTwo),
	}
	recordRevCommands(revFile, revCommands)

	// 添加xfrm state
	cmd := fmt.Sprintf("ip xfrm state add src %s dst %s proto esp spi 0x%s mode tunnel auth sha256 %s enc aes %s",
		ipOne, ipTwo, spiOne, authKey, encKey)
	if err := execCommand(cmd); err != nil {
		return err
	}

	cmd = fmt.Sprintf("ip xfrm state add src %s dst %s proto esp spi 0x%s mode tunnel auth sha256 %s enc aes %s",
		ipTwo, ipOne, spiTwo, authKey, encKey)
	if err := execCommand(cmd); err != nil {
		return err
	}

	// 添加xfrm policy
	cmd = fmt.Sprintf("ip xfrm policy add src %s dst %s dir out ptype main tmpl src %s dst %s proto esp mode tunnel",
		actualLocalIP, actualRemoteIP, actualLocalIP, actualRemoteIP)
	if err := execCommand(cmd); err != nil {
		return err
	}

	cmd = fmt.Sprintf("ip xfrm policy add src %s dst %s dir in ptype main tmpl src %s dst %s proto esp mode tunnel",
		actualRemoteIP, actualLocalIP, actualRemoteIP, actualLocalIP)
	if err := execCommand(cmd); err != nil {
		return err
	}

	fmt.Printf("   ✓ IPsec加密隧道已建立\n")

	// 测试连通性
	if pingHost(actualRemoteIP, 3) {
		fmt.Printf("   ✓ 加密连接验证成功 (%s <-> %s)\n", actualLocalIP, actualRemoteIP)
		return nil
	} else {
		fmt.Printf("   ⚠️  加密隧道已建立但未连接\n")
		fmt.Printf("      等待远程节点 %s 建立连接...\n", actualRemoteIP)
		return nil
	}
}

// 删除IPsec连接
func RemoveIPsec(ip1, ip2 string) error {
	ipOne, ipTwo := sortIPs(ip1, ip2)
	revFile := fmt.Sprintf("%s-%s.rev", ipOne, ipTwo)

	if err := executeRevCommands(revFile); err != nil {
		return fmt.Errorf("❌ 删除IPsec连接失败: %w", err)
	}

	return nil
}

// 创建GRE隧道
func (t *Tunnel) Create() error {
	// 清理旧配置（如果存在）
	revFile := fmt.Sprintf("%s.rev", t.Name)
	executeRevCommands(revFile)

	// 再次检查并强制删除（防止之前创建失败但接口残留）
	if interfaceExists(t.Name) {
		fmt.Printf("   ⚠️  接口 %s 已存在，正在清理...\n", t.Name)
		execCommandNoError(fmt.Sprintf("ip link set dev %s down", t.Name))
		execCommandNoError(fmt.Sprintf("ip tunnel del %s", t.Name))
	}

	// 记录撤销命令
	revCommands := []string{
		fmt.Sprintf("ip link set dev %s down", t.Name),
		fmt.Sprintf("ip tunnel del %s mode gre remote %s local %s key %d ttl 255", t.Name, t.RemoteIP, t.LocalIP, t.GREKey),
		fmt.Sprintf("ip addr del %s/32 dev %s", t.LocalVirtualIP, t.Name),
		fmt.Sprintf("ip route del %s/32 dev %s table 80", t.RemoteVirtualIP, t.Name),
	}
	recordRevCommands(revFile, revCommands)

	// 创建GRE隧道 (带key参数)
	cmd := fmt.Sprintf("ip tunnel add %s mode gre remote %s local %s key %d ttl 255",
		t.Name, t.RemoteIP, t.LocalIP, t.GREKey)
	if err := execCommand(cmd); err != nil {
		return err
	}

	// 设置IP地址
	cmd = fmt.Sprintf("ip addr add %s/32 dev %s", t.LocalVirtualIP, t.Name)
	if err := execCommand(cmd); err != nil {
		return err
	}

	// 启动接口
	cmd = fmt.Sprintf("ip link set dev %s up mtu 1400", t.Name)
	if err := execCommand(cmd); err != nil {
		return err
	}

	// 确保路由规则存在 (表80用于虚拟IP路由)
	checkCmd := exec.Command("bash", "-c", "ip rule list | grep -q ^80:")
	if err := checkCmd.Run(); err != nil {
		cmd = "ip rule add from all lookup 80 pref 80"
		if err := execCommand(cmd); err != nil {
			return err
		}
	}

	// 添加路由到表80
	cmd = fmt.Sprintf("ip route add %s/32 dev %s table 80", t.RemoteVirtualIP, t.Name)
	if err := execCommand(cmd); err != nil {
		return err
	}

	fmt.Printf("   ✓ GRE隧道已创建\n")

	// 测试连通性
	if pingHost(t.RemoteVirtualIP, 3) {
		fmt.Printf("   ✓ 隧道连接成功 (%s <-> %s)\n", t.LocalVirtualIP, t.RemoteVirtualIP)
		return nil
	} else {
		fmt.Printf("   ⚠️  隧道已创建但未连接\n")
		fmt.Printf("      等待远程节点 %s 建立连接...\n", t.RemoteIP)
		return nil
	}
}

// 删除隧道
func RemoveTunnel(tunnelName string) error {
	fmt.Printf("删除隧道: %s\n", tunnelName)

	// 1. 执行撤销命令清理网络配置
	revFile := fmt.Sprintf("%s.rev", tunnelName)
	if err := executeRevCommands(revFile); err != nil {
		return fmt.Errorf("❌ 清理网络配置失败: %w", err)
	}
	fmt.Printf("  ✓ 网络配置已清理\n")

	// 2. 删除隧道配置文件
	if err := network.DeleteTunnelConfig(tunnelName); err != nil {
		return fmt.Errorf("❌ 删除配置文件失败: %w", err)
	}
	fmt.Printf("  ✓ 配置文件已删除\n")

	fmt.Printf("✓ 隧道 %s 删除完成\n", tunnelName)
	return nil
}

// 检查IP是否是本地的
func isIPLocal(ip string) (bool, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.String() == ip {
				return true, nil
			}
		}
	}
	return false, nil
}

// 检查接口是否存在
func interfaceExists(name string) bool {
	_, err := net.InterfaceByName(name)
	return err == nil
}

// Ping检查
func pingHost(host string, timeout int) bool {
	cmd := exec.Command("ping", "-c", "3", "-W", fmt.Sprintf("%d", timeout), host)
	err := cmd.Run()
	return err == nil
}

// 创建完整的线路(IPsec + GRE隧道)
func CreateLine(remoteIP, remoteVIP, localIP, localVIP, tunnelName, authKey, encKey string) error {
	// 验证IP
	if net.ParseIP(remoteIP) == nil || net.ParseIP(remoteVIP) == nil ||
		net.ParseIP(localIP) == nil || net.ParseIP(localVIP) == nil {
		return fmt.Errorf("无效的IP地址")
	}

	// 验证IP不能相同
	if localIP == remoteIP {
		return fmt.Errorf("本地IP和远程IP不能相同")
	}
	if localVIP == remoteVIP {
		return fmt.Errorf("本地虚拟IP和远程虚拟IP不能相同")
	}

	// 创建IPsec
	fmt.Println("=== 创建 IPsec 连接 ===")
	if err := CreateIPsec(localIP, remoteIP, authKey, encKey); err != nil {
		return err
	}

	// 等待一下
	time.Sleep(time.Second)

	// 创建隧道
	fmt.Println("\n=== 创建 GRE 隧道 ===")
	tunnel := &Tunnel{
		Name:            tunnelName,
		LocalIP:         localIP,
		RemoteIP:        remoteIP,
		LocalVirtualIP:  localVIP,
		RemoteVirtualIP: remoteVIP,
	}

	return tunnel.Create()
}
