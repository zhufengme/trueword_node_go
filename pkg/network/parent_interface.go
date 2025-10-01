package network

import "fmt"

// ParentInterfaceInfo 父接口信息(用于显示选择)
type ParentInterfaceInfo struct {
	Name    string
	Type    string // "physical" 或 "tunnel"
	IP      string
	Gateway string
	Enabled bool
}

// ListAvailableParentInterfaces 列出所有可用的父接口
// 包括: 已启用的物理接口 + 已启用的隧道
func ListAvailableParentInterfaces() ([]ParentInterfaceInfo, error) {
	var result []ParentInterfaceInfo

	// 1. 加载物理接口
	ifaceConfig, err := LoadInterfaceConfig()
	if err == nil {
		for _, iface := range ifaceConfig.Interfaces {
			if iface.Enabled {
				result = append(result, ParentInterfaceInfo{
					Name:    iface.Name,
					Type:    "physical",
					IP:      iface.IP,
					Gateway: iface.Gateway,
					Enabled: true,
				})
			}
		}
	}

	// 2. 加载隧道配置
	tunnelConfigs, err := ListTunnelConfigs()
	if err == nil {
		for _, tunnel := range tunnelConfigs {
			if tunnel.Enabled {
				result = append(result, ParentInterfaceInfo{
					Name:    tunnel.Name,
					Type:    "tunnel",
					IP:      tunnel.LocalVIP, // 隧道使用LocalVIP
					Gateway: "",              // 隧道没有网关
					Enabled: true,
				})
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("没有可用的父接口，请先运行 'twnode init' 配置物理接口")
	}

	return result, nil
}

// GetParentInterfaceInfo 获取指定父接口的信息
func GetParentInterfaceInfo(name string) (*ParentInterfaceInfo, error) {
	// 先检查物理接口
	ifaceConfig, err := LoadInterfaceConfig()
	if err == nil {
		if iface := ifaceConfig.GetInterfaceByName(name); iface != nil {
			return &ParentInterfaceInfo{
				Name:    iface.Name,
				Type:    "physical",
				IP:      iface.IP,
				Gateway: iface.Gateway,
				Enabled: iface.Enabled,
			}, nil
		}
	}

	// 检查隧道
	tunnel, err := LoadTunnelConfig(name)
	if err == nil {
		return &ParentInterfaceInfo{
			Name:    tunnel.Name,
			Type:    "tunnel",
			IP:      tunnel.LocalVIP,
			Gateway: "",
			Enabled: tunnel.Enabled,
		}, nil
	}

	return nil, fmt.Errorf("父接口 %s 不存在", name)
}
