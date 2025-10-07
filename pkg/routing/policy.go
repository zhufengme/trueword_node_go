package routing

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/vishvananda/netlink"
	"trueword_node/pkg/network"
	"trueword_node/pkg/wireguard"
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
	CIDRs    []string // 目标CIDR列表
	From     string   // 源地址/源地址段（默认 "all"）
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
func (pm *PolicyManager) CreateGroup(name, exit string, priority int, from string) error {
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

	// 解析from参数
	parsedFrom, err := ParseFromInput(from)
	if err != nil {
		return fmt.Errorf("解析from参数失败: %w", err)
	}

	if parsedFrom != "all" {
		fmt.Printf("✓ 源限制: %s\n", parsedFrom)
	}

	pm.groups[name] = &PolicyGroup{
		Name:     name,
		Priority: priority,
		Exit:     exit,
		CIDRs:    make([]string, 0),
		From:     parsedFrom,
	}

	return nil
}

// 删除策略组
func (pm *PolicyManager) DeleteGroup(groupName string) error {
	// 检查策略组是否存在
	group := pm.groups[groupName]
	if group == nil {
		// 尝试加载
		if err := pm.LoadGroup(groupName); err != nil {
			return fmt.Errorf("策略组 %s 不存在", groupName)
		}
		group = pm.groups[groupName]
	}

	fmt.Printf("删除策略组: %s\n", groupName)

	// 检查策略是否已应用（通过检查内核中的规则）
	checkCmd := fmt.Sprintf("ip rule show pref %d", group.Priority)
	output, err := exec.Command("sh", "-c", checkCmd).CombinedOutput()

	isApplied := err == nil && len(output) > 0 && strings.Contains(string(output), fmt.Sprintf("%d:", group.Priority))

	if isApplied {
		fmt.Printf("  策略组已应用，先撤销...\n")
		if err := pm.RevokeGroup(groupName); err != nil {
			return fmt.Errorf("撤销策略组失败: %w", err)
		}
	} else {
		fmt.Printf("  策略组未应用，直接删除配置\n")
	}

	// 删除配置文件
	filePath := filepath.Join(PolicyDir, groupName+".policy")
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("  ⚠ 配置文件不存在: %s\n", filePath)
		} else {
			return fmt.Errorf("删除配置文件失败: %w", err)
		}
	} else {
		fmt.Printf("  ✓ 配置文件已删除\n")
	}

	// 从内存中移除
	delete(pm.groups, groupName)

	fmt.Printf("✓ 策略组 %s 删除完成\n", groupName)
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

// 获取默认路由出口
func (pm *PolicyManager) GetDefaultExit() string {
	return pm.defaultExit
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
	// 先同步保护路由（检测IP变化、清理僵尸规则）
	if err := SyncProtection(); err != nil {
		fmt.Printf("⚠ 警告: 同步保护路由失败: %v\n", err)
		// 不中断，继续应用策略
	}
	fmt.Println()

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

	// 2. 创建路由表并添加策略（仅应用有效的策略组）
	for _, group := range validGroups {
		if err := pm.ApplyGroup(group); err != nil {
			fmt.Printf("\n  ⚠ 应用策略组 %s 失败: %v，跳过\n", group.Name, err)
			continue
		}
		pm.appliedGroups = append(pm.appliedGroups, group.Name)
	}

	// 3. 应用默认路由(0.0.0.0/0)
	if pm.defaultExit != "" {
		if err := pm.applyDefaultRoute(); err != nil {
			return fmt.Errorf("应用默认路由失败: %w", err)
		}
	} else {
		fmt.Println("\n⚠ 未设置默认路由，将使用系统路由表")
	}

	// 4. 刷新路由缓存
	fmt.Println("\n刷新路由缓存...")
	exec.Command("ip", "route", "flush", "cache").Run()

	fmt.Println("\n✓ 策略路由应用完成")
	return nil
}

