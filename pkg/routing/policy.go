package routing

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"trueword_node/pkg/network"
)

const (
	PolicyDir = "/etc/trueword_node/policies"

	// 策略路由优先级范围
	// 保留0-99给系统关键路由（对端IP、对端VIP等）
	// 100-999给用户策略组
	// 32766是主路由表，32767是默认路由表
	PrioSystem         = 10  // 系统关键路由（对端真实IP、VIP）
	PrioUserPolicyBase = 100 // 用户策略组基础优先级
	PrioDefault        = 900 // 默认路由优先级
)

// PolicyGroup 策略组
type PolicyGroup struct {
	Name     string   // 组名
	Priority int      // 优先级
	Exit     string   // 出口（隧道名或物理接口名）
	CIDRs    []string // CIDR列表
}

// PolicyManager 策略管理器
type PolicyManager struct {
	groups        map[string]*PolicyGroup
	defaultExit   string
	appliedGroups []string // 已应用的策略组名称
}

func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		groups:        make(map[string]*PolicyGroup),
		appliedGroups: make([]string, 0),
	}
}

// 创建策略组
func (pm *PolicyManager) CreateGroup(name, exit string, priority int) error {
	if _, exists := pm.groups[name]; exists {
		return fmt.Errorf("策略组 %s 已存在", name)
	}

	if priority < PrioUserPolicyBase || priority >= PrioDefault {
		return fmt.Errorf("优先级必须在 %d-%d 之间", PrioUserPolicyBase, PrioDefault-1)
	}

	// 验证出口接口（允许物理接口、隧道、第三方接口，但不允许loopback）
	info, err := network.ValidateExitInterface(exit)
	if err != nil {
		return fmt.Errorf("出口接口验证失败: %w", err)
	}

	fmt.Printf("✓ 出口接口 %s 类型: %s, 状态: UP\n", exit, info.Type.String())

	pm.groups[name] = &PolicyGroup{
		Name:     name,
		Priority: priority,
		Exit:     exit,
		CIDRs:    make([]string, 0),
	}

	return nil
}

// 删除策略组
func (pm *PolicyManager) DeleteGroup(name string) error {
	if _, exists := pm.groups[name]; !exists {
		return fmt.Errorf("策略组 %s 不存在", name)
	}

	delete(pm.groups, name)
	return nil
}

// 添加CIDR到策略组
func (pm *PolicyManager) AddCIDR(groupName, cidr string) error {
	group, exists := pm.groups[groupName]
	if !exists {
		return fmt.Errorf("策略组 %s 不存在", groupName)
	}

	// 验证CIDR
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("无效的CIDR: %s", cidr)
	}

	group.CIDRs = append(group.CIDRs, cidr)
	return nil
}

// 从文件导入CIDR
func (pm *PolicyManager) ImportCIDRsFromFile(groupName, filePath string) error {
	group, exists := pm.groups[groupName]
	if !exists {
		return fmt.Errorf("策略组 %s 不存在", groupName)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 验证CIDR
		_, _, err := net.ParseCIDR(line)
		if err != nil {
			fmt.Printf("跳过无效CIDR: %s\n", line)
			continue
		}

		group.CIDRs = append(group.CIDRs, line)
		count++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	fmt.Printf("成功导入 %d 个CIDR到策略组 %s\n", count, groupName)
	return nil
}

// 设置默认路由出口
func (pm *PolicyManager) SetDefaultExit(exit string) {
	pm.defaultExit = exit
}

// 检查出口是否有效
func (pm *PolicyManager) checkExitValid(exit string) error {
	// 检查接口是否存在
	iface, err := net.InterfaceByName(exit)
	if err != nil {
		return fmt.Errorf("接口 %s 不存在", exit)
	}

	// 检查接口是否UP
	if iface.Flags&net.FlagUp == 0 {
		return fmt.Errorf("接口 %s 未启动", exit)
	}

	return nil
}

// 获取所有隧道接口
func getTunnelInterfaces() ([]string, error) {
	cmd := exec.Command("ip", "tunnel", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var tunnels []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) > 0 {
			tunnelName := strings.TrimSpace(parts[0])
			if tunnelName != "" && !strings.Contains(line, "remote any") {
				tunnels = append(tunnels, tunnelName)
			}
		}
	}
	return tunnels, nil
}

