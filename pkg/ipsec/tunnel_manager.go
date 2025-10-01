package ipsec

import (
	"fmt"
	"net"
	"strings"
	"time"

	"trueword_node/pkg/network"

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
		fmt.Printf("  ✓ 添加路由规则: table %d\n", routeTable)
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
		fmt.Printf("  ✓ 策略路由: %s via %s dev %s table %d\n", remoteIP, gateway, parentInterface, routeTable)
	} else {
		fmt.Printf("  ✓ 策略路由: %s dev %s table %d (P2P)\n", remoteIP, parentInterface, routeTable)
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

// Create 创建隧道(包括IPsec和GRE)
func (tm *TunnelManager) Create() error {
	cfg := tm.config

	fmt.Printf("创建隧道: %s\n", cfg.Name)
	fmt.Println(strings.Repeat("=", 60))

	// 1. 验证父接口
	if err := network.ValidateParentInterface(cfg.ParentInterface); err != nil {
		return fmt.Errorf("父接口验证失败: %w", err)
	}
	fmt.Printf("  ✓ 父接口: %s\n", cfg.ParentInterface)

	// 2. 如果未指定本地IP,从父接口获取
	if cfg.LocalIP == "" {
		localIP, err := getLocalIPFromParent(cfg.ParentInterface)
		if err != nil {
			return err
		}
		cfg.LocalIP = localIP
	}
	fmt.Printf("  ✓ 本地IP: %s\n", cfg.LocalIP)

	// 3. 获取网关(用于策略路由)
	gateway, _ := getGatewayFromParent(cfg.ParentInterface)
	if gateway != "" {
		fmt.Printf("  ✓ 网关: %s\n", gateway)
	}

	// 4. 设置策略路由(确保远程IP通过正确的物理接口)
	fmt.Println("\n设置策略路由...")
	if err := setupPolicyRoute(cfg.RemoteIP, cfg.ParentInterface, gateway); err != nil {
		return fmt.Errorf("设置策略路由失败: %w", err)
	}

	// 5. 创建IPsec连接(如果启用加密)
	if cfg.UseEncryption {
		fmt.Println("\n创建IPsec连接...")
		if err := CreateIPsec(cfg.LocalIP, cfg.RemoteIP, cfg.AuthKey, cfg.EncKey); err != nil {
			removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface)
			return fmt.Errorf("创建IPsec失败: %w", err)
		}
		time.Sleep(time.Second)
	}

	// 6. 创建GRE隧道
	fmt.Println("\n创建GRE隧道...")

	// 生成GRE Key (从auth密钥生成)
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
		return fmt.Errorf("创建GRE隧道失败: %w", err)
	}

	// 7. 保存配置
	if err := network.SaveTunnelConfig(cfg); err != nil {
		fmt.Printf("  ⚠ 保存配置失败: %v\n", err)
	} else {
		fmt.Printf("  ✓ 配置已保存\n")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("✓ 隧道 %s 创建成功!\n", cfg.Name)
	fmt.Println(strings.Repeat("=", 60))

	return nil
}

// Remove 删除隧道
func (tm *TunnelManager) Remove() error {
	cfg := tm.config

	fmt.Printf("删除隧道: %s\n", cfg.Name)

	// 1. 删除GRE隧道
	if err := RemoveTunnel(cfg.Name); err != nil {
		fmt.Printf("  ⚠ 删除GRE隧道失败: %v\n", err)
	}

	// 2. 删除IPsec连接(如果启用了加密)
	if cfg.UseEncryption {
		if err := RemoveIPsec(cfg.LocalIP, cfg.RemoteIP); err != nil {
			fmt.Printf("  ⚠ 删除IPsec失败: %v\n", err)
		}
	}

	// 3. 删除策略路由
	if err := removePolicyRoute(cfg.RemoteIP, cfg.ParentInterface); err != nil {
		fmt.Printf("  ⚠ 删除策略路由失败: %v\n", err)
	}

	// 4. 删除配置文件
	if err := network.DeleteTunnelConfig(cfg.Name); err != nil {
		fmt.Printf("  ⚠ 删除配置文件失败: %v\n", err)
	}

	fmt.Printf("✓ 隧道 %s 已删除\n", cfg.Name)
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
