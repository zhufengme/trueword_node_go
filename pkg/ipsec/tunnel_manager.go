package ipsec

import (
	"fmt"
	"net"
	"time"

	"trueword_node/pkg/network"
	"trueword_node/pkg/wireguard"

	"github.com/vishvananda/netlink"
)

// TunnelManager 隧道管理器
type TunnelManager struct {
	config *network.TunnelConfig
}

// NewTunnelManager 创建隧道管理器
func NewTunnelManager(config *network.TunnelConfig) *TunnelManager {
	return &TunnelManager{
		config: config,
	}
}

// getLocalIPFromParent 从父接口获取本地IP
// 如果父接口是物理接口，返回物理接口的IP
// 如果父接口是隧道，返回隧道的LocalVIP
func getLocalIPFromParent(parentName string) (string, error) {
	// 1. 先检查是否是物理接口
	ifaceConfig, err := network.LoadInterfaceConfig()
	if err == nil {
		if iface := ifaceConfig.GetInterfaceByName(parentName); iface != nil {
			// 是物理接口，返回物理接口的IP
			return iface.IP, nil
		}
	}

	// 2. 检查是否是隧道接口
	tunnelConfig, err := network.LoadTunnelConfig(parentName)
	if err == nil {
		// 是隧道接口，返回隧道的LocalVIP (不是LocalIP!)
		return tunnelConfig.LocalVIP, nil
	}

	return "", fmt.Errorf("父接口 %s 不存在", parentName)
}

// getGatewayFromParent 从父接口获取网关
func getGatewayFromParent(parentName string) (string, error) {
	// 先尝试从物理接口配置中获取
	ifaceConfig, err := network.LoadInterfaceConfig()
	if err == nil {
		if iface := ifaceConfig.GetInterfaceByName(parentName); iface != nil {
			return iface.Gateway, nil
		}
	}

	// 如果是隧道,返回空(隧道不需要网关)
	_, err = network.LoadTunnelConfig(parentName)
	if err == nil {
		return "", nil
	}

	return "", fmt.Errorf("无法从父接口 %s 获取网关", parentName)
}

// setupPolicyRoute 设置策略路由
// 确保远程IP通过指定的物理接口路由
func setupPolicyRoute(remoteIP, parentInterface, gateway string) error {
	// 检查是否是物理接口
	ifaceConfig, err := network.LoadInterfaceConfig()
	if err != nil {
		return fmt.Errorf("加载接口配置失败: %w", err)
	}

	iface := ifaceConfig.GetInterfaceByName(parentInterface)
	if iface == nil {
		// 不是物理接口,可能是上级隧道,不需要设置策略路由
		fmt.Printf("   ✓ 策略路由: 跳过 (父接口为隧道)\n")
		return nil
	}

	// 使用路由表50进行策略路由
	const routeTable = 50

	// 检查规则是否存在
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("获取路由规则失败: %w", err)
	}

	ruleExists := false
	for _, rule := range rules {
		if rule.Table == routeTable {
			ruleExists = true
			break
		}
	}

	// 添加路由规则(如果不存在)
	if !ruleExists {
		rule := netlink.NewRule()
		rule.Table = routeTable
		rule.Priority = routeTable
		if err := netlink.RuleAdd(rule); err != nil {
			return fmt.Errorf("添加路由规则失败: %w", err)
		}
	}

	// 获取父接口
	link, err := netlink.LinkByName(parentInterface)
	if err != nil {
		return fmt.Errorf("获取接口 %s 失败: %w", parentInterface, err)
	}

	// 添加到路由表50
	_, ipNet, err := net.ParseCIDR(remoteIP + "/32")
	if err != nil {
		return fmt.Errorf("解析IP失败: %w", err)
	}

	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       ipNet,
		Table:     routeTable,
	}

	// 如果有网关,设置网关
	if gateway != "" && gateway != "0.0.0.0" {
		gw := net.ParseIP(gateway)
		if gw != nil {
			route.Gw = gw
		}
	}

	// 先删除可能存在的旧路由
	netlink.RouteDel(route)

	// 添加新路由
	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("添加策略路由失败: %w", err)
	}

	if gateway != "" {
		fmt.Printf("   ✓ 策略路由: %s -> %s (via %s)\n", remoteIP, parentInterface, gateway)
	} else {
		fmt.Printf("   ✓ 策略路由: %s -> %s (P2P)\n", remoteIP, parentInterface)
	}

	return nil
}