// 应用策略路由
func (pm *PolicyManager) Apply() error {
	fmt.Println("开始应用策略路由...")

	// 1. 检查所有出口是否有效
	fmt.Println("\n检查出口状态...")
	validGroups := make(map[string]*PolicyGroup)
	for _, group := range pm.groups {
		// 使用新的接口验证函数
		if !network.IsInterfaceUp(group.Exit) {
			fmt.Printf("  ✗ %s: 接口 %s 不存在或未启动，跳过此策略组\n", group.Name, group.Exit)
			continue
		}

		// 检测接口类型
		info, err := network.GetInterfaceInfo(group.Exit)
		if err != nil {
			fmt.Printf("  ✗ %s: 无法获取接口 %s 信息，跳过此策略组\n", group.Name, group.Exit)
			continue
		}

		// 拒绝loopback
		if info.Type == network.InterfaceTypeLoopback {
			fmt.Printf("  ✗ %s: 接口 %s 是回环接口，跳过此策略组\n", group.Name, group.Exit)
			continue
		}

		fmt.Printf("  ✓ %s: 接口 %s 正常 (类型: %s)\n", group.Name, group.Exit, info.Type.String())
		validGroups[group.Name] = group
	}

	if pm.defaultExit != "" {
		if !network.IsInterfaceUp(pm.defaultExit) {
			fmt.Printf("  ⚠ 默认出口: 接口 %s 不存在或未启动，将跳过默认路由设置\n", pm.defaultExit)
			pm.defaultExit = "" // 清空，跳过后续默认路由应用
		} else {
			info, err := network.GetInterfaceInfo(pm.defaultExit)
			if err != nil || info.Type == network.InterfaceTypeLoopback {
				fmt.Printf("  ⚠ 默认出口: 接口 %s 不可用或是回环接口，将跳过默认路由设置\n", pm.defaultExit)
				pm.defaultExit = ""
			} else {
				fmt.Printf("  ✓ 默认出口: 接口 %s 正常 (类型: %s)\n", pm.defaultExit, info.Type.String())
			}
		}
	}

	// 2. 获取所有隧道，为它们设置保护路由
	tunnels, _ := getTunnelInterfaces()
	if len(tunnels) > 0 {
		fmt.Println("\n添加系统保护路由...")
		for _, tunnel := range tunnels {
			// 获取隧道的远程IP
			remoteIP, err := getTunnelRemoteIP(tunnel)
			if err != nil || remoteIP == "" {
				continue
			}

			// 先删除旧的保护路由（如果存在）
			delCmd := fmt.Sprintf("ip rule del to %s lookup main pref %d", remoteIP, PrioSystem)
			execIPCommandNoError(delCmd)

			// 添加规则：到远程IP的流量不走策略路由
			// 这确保隧道底层连接不会被策略路由影响
			cmd := fmt.Sprintf("ip rule add to %s lookup main pref %d", remoteIP, PrioSystem)
			if err := execIPCommand(cmd); err != nil {
				fmt.Printf("  ⚠ 警告: 添加保护路由失败: %s\n", err)
			} else {
				fmt.Printf("  ✓ 保护隧道 %s 的远程IP %s\n", tunnel, remoteIP)
			}
		}
	}

	// 3. 创建路由表并添加策略（仅应用有效的策略组）
	for _, group := range validGroups {
		if err := pm.applyGroup(group); err != nil {
			fmt.Printf("\n  ⚠ 应用策略组 %s 失败: %v，跳过\n", group.Name, err)
			continue
		}
		pm.appliedGroups = append(pm.appliedGroups, group.Name)
	}

	// 4. 应用默认路由(0.0.0.0/0)
	if pm.defaultExit != "" {
		if err := pm.applyDefaultRoute(); err != nil {
			return fmt.Errorf("应用默认路由失败: %w", err)
		}
	} else {
		fmt.Println("\n⚠ 未设置默认路由，将使用系统路由表")
	}

	// 5. 刷新路由缓存
	fmt.Println("\n刷新路由缓存...")
	exec.Command("ip", "route", "flush", "cache").Run()

	fmt.Println("\n✓ 策略路由应用完成")
	return nil
}

