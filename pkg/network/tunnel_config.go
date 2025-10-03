package network

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	TunnelConfigDir = "/etc/trueword_node/tunnels"
)

// TunnelConfig 隧道配置
type TunnelConfig struct {
	Name            string `yaml:"name"`             // 隧道名称
	ParentInterface string `yaml:"parent_interface"` // 父接口(物理接口名或上级隧道名)
	LocalIP         string `yaml:"local_ip"`         // 本地IP (物理接口IP或上级隧道VIP)
	RemoteIP        string `yaml:"remote_ip"`        // 远程IP
	LocalVIP        string `yaml:"local_vip"`        // 本地虚拟IP
	RemoteVIP       string `yaml:"remote_vip"`       // 远程虚拟IP
	AuthKey         string `yaml:"auth_key"`         // 认证密钥
	EncKey          string `yaml:"enc_key"`          // 加密密钥
	Enabled         bool   `yaml:"enabled"`          // 是否启用
	UseEncryption   bool   `yaml:"use_encryption"`   // 是否使用IPsec加密
}

// SaveTunnelConfig 保存隧道配置
func SaveTunnelConfig(config *TunnelConfig) error {
	if err := os.MkdirAll(TunnelConfigDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	configPath := filepath.Join(TunnelConfigDir, config.Name+".yaml")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// LoadTunnelConfig 加载隧道配置
func LoadTunnelConfig(name string) (*TunnelConfig, error) {
	configPath := filepath.Join(TunnelConfigDir, name+".yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config TunnelConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// ListTunnelConfigs 列出所有隧道配置
func ListTunnelConfigs() ([]*TunnelConfig, error) {
	entries, err := os.ReadDir(TunnelConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*TunnelConfig{}, nil
		}
		return nil, fmt.Errorf("读取配置目录失败: %w", err)
	}

	var configs []*TunnelConfig
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}

		tunnelName := name[:len(name)-5] // 去掉 .yaml 后缀
		config, err := LoadTunnelConfig(tunnelName)
		if err != nil {
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// DeleteTunnelConfig 删除隧道配置
func DeleteTunnelConfig(name string) error {
	configPath := filepath.Join(TunnelConfigDir, name+".yaml")
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除配置文件失败: %w", err)
	}
	return nil
}

// ValidateParentInterface 验证父接口是否存在
func ValidateParentInterface(parentName string) error {
	// 首先检查是否是物理接口
	ifaceConfig, err := LoadInterfaceConfig()
	if err == nil {
		if iface := ifaceConfig.GetInterfaceByName(parentName); iface != nil {
			if !iface.Enabled {
				return fmt.Errorf("父接口 %s 未启用", parentName)
			}
			return nil
		}
	}

	// 检查是否是隧道接口
	tunnelConfig, err := LoadTunnelConfig(parentName)
	if err != nil {
		return fmt.Errorf("父接口 %s 不存在", parentName)
	}

	if !tunnelConfig.Enabled {
		return fmt.Errorf("父隧道 %s 未启用", parentName)
	}

	return nil
}