// ApplyGroup 应用单个策略组
func (pm *PolicyManager) ApplyGroup(group *PolicyGroup) error {
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

	// 策略规则管理：先添加新规则，再清理重复规则（避免中断）
	var ruleCmd string
	if group.From == "" || group.From == "all" {
		ruleCmd = fmt.Sprintf("ip rule add from all lookup %d pref %d", tableID, group.Priority)
	} else {
		ruleCmd = fmt.Sprintf("ip rule add from %s lookup %d pref %d", group.From, tableID, group.Priority)
	}

	// 添加新规则
	if err := execIPCommand(ruleCmd); err != nil {
		// 添加失败，可能是规则已存在，这是正常的，不报错
		// 但我们需要确保规则确实存在
	}

	// 清理重复规则：删除除了最后一个之外的所有相同优先级规则
	// 使用循环删除，直到只剩一个
	delCmd := fmt.Sprintf("ip rule del pref %d", group.Priority)
	for i := 0; i < 10; i++ { // 最多尝试10次，避免无限循环
		// 检查是否有多个相同优先级的规则
		checkCmd := fmt.Sprintf("ip rule show pref %d | wc -l", group.Priority)
		output, err := exec.Command("sh", "-c", checkCmd).Output()
		if err != nil {
			break
		}

		count := strings.TrimSpace(string(output))
		if count == "1" || count == "0" {
			// 只有一个或没有，停止删除
			break
		}

		// 有多个，删除一个
		execIPCommandNoError(delCmd)
	}

	// 最后验证规则是否存在
	checkCmd := fmt.Sprintf("ip rule show pref %d", group.Priority)
	output, err := exec.Command("sh", "-c", checkCmd).Output()
	if err != nil || len(output) == 0 {
		// 规则不存在，重新添加
		if err := execIPCommand(ruleCmd); err != nil {
			fmt.Printf("  ✗ 添加策略规则失败\n")
			fmt.Printf("     错误: %v\n", err)
			fmt.Printf("     命令: %s\n", ruleCmd)
			return err
		}
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

	// 策略规则管理：先添加新规则，再清理重复规则（避免中断）
	cmd = fmt.Sprintf("ip rule add from all lookup %d pref %d", tableID, PrioDefault)

	// 添加新规则
	if err := execIPCommand(cmd); err != nil {
		// 添加失败，可能是规则已存在，这是正常的
	}

	// 清理重复规则：删除除了最后一个之外的所有相同优先级规则
	delCmd := fmt.Sprintf("ip rule del pref %d", PrioDefault)
	for i := 0; i < 10; i++ {
		checkCmd := fmt.Sprintf("ip rule show pref %d | wc -l", PrioDefault)
		output, err := exec.Command("sh", "-c", checkCmd).Output()
		if err != nil {
			break
		}

		count := strings.TrimSpace(string(output))
		if count == "1" || count == "0" {
			break
		}

		execIPCommandNoError(delCmd)
	}

	// 最后验证规则是否存在
	checkCmd := fmt.Sprintf("ip rule show pref %d", PrioDefault)
	output, err := exec.Command("sh", "-c", checkCmd).Output()
	if err != nil || len(output) == 0 {
		// 规则不存在，重新添加
		if err := execIPCommand(cmd); err != nil {
			fmt.Printf("  ✗ 添加策略规则失败\n")
			fmt.Printf("     错误: %v\n", err)
			fmt.Printf("     命令: %s\n", cmd)
			return err
		}
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

// RevokeGroup 撤销单个策略组
func (pm *PolicyManager) RevokeGroup(groupName string) error {
	group := pm.groups[groupName]
	if group == nil {
		return fmt.Errorf("策略组 %s 不存在", groupName)
	}

	fmt.Printf("撤销策略组: %s\n", groupName)

	tableID := group.Priority

	// 删除规则 - 使用 pref 精确删除
	cmd := fmt.Sprintf("ip rule del pref %d", group.Priority)
	execIPCommandNoError(cmd)

	// 清空路由表
	cmd = fmt.Sprintf("ip route flush table %d", tableID)
	execIPCommandNoError(cmd)

	// 刷新缓存
	exec.Command("ip", "route", "flush", "cache").Run()

	fmt.Printf("  ✓ 策略组 %s 已撤销\n", groupName)
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

		// 添加From字段
		if group.From != "" && group.From != "all" {
			content += fmt.Sprintf("# From: %s\n", group.From)
		}

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
	var from string
	cidrs := make([]string, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "# Exit:") {
			exit = strings.TrimSpace(strings.TrimPrefix(line, "# Exit:"))
		} else if strings.HasPrefix(line, "# Priority:") {
			fmt.Sscanf(line, "# Priority: %d", &priority)
		} else if strings.HasPrefix(line, "# From:") {
			from = strings.TrimSpace(strings.TrimPrefix(line, "# From:"))
		} else if line != "" && !strings.HasPrefix(line, "#") {
			cidrs = append(cidrs, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// 如果没有From字段，默认为"all"（向后兼容）
	if from == "" {
		from = "all"
	}

	group := &PolicyGroup{
		Name:     name,
		Exit:     exit,
		Priority: priority,
		CIDRs:    cidrs,
		From:     from,
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

	// 转换为切片以便排序
	type groupInfo struct {
		name     string
		priority int
		exit     string
		cidrNum  int
		from     string
	}

	groupList := make([]groupInfo, 0, len(pm.groups))
	for _, group := range pm.groups {
		fromStr := group.From
		if fromStr == "" || fromStr == "all" {
			fromStr = "all"
		}
		groupList = append(groupList, groupInfo{
			name:     group.Name,
			priority: group.Priority,
			exit:     group.Exit,
			cidrNum:  len(group.CIDRs),
			from:     fromStr,
		})
	}

	// 按优先级从高到低排序（数字小的优先级高）
	for i := 0; i < len(groupList)-1; i++ {
		for j := i + 1; j < len(groupList); j++ {
			if groupList[i].priority > groupList[j].priority {
				groupList[i], groupList[j] = groupList[j], groupList[i]
			}
		}
	}

	// 使用 tablewriter 打印表格
	fmt.Println()
	fmt.Println("策略组列表:")
	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("组名", "优先级", "出口", "CIDR数量", "源限制")

	for _, g := range groupList {
		table.Append(
			g.name,
			strconv.Itoa(g.priority),
			g.exit,
			strconv.Itoa(g.cidrNum),
			g.from,
		)
	}

	table.Render()

	fmt.Println()
	fmt.Printf("共 %d 个策略组\n", len(groupList))
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

// GetInterfaceIPs 获取接口的所有IPv4地址段
func GetInterfaceIPs(ifaceName string) ([]string, error) {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("接口 %s 不存在: %w", ifaceName, err)
	}

	// 获取所有IPv4地址
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("获取接口地址失败: %w", err)
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("接口 %s 没有IPv4地址", ifaceName)
	}

	var cidrs []string
	for _, addr := range addrs {
		cidrs = append(cidrs, addr.IPNet.String())
	}

	return cidrs, nil
}

// ParseFromInput 解析用户输入的from参数（接口名、CIDR或IP）
func ParseFromInput(input string) (string, error) {
	// 空或"all"表示所有源
	if input == "" || input == "all" {
		return "all", nil
	}

	// 检查是否是CIDR格式
	if _, _, err := net.ParseCIDR(input); err == nil {
		return input, nil
	}

	// 检查是否是单个IP
	if ip := net.ParseIP(input); ip != nil {
		// 单个IP转为/32 CIDR
		return input + "/32", nil
	}

	// 尝试作为接口名处理
	// 1. 先检查是否是本地管理的隧道接口(P2P类型)
	tunnelConfig, err := network.LoadTunnelConfig(input)
	if err == nil && tunnelConfig != nil {
		// 是本地管理的隧道接口，使用对端VIP作为from源
		if tunnelConfig.RemoteVIP != "" {
			fmt.Printf("注意: 接口 %s 是P2P隧道，使用对端VIP: %s\n", input, tunnelConfig.RemoteVIP)
			return tunnelConfig.RemoteVIP + "/32", nil
		}
		return "", fmt.Errorf("隧道 %s 没有配置对端VIP", input)
	}

	// 2. 检查是否是物理接口
	cidrs, err := GetInterfaceIPs(input)
	if err == nil && len(cidrs) > 0 {
		// 是物理接口，使用本端IP
		if len(cidrs) > 1 {
			fmt.Printf("注意: 接口 %s 有多个IP地址，使用第一个: %s\n", input, cidrs[0])
		}
		return cidrs[0], nil
	}

	// 3. 检查接口是否存在于系统中（可能是第三方隧道）
	if network.IsInterfaceUp(input) {
		// 接口存在但无法自动获取对端IP（第三方隧道）
		return "", fmt.Errorf("接口 %s 是第三方隧道或P2P接口，无法自动获取对端IP，请手动输入对端IP地址", input)
	}

	// 4. 接口不存在
	return "", fmt.Errorf("无法识别输入 '%s': 不是有效的CIDR、IP或接口名", input)
}

// ExitScore 出口评分结果
type ExitScore struct {
	Name        string
	Latency     float64
	PacketLoss  float64
	BaseScore   float64 // 基础评分 (丢包+延迟)
	Cost        int     // 成本 (0-100)
	CostPenalty float64 // 成本惩罚
	Score       float64 // 最终评分 = 基础评分 - 成本惩罚
	Available   bool
	Reason      string
}

// calculateBaseScore 计算基础评分（不含成本）
func calculateBaseScore(latency, packetLoss float64) float64 {
	var score float64

	// 丢包率评分（0-60分）
	if packetLoss == 0 {
		score += 60
	} else if packetLoss <= 5 {
		score += 45
	} else if packetLoss <= 10 {
		score += 30
	} else if packetLoss <= 20 {
		score += 15
	} else {
		score += 0
	}

	// 延迟评分（0-40分）
	if latency < 50 {
		score += 40
	} else if latency < 100 {
		score += 35
	} else if latency < 150 {
		score += 30
	} else if latency < 200 {
		score += 25
	} else if latency < 300 {
		score += 15
	} else {
		score += 5
	}

	return score
}

// calculateFinalScore 计算最终评分（基础评分 - 成本惩罚）
func calculateFinalScore(baseScore float64, cost int) (float64, float64) {
	// 成本惩罚 = cost * 0.5
	costPenalty := float64(cost) * 0.5
	finalScore := baseScore - costPenalty

	// 确保最终评分不为负数
	if finalScore < 0 {
		finalScore = 0
	}

	return finalScore, costPenalty
}

// SelectBestExit 从候选出口中选择最佳出口
func SelectBestExit(candidates []string, checkResults *network.AllCheckResults) (*ExitScore, []*ExitScore, error) {
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("候选出口列表为空")
	}

	scores := make([]*ExitScore, 0, len(candidates))
	var bestExit *ExitScore

	for _, candidate := range candidates {
		exitScore := &ExitScore{
			Name:      candidate,
			Available: false,
		}

		// 检查是否有检查结果
		result, ok := checkResults.Results[candidate]
		if !ok || result == nil {
			exitScore.Reason = "未检查"
			scores = append(scores, exitScore)
			continue
		}

		// 检查状态
		if result.Status != "UP" {
			exitScore.Reason = result.Status
			scores = append(scores, exitScore)
			continue
		}

		// 获取出口成本
		cost := getExitCost(candidate)

		// 计算评分
		exitScore.Latency = result.Latency
		exitScore.PacketLoss = result.PacketLoss
		exitScore.Cost = cost
		exitScore.BaseScore = calculateBaseScore(result.Latency, result.PacketLoss)
		exitScore.Score, exitScore.CostPenalty = calculateFinalScore(exitScore.BaseScore, cost)
		exitScore.Available = true

		scores = append(scores, exitScore)

		// 选择最佳
		if bestExit == nil {
			bestExit = exitScore
		} else if exitScore.Score > bestExit.Score {
			bestExit = exitScore
		} else if exitScore.Score == bestExit.Score && exitScore.Latency < bestExit.Latency {
			// 分数相同，选择延迟更低的
			bestExit = exitScore
		}
	}

	if bestExit == nil {
		return nil, scores, fmt.Errorf("所有候选出口均不可用")
	}

	return bestExit, scores, nil
}

// getExitCost 获取出口的成本值
func getExitCost(exitName string) int {
	// 先尝试作为隧道加载
	tunnelConfig, err := network.LoadTunnelConfig(exitName)
	if err == nil && tunnelConfig != nil {
		return tunnelConfig.Cost
	}

	// 再尝试作为物理接口加载
	ifaceConfig, err := network.LoadInterfaceConfig()
	if err == nil {
		iface := ifaceConfig.GetInterfaceByName(exitName)
		if iface != nil {
			return iface.Cost
		}
	}

	// 默认成本为0
	return 0
}

// checkCandidates 检查候选出口连通性并保存结果
func checkCandidates(candidates []string, checkIP string) error {
	fmt.Println("检查出口连通性...")

	// 加载现有检查结果
	allResults, err := network.LoadCheckResults()
	if err != nil {
		allResults = &network.AllCheckResults{
			Results: make(map[string]*network.CheckResult),
		}
	}

	// 检查每个候选出口
	for _, candidate := range candidates {
		fmt.Printf("  检查 %s -> %s ... ", candidate, checkIP)

		result := network.CheckInterface(candidate, []string{checkIP})
		if result == nil {
			fmt.Printf("✗ (检查失败)\n")
			continue
		}

		// 保存结果到 allResults
		allResults.Results[candidate] = result

		if result.Status == "UP" {
			fmt.Printf("✓ (%.1fms, %.0f%% 丢包)\n", result.Latency, result.PacketLoss)
		} else {
			fmt.Printf("✗ (%s)\n", result.Status)
		}
	}

	// 保存检查结果（SaveCheckResults 会自动更新时间戳）
	if err := network.SaveCheckResults(allResults); err != nil {
		return fmt.Errorf("保存检查结果失败: %w", err)
	}

	fmt.Println()
	return nil
}

// printScores 打印评分结果
func printScores(scores []*ExitScore, bestExit *ExitScore) {
	fmt.Println("候选出口评分:")

	for _, score := range scores {
		if score.Available {
			best := ""
			if bestExit != nil && score.Name == bestExit.Name {
				best = " \033[93m★ 最佳\033[0m"
			}

			// 显示详细评分信息：基础评分、成本、成本惩罚、最终评分
			if score.Cost > 0 {
				fmt.Printf("  \033[1m%-10s\033[0m | 延迟: \033[96m%-8.1fms\033[0m 丢包: \033[96m%-5.0f%%\033[0m | 基础评分: \033[94m%.1f\033[0m Cost: \033[33m%d\033[0m 惩罚: \033[91m-%.1f\033[0m → 最终: \033[92m%.1f\033[0m%s\n",
					score.Name, score.Latency, score.PacketLoss, score.BaseScore, score.Cost, score.CostPenalty, score.Score, best)
			} else {
				// 成本为0时,简化显示
				fmt.Printf("  \033[1m%-10s\033[0m | 延迟: \033[96m%-8.1fms\033[0m 丢包: \033[96m%-5.0f%%\033[0m | 基础评分: \033[94m%.1f\033[0m Cost: \033[33m%d\033[0m → 最终: \033[92m%.1f\033[0m%s\n",
					score.Name, score.Latency, score.PacketLoss, score.BaseScore, score.Cost, score.Score, best)
			}
		} else {
			fmt.Printf("  \033[1m%-10s\033[0m | 状态: \033[90m%-25s\033[0m | 不可用\n",
				score.Name, score.Reason)
		}
	}
	fmt.Println()
}

// FailoverGroup 对策略组执行 failover
func (pm *PolicyManager) FailoverGroup(groupName string, candidates []string, checkIP string) error {
	fmt.Printf("准备 failover: %s -> 候选出口 [%s]\n\n", groupName, strings.Join(candidates, ", "))

	// 只加载指定的策略组
	if err := pm.LoadGroup(groupName); err != nil {
		return fmt.Errorf("策略组 %s 不存在", groupName)
	}

	group := pm.groups[groupName]
	if group == nil {
		return fmt.Errorf("策略组 %s 不存在", groupName)
	}

	// 验证候选出口是否存在
	for _, candidate := range candidates {
		if !network.IsInterfaceUp(candidate) {
			// 尝试作为隧道检查
			_, err := network.LoadTunnelConfig(candidate)
			if err != nil {
				return fmt.Errorf("候选出口 %s 不存在", candidate)
			}
		}
	}

	// 根据参数决定是否执行检查
	if checkIP != "" {
		// 提供了 check_ip，执行检查
		if err := checkCandidates(candidates, checkIP); err != nil {
			return err
		}
	} else {
		// 未提供 check_ip，使用现有检查结果
		fmt.Println("使用上次检查结果（未指定 check_ip）")
	}

	// 加载检查结果
	checkResults, err := network.LoadCheckResults()
	if err != nil {
		return fmt.Errorf("加载检查结果失败: %w (提示: 请先运行 'twnode line check <ip>' 或使用 --force 参数)", err)
	}

	// 选择最佳出口
	bestExit, scores, err := SelectBestExit(candidates, checkResults)
	if err != nil {
		return err
	}

	// 打印评分
	printScores(scores, bestExit)

	fmt.Printf("选择最佳出口: \033[92m%s\033[0m (评分: %.1f)\n\n", bestExit.Name, bestExit.Score)

	// 检查是否需要切换
	if group.Exit == bestExit.Name {
		fmt.Printf("当前出口已是最佳选择 (%s)，无需切换\n", group.Exit)
		return nil
	}

	fmt.Printf("当前出口: \033[33m%s\033[0m -> 新出口: \033[92m%s\033[0m\n\n", group.Exit, bestExit.Name)

	// 切换出口
	fmt.Printf("应用策略组 '%s' 切换...\n", groupName)
	group.Exit = bestExit.Name

	// 保存配置
	if err := pm.Save(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	fmt.Printf("  ✓ 出口已更改为 %s\n", bestExit.Name)
	fmt.Printf("  ✓ 配置已保存\n")

	// 只应用当前策略组（不影响其他策略和默认路由）
	if err := pm.ApplyGroup(group); err != nil {
		return fmt.Errorf("应用策略失败: %w", err)
	}
	fmt.Printf("  ✓ 策略已应用\n")

	// 刷新路由缓存
	exec.Command("ip", "route", "flush", "cache").Run()

	fmt.Println("\n\033[92mFailover 完成！\033[0m")
	return nil
}

// FailoverDefault 对默认路由执行 failover
func (pm *PolicyManager) FailoverDefault(candidates []string, checkIP string) error {
	fmt.Printf("准备 failover: 默认路由 -> 候选出口 [%s]\n\n", strings.Join(candidates, ", "))

	// 验证候选出口是否存在
	for _, candidate := range candidates {
		if !network.IsInterfaceUp(candidate) {
			// 尝试作为隧道检查
			_, err := network.LoadTunnelConfig(candidate)
			if err != nil {
				return fmt.Errorf("候选出口 %s 不存在", candidate)
			}
		}
	}

	// 根据参数决定是否执行检查
	if checkIP != "" {
		// 提供了 check_ip，执行检查
		if err := checkCandidates(candidates, checkIP); err != nil {
			return err
		}
	} else {
		// 未提供 check_ip，使用现有检查结果
		fmt.Println("使用上次检查结果（未指定 check_ip）")
	}

	// 加载检查结果
	checkResults, err := network.LoadCheckResults()
	if err != nil {
		return fmt.Errorf("加载检查结果失败: %w (提示: 请先运行 'twnode line check <ip>' 或使用 --force 参数)", err)
	}

	// 选择最佳出口
	bestExit, scores, err := SelectBestExit(candidates, checkResults)
	if err != nil {
		return err
	}

	// 打印评分
	printScores(scores, bestExit)

	fmt.Printf("选择最佳出口: \033[92m%s\033[0m (评分: %.1f)\n\n", bestExit.Name, bestExit.Score)

	// 检查是否需要切换
	currentExit := pm.defaultExit
	if currentExit == bestExit.Name {
		fmt.Printf("当前出口已是最佳选择 (%s)，无需切换\n", currentExit)
		return nil
	}

	if currentExit == "" {
		fmt.Printf("设置默认路由出口: \033[92m%s\033[0m\n\n", bestExit.Name)
	} else {
		fmt.Printf("当前出口: \033[33m%s\033[0m -> 新出口: \033[92m%s\033[0m\n\n", currentExit, bestExit.Name)
	}

	// 切换出口
	fmt.Println("应用默认路由切换...")
	pm.SetDefaultExit(bestExit.Name)

	// 应用默认路由
	if err := pm.ApplyDefaultRouteOnly(); err != nil {
		return fmt.Errorf("应用默认路由失败: %w", err)
	}
	fmt.Printf("  ✓ 默认路由已切换到 %s\n", bestExit.Name)
	fmt.Printf("  ✓ 配置已保存\n\n")

	fmt.Println("\033[92mFailover 完成！\033[0m")
	return nil
}

// getAllTunnelConfigs 获取所有隧道配置（GRE 和 WireGuard）
func getAllTunnelConfigs() ([]*network.TunnelConfig, error) {
	tunnelDir := "/etc/trueword_node/tunnels"

	entries, err := os.ReadDir(tunnelDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	configs := make([]*network.TunnelConfig, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 提取隧道名（去掉 .yaml 后缀）
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		tunnelName := strings.TrimSuffix(name, ".yaml")

		// 加载配置
		config, err := network.LoadTunnelConfig(tunnelName)
		if err != nil {
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// getWireGuardPeerEndpoint 包装 wireguard 包的函数
func getWireGuardPeerEndpoint(interfaceName string) string {
	return wireguard.GetWireGuardPeerEndpoint(interfaceName)
}

// getTunnelTypeDisplay 获取隧道类型的显示名称
func getTunnelTypeDisplay(tunnelType string) string {
	if tunnelType == "wireguard" {
		return "WireGuard"
	}
	return "GRE"
}

// SyncProtection 同步保护路由规则（检测、更新、清理）
// 此函数整合了保护路由的完整管理：
// 1. 检测并更新隧道对端IP变化
// 2. 添加缺失的保护路由
// 3. 清理僵尸规则（无对应隧道的保护路由）
func SyncProtection() error {
	fmt.Println("同步保护路由...")

	protectedCount := 0
	updatedCount := 0

	// 1. 加载所有隧道配置
	tunnelConfigs, err := getAllTunnelConfigs()
	if err != nil {
		return fmt.Errorf("加载隧道配置失败: %w", err)
	}

	// 建立有效IP映射（隧道名 -> 当前对端IP）
	validIPs := make(map[string]string) // IP -> 隧道名

	// 2. 遍历所有隧道，检测IP变化并更新保护路由
	for _, config := range tunnelConfigs {
		remoteIP := config.RemoteIP

		// 如果是 WireGuard 且 RemoteIP 是 0.0.0.0，尝试从运行状态获取
		if config.TunnelType == "wireguard" && (remoteIP == "" || remoteIP == "0.0.0.0") {
			if network.IsInterfaceUp(config.Name) {
				endpoint := getWireGuardPeerEndpoint(config.Name)
				if endpoint != "" {
					remoteIP = endpoint
					fmt.Printf("  ℹ 从运行状态检测到 WireGuard 隧道 %s 的对端IP: %s\n", config.Name, remoteIP)
				}
			}
		}

		// 如果仍然没有有效的远程IP，跳过
		if remoteIP == "" || remoteIP == "0.0.0.0" {
			// 如果之前有保护IP，清理旧的保护路由
			if config.ProtectedIP != "" {
				delCmd := fmt.Sprintf("ip rule del to %s lookup main pref %d", config.ProtectedIP, PrioSystem)
				execIPCommandNoError(delCmd)
				config.ProtectedIP = ""
				network.SaveTunnelConfig(config)
			}
			continue
		}

		// 记录有效IP
		validIPs[remoteIP] = config.Name

		// 检查IP是否变化
		ipChanged := false
		if config.ProtectedIP != "" && config.ProtectedIP != remoteIP {
			// IP已变化，先删除旧的保护路由
			delCmd := fmt.Sprintf("ip rule del to %s lookup main pref %d", config.ProtectedIP, PrioSystem)
			execIPCommandNoError(delCmd)
			fmt.Printf("  ⚠ %s 隧道 %s 对端IP已变化: %s → %s\n",
				getTunnelTypeDisplay(config.TunnelType), config.Name, config.ProtectedIP, remoteIP)
			ipChanged = true
			updatedCount++
		}

		// 删除当前remoteIP的旧规则（防止重复）
		delCmd := fmt.Sprintf("ip rule del to %s lookup main pref %d", remoteIP, PrioSystem)
		execIPCommandNoError(delCmd)

		// 添加规则：到远程IP的流量不走策略路由
		cmd := fmt.Sprintf("ip rule add to %s lookup main pref %d", remoteIP, PrioSystem)
		if err := execIPCommand(cmd); err != nil {
			fmt.Printf("  ⚠ 警告: 添加保护路由失败: %s\n", err)
		} else {
			if !ipChanged {
				fmt.Printf("  ✓ 保护 %s 隧道 %s 的远程IP %s\n",
					getTunnelTypeDisplay(config.TunnelType), config.Name, remoteIP)
			}
			protectedCount++

			// 更新配置文件中的 ProtectedIP
			if config.ProtectedIP != remoteIP {
				config.ProtectedIP = remoteIP
				network.SaveTunnelConfig(config)
			}
		}
	}

	// 3. 清理僵尸规则（无对应隧道的保护路由）
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ip rule show pref %d", PrioSystem))
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// 解析规则，提取保护的IP
		protectedIPs := make(map[string]bool)
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// 规则格式: 10:	from all to 1.2.3.4 lookup main
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "to" && i+1 < len(parts) {
					ip := parts[i+1]
					protectedIPs[ip] = true
					break
				}
			}
		}

		// 找出僵尸IP
		orphanedIPs := make([]string, 0)
		for ip := range protectedIPs {
			if _, valid := validIPs[ip]; !valid {
				orphanedIPs = append(orphanedIPs, ip)
			}
		}

		// 清理僵尸规则
		if len(orphanedIPs) > 0 {
			fmt.Printf("  清理 %d 个僵尸规则...\n", len(orphanedIPs))
			for _, ip := range orphanedIPs {
				delCmd := fmt.Sprintf("ip rule del to %s lookup main pref %d", ip, PrioSystem)
				if err := execIPCommand(delCmd); err == nil {
					fmt.Printf("  ✓ 已清理僵尸规则: %s\n", ip)
				}
			}
		}
	}

	// 刷新路由缓存
	exec.Command("ip", "route", "flush", "cache").Run()

	if protectedCount == 0 {
		fmt.Printf("  未找到需要保护的隧道远程IP\n")
	} else if updatedCount > 0 {
		fmt.Printf("  已更新 %d 个隧道的保护路由\n", updatedCount)
	}

	fmt.Println("✓ 保护路由同步完成")
	return nil
}