// 应用单个策略组
func (pm *PolicyManager) applyGroup(group *PolicyGroup) error {
	tableID := group.Priority

	fmt.Printf("\n应用策略组: %s\n", group.Name)
	fmt.Printf("  出口接口: %s\n", group.Exit)
	fmt.Printf("  优先级: %d\n", group.Priority)

	// 清空路由表
	cmd := fmt.Sprintf("ip route flush table %d", tableID)
	execIPCommand(cmd)

	// 获取接口信息以决定路由命令
	info, err := network.GetInterfaceInfo(group.Exit)
	if err != nil {
		return fmt.Errorf("无法获取接口信息: %w", err)
	}

	// 添加路由到表
	successCount := 0
	for _, cidr := range group.CIDRs {
		var cmd string

		// 根据接口类型决定路由命令
		if info.Type == network.InterfaceTypePhysical && info.Gateway != "" {
			// 物理接口有网关：通过网关路由
			cmd = fmt.Sprintf("ip route add %s via %s dev %s table %d", cidr, info.Gateway, group.Exit, tableID)
		} else if info.Type == network.InterfaceTypeThirdParty {
			// 第三方接口：尝试获取网关
			gateway := network.GetGatewayFromRoutes(group.Exit)
			if gateway != "" {
				cmd = fmt.Sprintf("ip route add %s via %s dev %s table %d", cidr, gateway, group.Exit, tableID)
			} else {
				// 无网关，直接通过设备
				cmd = fmt.Sprintf("ip route add %s dev %s table %d", cidr, group.Exit, tableID)
			}
		} else {
			// 隧道或无网关的P2P连接：直接通过设备
			cmd = fmt.Sprintf("ip route add %s dev %s table %d", cidr, group.Exit, tableID)
		}

		if err := execIPCommand(cmd); err != nil {
			fmt.Printf("  ✗ IP: %s, 出口: %s - 失败\n", cidr, group.Exit)
			fmt.Printf("     错误: %v\n", err)
			fmt.Printf("     命令: %s\n", cmd)
		} else {
			fmt.Printf("  ✓ IP: %s, 出口: %s\n", cidr, group.Exit)
			successCount++
		}
	}

	// 删除旧的规则（如果存在）- 使用 pref 精确删除
	cmd = fmt.Sprintf("ip rule del pref %d", group.Priority)
	execIPCommandNoError(cmd)

	// 添加策略规则
	cmd = fmt.Sprintf("ip rule add from all lookup %d pref %d", tableID, group.Priority)
	if err := execIPCommand(cmd); err != nil {
		fmt.Printf("  ✗ 添加策略规则失败\n")
		fmt.Printf("     错误: %v\n", err)
		fmt.Printf("     命令: %s\n", cmd)
		return err
	}

	fmt.Printf("  ✓ 策略组应用完成: 成功 %d/%d 个CIDR\n", successCount, len(group.CIDRs))

	return nil
}

// ApplyDefaultRouteOnly 只应用默认路由（不影响其他策略组）
func (pm *PolicyManager) ApplyDefaultRouteOnly() error {
	if pm.defaultExit == "" {
		return fmt.Errorf("未设置默认路由出口")
	}

	fmt.Println("应用默认路由...")

	// 验证接口
	if !network.IsInterfaceUp(pm.defaultExit) {
		return fmt.Errorf("接口 %s 不存在或未启动", pm.defaultExit)
	}

	info, err := network.GetInterfaceInfo(pm.defaultExit)
	if err != nil {
		return fmt.Errorf("无法获取接口信息: %w", err)
	}

	if info.Type == network.InterfaceTypeLoopback {
		return fmt.Errorf("不能使用回环接口作为默认路由出口")
	}

	fmt.Printf("  出口接口: %s (类型: %s)\n", pm.defaultExit, info.Type.String())

	return pm.applyDefaultRoute()
}