// removePolicyRoute 移除策略路由
func removePolicyRoute(remoteIP, parentInterface string) error {
	const routeTable = 50

	// 获取父接口
	link, err := netlink.LinkByName(parentInterface)
	if err != nil {
		// 接口不存在,忽略
		return nil
	}

	// 删除路由
	_, ipNet, err := net.ParseCIDR(remoteIP + "/32")
	if err != nil {
		return nil
	}

	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       ipNet,
		Table:     routeTable,
	}

	netlink.RouteDel(route)
	return nil
}

// Create 创建隧道(根据类型分发到IPsec或WireGuard)
func (tm *TunnelManager) Create() error {
	cfg := tm.config

	// 根据隧道类型分发
	if cfg.TunnelType == "wireguard" {
		return tm.createWireGuardTunnel()
	}

	// 默认创建 IPsec 隧道
	return tm.createIPsecTunnel()
}

// createIPsecTunnel 创建 IPsec 隧道(原有逻辑)
func (tm *TunnelManager) createIPsecTunnel() error {
	cfg := tm.config

	// 显示创建信息
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  创建隧道: %-48s║\n", cfg.Name)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. 验证父接口
	if err := network.ValidateParentInterface(cfg.ParentInterface); err != nil {
		return fmt.Errorf("❌ 父接口验证失败: %w", err)
	}

	// 2. 如果未指定本地IP,从父接口获取
	if cfg.LocalIP == "" {
		localIP, err := getLocalIPFromParent(cfg.ParentInterface)
		if err != nil {
			return err
		}
		cfg.LocalIP = localIP
	}

	// 3. 获取网关(用于策略路由)
	gateway, _ := getGatewayFromParent(cfg.ParentInterface)

	// 显示配置信息
	fmt.Println("【配置信息】")
	fmt.Printf("  父接口:     %s\n", cfg.ParentInterface)
	fmt.Printf("  本地IP:     %s (自动获取)\n", cfg.LocalIP)
	if gateway != "" {
		fmt.Printf("  网关:       %s\n", gateway)
	}
	fmt.Printf("  远程IP:     %s\n", cfg.RemoteIP)
	fmt.Printf("  本地VIP:    %s\n", cfg.LocalVIP)
	fmt.Printf("  远程VIP:    %s\n", cfg.RemoteVIP)
	if cfg.UseEncryption {
		fmt.Printf("  加密:       已启用 (IPsec ESP)\n")
	} else {
		fmt.Printf("  加密:       未启用\n")
	}
	fmt.Println()

	// 4. 设置策略路由(确保远程IP通过正确的物理接口)
	fmt.Println("【建立连接】")
	if err := setupPolicyRoute(cfg.RemoteIP, cfg.ParentInterface, gateway); err != nil {
		return fmt.Errorf("❌ 设置策略路由失败: %w", err)
	}

	// 5. 创建IPsec连接(如果启用加密)
	if cfg.UseEncryption {
		if err := CreateIPsec(cfg.LocalIP, cfg.RemoteIP, cfg.AuthKey, cfg.EncKey); err != nil {
			removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
			return fmt.Errorf("❌ 创建IPsec失败: %w", err)
		}
		time.Sleep(time.Second)
	}

	// 6. 创建GRE隧道
	greKey := generateGREKey(cfg.AuthKey)
	tunnel := &Tunnel{
		Name:            cfg.Name,
		LocalIP:         cfg.LocalIP,
		RemoteIP:        cfg.RemoteIP,
		LocalVirtualIP:  cfg.LocalVIP,
		RemoteVirtualIP: cfg.RemoteVIP,
		GREKey:          greKey,
	}

	if err := tunnel.Create(); err != nil {
		// 失败时清理
		if cfg.UseEncryption {
			RemoveIPsec(cfg.LocalIP, cfg.RemoteIP)
		}
		removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
		return fmt.Errorf("❌ 创建GRE隧道失败: %w", err)
	}

	// 7. 保存配置
	if err := network.SaveTunnelConfig(cfg); err != nil {
		fmt.Printf("   ⚠️  配置保存失败: %v\n", err)
	} else {
		fmt.Printf("   ✓ 配置已保存\n")
	}

	// 成功提示
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  ✓ 隧道 %-49s 创建成功! ║\n", cfg.Name)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}

// createWireGuardTunnel 创建 WireGuard 隧道
func (tm *TunnelManager) createWireGuardTunnel() error {
	cfg := tm.config

	// 显示创建信息
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  创建隧道: %-48s║\n", cfg.Name)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. 验证父接口
	if err := network.ValidateParentInterface(cfg.ParentInterface); err != nil {
		return fmt.Errorf("❌ 父接口验证失败: %w", err)
	}

	// 2. 如果未指定本地IP,从父接口获取
	if cfg.LocalIP == "" {
		localIP, err := getLocalIPFromParent(cfg.ParentInterface)
		if err != nil {
			return err
		}
		cfg.LocalIP = localIP
	}

	// 3. 获取网关(用于策略路由)
	gateway, _ := getGatewayFromParent(cfg.ParentInterface)

	// 显示配置信息
	fmt.Println("【配置信息】")
	fmt.Printf("  父接口:     %s\n", cfg.ParentInterface)
	fmt.Printf("  本地IP:     %s (自动获取)\n", cfg.LocalIP)
	if gateway != "" {
		fmt.Printf("  网关:       %s\n", gateway)
	}
	if cfg.WGMode == "client" {
		// 客户端模式显示远程IP
		fmt.Printf("  远程IP:     %s\n", cfg.RemoteIP)
	}
	fmt.Printf("  本地VIP:    %s\n", cfg.LocalVIP)
	fmt.Printf("  远程VIP:    %s\n", cfg.RemoteVIP)
	fmt.Printf("  类型:       WireGuard (%s模式)\n", cfg.WGMode)
	if cfg.ListenPort > 0 {
		fmt.Printf("  监听端口:   %d\n", cfg.ListenPort)
	} else {
		fmt.Printf("  监听端口:   自动分配\n")
	}
	fmt.Println()

	// 4. 设置策略路由(确保远程IP通过正确的物理接口)
	fmt.Println("【建立连接】")
	// WireGuard 服务端模式不需要策略路由（RemoteIP 为 0.0.0.0）
	if cfg.RemoteIP != "0.0.0.0" {
		if err := setupPolicyRoute(cfg.RemoteIP, cfg.ParentInterface, gateway); err != nil {
			return fmt.Errorf("❌ 设置策略路由失败: %w", err)
		}
	} else {
		fmt.Printf("   ✓ 策略路由: 跳过 (服务端模式)\n")
	}

	// 5. 创建 WireGuard 隧道
	wgTunnel := &wireguard.WireGuardTunnel{
		Name:           cfg.Name,
		Mode:           cfg.WGMode,
		LocalIP:        cfg.LocalIP,
		RemoteIP:       cfg.RemoteIP,
		LocalVIP:       cfg.LocalVIP,
		RemoteVIP:      cfg.RemoteVIP,
		PrivateKey:     cfg.PrivateKey,
		PeerPublicKey:  cfg.PeerPublicKey,
		ListenPort:     cfg.ListenPort,
		PeerListenPort: cfg.PeerListenPort,
	}

	if err := wgTunnel.Create(); err != nil {
		removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
		return fmt.Errorf("❌ 创建WireGuard隧道失败: %w", err)
	}

	// 6. 保存配置
	if err := network.SaveTunnelConfig(cfg); err != nil {
		fmt.Printf("   ⚠️  配置保存失败: %v\n", err)
	} else {
		fmt.Printf("   ✓ 配置已保存\n")
	}

	// 成功提示
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  ✓ 隧道 %-49s 创建成功! ║\n", cfg.Name)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}

// Remove 删除隧道
func (tm *TunnelManager) Remove() error {
	cfg := tm.config

	fmt.Println()
	fmt.Printf("正在删除隧道: %s\n", cfg.Name)

	// 根据类型删除隧道
	if cfg.TunnelType == "wireguard" {
		// 删除 WireGuard 隧道
		if err := wireguard.RemoveTunnel(cfg.Name); err != nil {
			fmt.Printf("   ⚠️  删除WireGuard隧道失败: %v\n", err)
		}
	} else {
		// 删除 GRE 隧道
		if err := RemoveTunnel(cfg.Name); err != nil {
			fmt.Printf("   ⚠️  删除GRE隧道失败: %v\n", err)
		}

		// 删除IPsec连接(如果启用了加密)
		if cfg.UseEncryption {
			if err := RemoveIPsec(cfg.LocalIP, cfg.RemoteIP); err != nil {
				fmt.Printf("   ⚠️  删除IPsec失败: %v\n", err)
			}
		}
	}

	// 删除策略路由（WireGuard 服务端模式无需删除）
	if cfg.RemoteIP != "0.0.0.0" {
		if err := removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface); err != nil {
			fmt.Printf("   ⚠️  删除策略路由失败: %v\n", err)
		}
	}

	// 删除配置文件
	if err := network.DeleteTunnelConfig(cfg.Name); err != nil {
		fmt.Printf("   ⚠️  删除配置文件失败: %v\n", err)
	} else {
		fmt.Printf("   ✓ 配置文件已删除\n")
	}

	fmt.Println()
	fmt.Printf("✓ 隧道 %s 删除完成\n", cfg.Name)
	fmt.Println()
	return nil
}

// checkInterfaceUp 检查接口是否UP
func checkInterfaceUp(name string) bool {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return false
	}
	return link.Attrs().Flags&net.FlagUp != 0
}

