package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
)

// InterfaceType 接口类型
type InterfaceType int

const (
	InterfaceTypeUnknown    InterfaceType = iota // 未知类型
	InterfaceTypeLoopback                        // 回环接口
	InterfaceTypePhysical                        // 物理接口（本程序管理）
	InterfaceTypeTunnel                          // 本程序创建的隧道
	InterfaceTypeThirdParty                      // 第三方创建的接口（OpenVPN、WireGuard等）
)

func (t InterfaceType) String() string {
	switch t {
	case InterfaceTypeLoopback:
		return "Loopback"
	case InterfaceTypePhysical:
		return "物理接口"
	case InterfaceTypeTunnel:
		return "隧道"
	case InterfaceTypeThirdParty:
		return "第三方接口"
	default:
		return "未知"
	}
}

// InterfaceInfo 接口信息
type InterfaceInfo struct {
	Name    string        // 接口名
	Type    InterfaceType // 接口类型
	IP      string        // IP地址
	Gateway string        // 网关（如果有）
	IsUp    bool          // 是否UP
}

// DetectInterfaceType 检测接口类型
func DetectInterfaceType(interfaceName string) InterfaceType {
	// 1. 检查是否是Loopback
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return InterfaceTypeUnknown
	}

	if link.Attrs().Flags&net.FlagLoopback != 0 {
		return InterfaceTypeLoopback
	}

	// 2. 检查是否是本程序管理的物理接口
	ifaceConfig, err := LoadInterfaceConfig()
	if err == nil {
		for _, iface := range ifaceConfig.Interfaces {
			if iface.Name == interfaceName {
				return InterfaceTypePhysical
			}
		}
	}

	// 3. 检查是否是本程序创建的隧道
	_, err = LoadTunnelConfig(interfaceName)
	if err == nil {
		return InterfaceTypeTunnel
	}

	// 4. 其他都是第三方接口
	return InterfaceTypeThirdParty
}

// GetInterfaceInfo 获取接口详细信息
func GetInterfaceInfo(interfaceName string) (*InterfaceInfo, error) {
	info := &InterfaceInfo{
		Name: interfaceName,
		Type: DetectInterfaceType(interfaceName),
	}

	// 检查接口是否存在
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("接口 %s 不存在: %w", interfaceName, err)
	}

	// 检查接口状态
	info.IsUp = link.Attrs().Flags&net.FlagUp != 0

	// 获取IP地址
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err == nil && len(addrs) > 0 {
		info.IP = addrs[0].IP.String()
	}

	// 尝试获取网关（根据接口类型）
	switch info.Type {
	case InterfaceTypePhysical:
		// 从配置文件获取
		ifaceConfig, err := LoadInterfaceConfig()
		if err == nil {
			for _, iface := range ifaceConfig.Interfaces {
				if iface.Name == interfaceName {
					info.Gateway = iface.Gateway
					break
				}
			}
		}

	case InterfaceTypeTunnel:
		// 隧道没有网关
		info.Gateway = ""

	case InterfaceTypeThirdParty:
		// 尝试从路由表获取网关
		info.Gateway = GetGatewayFromRoutes(interfaceName)
	}

	return info, nil
}

// GetGatewayFromRoutes 从路由表获取接口的网关
func GetGatewayFromRoutes(interfaceName string) string {
	// 使用 ip route show 查找默认网关
	cmd := exec.Command("ip", "route", "show", "dev", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// 解析输出，查找 "via <gateway>" 模式
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "via") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "via" && i+1 < len(fields) {
					return fields[i+1]
				}
			}
		}
	}

	return ""
}

// IsInterfaceUp 检查接口是否存在且UP
func IsInterfaceUp(interfaceName string) bool {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return false
	}
	return link.Attrs().Flags&net.FlagUp != 0
}

// ListAllInterfaces 列出所有可用的网络接口（排除loopback）
func ListAllInterfaces() ([]*InterfaceInfo, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("获取接口列表失败: %w", err)
	}

	var interfaces []*InterfaceInfo

	for _, link := range links {
		attrs := link.Attrs()

		// 跳过loopback
		if attrs.Flags&net.FlagLoopback != 0 {
			continue
		}

		info, err := GetInterfaceInfo(attrs.Name)
		if err != nil {
			continue
		}

		interfaces = append(interfaces, info)
	}

	return interfaces, nil
}

// ValidateExitInterface 验证接口是否可以作为出口
// 返回接口信息和错误
func ValidateExitInterface(interfaceName string) (*InterfaceInfo, error) {
	// 1. 检查接口是否存在
	if _, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", interfaceName)); os.IsNotExist(err) {
		return nil, fmt.Errorf("接口 %s 不存在", interfaceName)
	}

	// 2. 获取接口信息
	info, err := GetInterfaceInfo(interfaceName)
	if err != nil {
		return nil, err
	}

	// 3. 检查是否是loopback
	if info.Type == InterfaceTypeLoopback {
		return nil, fmt.Errorf("不能使用回环接口作为出口")
	}

	// 4. 检查接口是否UP
	if !info.IsUp {
		return nil, fmt.Errorf("接口 %s 未启动", interfaceName)
	}

	return info, nil
}