// RevokeDefaultRouteOnly 只撤销默认路由（不影响其他策略组）
func (pm *PolicyManager) RevokeDefaultRouteOnly() error {
	fmt.Println("撤销默认路由...")

	// 删除规则 - 使用 pref 精确删除
	cmd := fmt.Sprintf("ip rule del pref %d", PrioDefault)
	execIPCommandNoError(cmd)

	// 清空路由表
	cmd = fmt.Sprintf("ip route flush table %d", PrioDefault)
	execIPCommandNoError(cmd)

	// 刷新缓存
	exec.Command("ip", "route", "flush", "cache").Run()

	fmt.Printf("  ✓ 默认路由已撤销\n")
	return nil
}

// 应用默认路由(0.0.0.0/0)
func (pm *PolicyManager) applyDefaultRoute() error {
	tableID := PrioDefault

	fmt.Printf("\n应用默认路由\n")
	fmt.Printf("  IP: 0.0.0.0/0\n")
	fmt.Printf("  出口接口: %s\n", pm.defaultExit)
	fmt.Printf("  优先级: %d\n", PrioDefault)

	// 清空路由表
	cmd := fmt.Sprintf("ip route flush table %d", tableID)
	execIPCommand(cmd)

	// 获取接口信息以决定路由命令
	info, err := network.GetInterfaceInfo(pm.defaultExit)
	if err != nil {
		return fmt.Errorf("无法获取接口信息: %w", err)
	}

	// 添加 0.0.0.0/0 路由
	var routeCmd string
	if info.Type == network.InterfaceTypePhysical && info.Gateway != "" {
		// 物理接口有网关：通过网关路由
		routeCmd = fmt.Sprintf("ip route add 0.0.0.0/0 via %s dev %s table %d", info.Gateway, pm.defaultExit, tableID)
	} else if info.Type == network.InterfaceTypeThirdParty {
		// 第三方接口：尝试获取网关
		gateway := network.GetGatewayFromRoutes(pm.defaultExit)
		if gateway != "" {
			routeCmd = fmt.Sprintf("ip route add 0.0.0.0/0 via %s dev %s table %d", gateway, pm.defaultExit, tableID)
		} else {
			// 无网关，直接通过设备
			routeCmd = fmt.Sprintf("ip route add 0.0.0.0/0 dev %s table %d", pm.defaultExit, tableID)
		}
	} else {
		// 隧道或无网关的P2P连接：直接通过设备
		routeCmd = fmt.Sprintf("ip route add 0.0.0.0/0 dev %s table %d", pm.defaultExit, tableID)
	}

	if err := execIPCommand(routeCmd); err != nil {
		fmt.Printf("  ✗ 添加默认路由失败\n")
		fmt.Printf("     错误: %v\n", err)
		fmt.Printf("     命令: %s\n", routeCmd)
		return err
	}

	// 删除旧的规则 - 使用 pref 精确删除
	cmd = fmt.Sprintf("ip rule del pref %d", PrioDefault)
	execIPCommandNoError(cmd)

	// 添加规则
	cmd = fmt.Sprintf("ip rule add from all lookup %d pref %d", tableID, PrioDefault)
	if err := execIPCommand(cmd); err != nil {
		fmt.Printf("  ✗ 添加策略规则失败\n")
		fmt.Printf("     错误: %v\n", err)
		fmt.Printf("     命令: %s\n", cmd)
		return err
	}

	fmt.Printf("  ✓ 默认路由应用完成\n")
	return nil
}

// 撤销策略路由
func (pm *PolicyManager) Revoke() error {
	fmt.Println("撤销策略路由...")

	// 1. 删除系统保护路由
	tunnels, _ := getTunnelInterfaces()
	for _, tunnel := range tunnels {
		remoteIP, _ := getTunnelRemoteIP(tunnel)
		if remoteIP != "" {
			cmd := fmt.Sprintf("ip rule del to %s lookup main pref %d", remoteIP, PrioSystem)
			execIPCommandNoError(cmd)
		}
	}

	// 2. 删除策略组
	for _, group := range pm.groups {
		tableID := group.Priority

		// 删除规则 - 使用 pref 精确删除
		cmd := fmt.Sprintf("ip rule del pref %d", group.Priority)
		execIPCommandNoError(cmd)

		// 清空路由表
		cmd = fmt.Sprintf("ip route flush table %d", tableID)
		execIPCommandNoError(cmd)

		fmt.Printf("  ✓ 已撤销策略组: %s\n", group.Name)
	}

	// 3. 删除默认路由
	if pm.defaultExit != "" {
		// 删除规则 - 使用 pref 精确删除
		cmd := fmt.Sprintf("ip rule del pref %d", PrioDefault)
		execIPCommandNoError(cmd)

		cmd = fmt.Sprintf("ip route flush table %d", PrioDefault)
		execIPCommandNoError(cmd)

		fmt.Printf("  ✓ 已撤销默认路由\n")
	}

	// 4. 刷新缓存
	exec.Command("ip", "route", "flush", "cache").Run()

	pm.appliedGroups = make([]string, 0)
	fmt.Println("✓ 策略路由撤销完成")
	return nil
}