// Restart 重启隧道
func (tm *TunnelManager) Restart() error {
	fmt.Printf("重启隧道: %s\n", tm.config.Name)

	// 先删除
	if err := tm.Remove(); err != nil {
		fmt.Printf("  ⚠ 删除失败: %v\n", err)
	}

	time.Sleep(2 * time.Second)

	// 再创建
	return tm.Create()
}

// Start 启动隧道(仅创建IPsec连接和GRE隧道，不重新配置)
func (tm *TunnelManager) Start() error {
	cfg := tm.config

	fmt.Printf("  启动隧道: %s ... ", cfg.Name)

	// 检查隧道是否已存在
	if _, err := netlink.LinkByName(cfg.Name); err == nil {
		fmt.Printf("已运行\n")
		return nil
	}

	// 根据隧道类型启动
	if cfg.TunnelType == "wireguard" {
		return tm.startWireGuardTunnel()
	}
	return tm.startIPsecTunnel()
}

// startIPsecTunnel 启动 IPsec 隧道
func (tm *TunnelManager) startIPsecTunnel() error {
	cfg := tm.config

	// 1. 设置策略路由
	gateway, _ := getGatewayFromParent(cfg.ParentInterface)
	if err := setupPolicyRoute(cfg.RemoteIP, cfg.ParentInterface, gateway); err != nil {
		fmt.Printf("失败 (策略路由错误)\n")
		return err
	}

	// 2. 创建IPsec连接(如果启用加密)
	if cfg.UseEncryption {
		if err := CreateIPsec(cfg.LocalIP, cfg.RemoteIP, cfg.AuthKey, cfg.EncKey); err != nil {
			removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
			fmt.Printf("失败 (IPsec错误)\n")
			return err
		}
	}

	// 3. 创建GRE隧道
	greKey := generateGREKey(cfg.AuthKey)
	tunnel := &Tunnel{
		Name:            cfg.Name,
		LocalIP:         cfg.LocalIP,
		RemoteIP:        cfg.RemoteIP,
		LocalVirtualIP:  cfg.LocalVIP,
		RemoteVirtualIP: cfg.RemoteVIP,
		GREKey:          greKey,
	}

	if err := tunnel.Create(); err != nil {
		// 失败时清理
		if cfg.UseEncryption {
			RemoveIPsec(cfg.LocalIP, cfg.RemoteIP)
		}
		removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
		fmt.Printf("失败 (GRE错误)\n")
		return err
	}

	fmt.Printf("✓\n")
	return nil
}

