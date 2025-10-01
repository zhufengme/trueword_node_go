package network

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/vishvananda/netlink"
	"gopkg.in/yaml.v3"
)

const (
	InterfaceConfigDir  = "/etc/trueword_node/interfaces"
	InterfaceConfigFile = "physical.yaml"
)

// PhysicalInterface 物理网络接口
type PhysicalInterface struct {
	Name    string `yaml:"name"`     // 接口名 (如 eth0, ens33)
	IP      string `yaml:"ip"`       // IP地址
	Gateway string `yaml:"gateway"`  // 网关地址
	Enabled bool   `yaml:"enabled"`  // 是否启用
}

// InterfaceConfig 所有物理接口配置
type InterfaceConfig struct {
	Interfaces []PhysicalInterface `yaml:"interfaces"`
}

// ScanPhysicalInterfaces 扫描所有物理网络接口
func ScanPhysicalInterfaces() ([]PhysicalInterface, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("获取网络接口列表失败: %w", err)
	}

	var interfaces []PhysicalInterface

	for _, link := range links {
		attrs := link.Attrs()

		// 跳过loopback和虚拟接口
		if attrs.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 只保留以太网类型的物理接口
		// 跳过隧道、bridge、veth等虚拟接口
		linkType := link.Type()
		if linkType != "device" && linkType != "tun" && linkType != "bond" {
			// 允许普通设备和bond接口
			if attrs.Name != "eth0" && attrs.Name != "ens" &&
			   !isPhysicalInterface(attrs.Name) {
				continue
			}
		}

		// 获取接口的IPv4地址
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			continue
		}

		if len(addrs) == 0 {
			continue // 没有IPv4地址的接口跳过
		}

		ip := addrs[0].IP.String()

		// 获取网关
		gateway, _ := GetGatewayByInterface(attrs.Name)

		interfaces = append(interfaces, PhysicalInterface{
			Name:    attrs.Name,
			IP:      ip,
			Gateway: gateway,
			Enabled: true,
		})
	}

	return interfaces, nil
}

// isPhysicalInterface 判断是否为物理接口
func isPhysicalInterface(name string) bool {
	// 常见的物理接口前缀
	prefixes := []string{"eth", "ens", "enp", "eno", "em", "p"}

	for _, prefix := range prefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// GetGatewayByInterface 获取指定接口的网关
func GetGatewayByInterface(ifname string) (string, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", fmt.Errorf("获取路由表失败: %w", err)
	}

	link, err := netlink.LinkByName(ifname)
	if err != nil {
		return "", fmt.Errorf("获取接口失败: %w", err)
	}

	linkIndex := link.Attrs().Index

	// 查找默认路由或通过该接口的路由
	for _, route := range routes {
		if route.LinkIndex == linkIndex && route.Gw != nil {
			return route.Gw.String(), nil
		}
	}

	return "", nil
}

// SaveInterfaceConfig 保存接口配置
func SaveInterfaceConfig(config *InterfaceConfig) error {
	if err := os.MkdirAll(InterfaceConfigDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	configPath := filepath.Join(InterfaceConfigDir, InterfaceConfigFile)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// LoadInterfaceConfig 加载接口配置
func LoadInterfaceConfig() (*InterfaceConfig, error) {
	configPath := filepath.Join(InterfaceConfigDir, InterfaceConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &InterfaceConfig{Interfaces: []PhysicalInterface{}}, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config InterfaceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// GetInterfaceByName 根据名称获取物理接口配置
func (c *InterfaceConfig) GetInterfaceByName(name string) *PhysicalInterface {
	for i := range c.Interfaces {
		if c.Interfaces[i].Name == name {
			return &c.Interfaces[i]
		}
	}
	return nil
}

// AddOrUpdateInterface 添加或更新接口
func (c *InterfaceConfig) AddOrUpdateInterface(iface PhysicalInterface) {
	for i := range c.Interfaces {
		if c.Interfaces[i].Name == iface.Name {
			c.Interfaces[i] = iface
			return
		}
	}
	c.Interfaces = append(c.Interfaces, iface)
}

// RemoveInterface 移除接口
func (c *InterfaceConfig) RemoveInterface(name string) bool {
	for i := range c.Interfaces {
		if c.Interfaces[i].Name == name {
			c.Interfaces = append(c.Interfaces[:i], c.Interfaces[i+1:]...)
			return true
		}
	}
	return false
}