// 保存策略到文件
func (pm *PolicyManager) Save() error {
	if err := os.MkdirAll(PolicyDir, 0755); err != nil {
		return err
	}

	for _, group := range pm.groups {
		filePath := filepath.Join(PolicyDir, group.Name+".policy")
		content := fmt.Sprintf("# Policy Group: %s\n", group.Name)
		content += fmt.Sprintf("# Exit: %s\n", group.Exit)
		content += fmt.Sprintf("# Priority: %d\n", group.Priority)
		content += "\n"
		content += strings.Join(group.CIDRs, "\n")

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// 从文件加载策略组
func (pm *PolicyManager) LoadGroup(name string) error {
	filePath := filepath.Join(PolicyDir, name+".policy")

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var exit string
	var priority int
	cidrs := make([]string, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "# Exit:") {
			exit = strings.TrimSpace(strings.TrimPrefix(line, "# Exit:"))
		} else if strings.HasPrefix(line, "# Priority:") {
			fmt.Sscanf(line, "# Priority: %d", &priority)
		} else if line != "" && !strings.HasPrefix(line, "#") {
			cidrs = append(cidrs, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	group := &PolicyGroup{
		Name:     name,
		Exit:     exit,
		Priority: priority,
		CIDRs:    cidrs,
	}

	pm.groups[name] = group
	return nil
}

// 获取策略组
func (pm *PolicyManager) GetGroup(name string) *PolicyGroup {
	return pm.groups[name]
}

// 列出所有策略组
func (pm *PolicyManager) ListGroups() {
	if len(pm.groups) == 0 {
		fmt.Println("没有策略组")
		return
	}

	fmt.Println("策略组列表:")
	for _, group := range pm.groups {
		fmt.Printf("\n  组名: %s\n", group.Name)
		fmt.Printf("  出口: %s\n", group.Exit)
		fmt.Printf("  优先级: %d\n", group.Priority)
		fmt.Printf("  CIDR数量: %d\n", len(group.CIDRs))
	}
}

// 工具函数
func execIPCommand(cmd string) error {
	parts := strings.Fields(cmd)
	command := exec.Command(parts[0], parts[1:]...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func execIPCommandNoError(cmd string) {
	parts := strings.Fields(cmd)
	command := exec.Command(parts[0], parts[1:]...)
	command.Run()
}

// 获取隧道的远程IP
func getTunnelRemoteIP(tunnelName string) (string, error) {
	cmd := exec.Command("ip", "tunnel", "show", tunnelName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	line := string(output)
	if strings.Contains(line, "remote") {
		parts := strings.Fields(line)
		for i, part := range parts {
			if part == "remote" && i+1 < len(parts) {
				return parts[i+1], nil
			}
		}
	}

	return "", nil
}

// 获取接口的网关
func getInterfaceGateway(ifaceName string) (string, error) {
	cmd := exec.Command("ip", "route", "show", "dev", ifaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "via") && strings.Contains(line, "default") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "via" && i+1 < len(parts) {
					return parts[i+1], nil
				}
			}
		}
	}

	// 如果没找到，尝试从主路由表找
	cmd = exec.Command("ip", "route")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	lines = strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "default") && strings.Contains(line, ifaceName) {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "via" && i+1 < len(parts) {
					return parts[i+1], nil
				}
			}
		}
	}

	return "", nil
}