// startWireGuardTunnel 启动 WireGuard 隧道
func (tm *TunnelManager) startWireGuardTunnel() error {
	cfg := tm.config

	// 1. 设置策略路由（服务端模式跳过）
	gateway, _ := getGatewayFromParent(cfg.ParentInterface)
	if cfg.RemoteIP != "0.0.0.0" {
		if err := setupPolicyRoute(cfg.RemoteIP, cfg.ParentInterface, gateway); err != nil {
			fmt.Printf("失败 (策略路由错误)\n")
			return err
		}
	}

	// 2. 创建 WireGuard 隧道
	wgTunnel := &wireguard.WireGuardTunnel{
		Name:            cfg.Name,
		Mode:            cfg.WGMode,
		LocalIP:         cfg.LocalIP,
		RemoteIP:        cfg.RemoteIP,
		LocalVIP:        cfg.LocalVIP,
		RemoteVIP:       cfg.RemoteVIP,
		PrivateKey:      cfg.PrivateKey,
		PeerPublicKey:   cfg.PeerPublicKey,
		ListenPort:      cfg.ListenPort,
		PeerListenPort:  cfg.PeerListenPort,
	}

	if err := wgTunnel.Create(); err != nil {
		// 失败时清理策略路由
		if cfg.RemoteIP != "0.0.0.0" {
			removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
		}
		fmt.Printf("失败 (WireGuard错误)\n")
		return err
	}

	fmt.Printf("✓\n")
	return nil
}

// Stop 停止隧道(删除隧道接口，保留配置)
func (tm *TunnelManager) Stop() error {
	cfg := tm.config

	fmt.Printf("  停止隧道: %s ... ", cfg.Name)

	// 检查隧道是否存在
	if _, err := netlink.LinkByName(cfg.Name); err != nil {
		fmt.Printf("未运行\n")
		return nil
	}

	// 根据隧道类型停止
	if cfg.TunnelType == "wireguard" {
		return tm.stopWireGuardTunnel()
	}
	return tm.stopIPsecTunnel()
}

// stopIPsecTunnel 停止 IPsec 隧道
func (tm *TunnelManager) stopIPsecTunnel() error {
	cfg := tm.config

	// 1. 删除GRE隧道 (直接执行撤销命令，避免重复输出)
	revFile := fmt.Sprintf("%s.rev", cfg.Name)
	if err := executeRevCommands(revFile); err != nil {
		fmt.Printf("失败 (GRE错误)\n")
		return err
	}

	// 2. 删除IPsec连接(如果启用了加密)
	if cfg.UseEncryption {
		if err := RemoveIPsec(cfg.LocalIP, cfg.RemoteIP); err != nil {
			fmt.Printf("失败 (IPsec错误)\n")
			return err
		}
	}

	// 3. 删除策略路由
	removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)

	fmt.Printf("✓\n")
	return nil
}

// stopWireGuardTunnel 停止 WireGuard 隧道
func (tm *TunnelManager) stopWireGuardTunnel() error {
	cfg := tm.config

	// 1. 删除 WireGuard 隧道 (执行撤销命令)
	revFile := fmt.Sprintf("%s.rev", cfg.Name)
	if err := executeRevCommands(revFile); err != nil {
		fmt.Printf("失败 (WireGuard错误)\n")
		return err
	}

	// 2. 删除策略路由（服务端模式跳过）
	if cfg.RemoteIP != "0.0.0.0" {
		removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
	}

	fmt.Printf("✓\n")
	return nil
}
