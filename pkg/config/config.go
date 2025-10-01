package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDir  = "/etc/trueword_node"
	ConfigFile = "config.yaml"
)

type Config struct {
	// 路由配置
	Routing RoutingConfig `yaml:"routing"`
}

type RoutingConfig struct {
	// 默认路由出口 (可以是隧道名或物理接口名)
	DefaultExit string `yaml:"default_exit"`
}

// 生成IPsec密钥(从字符串生成)
func GenerateIPsecKeys(authPass, encPass string) (authKey, encKey string, err error) {
	if authPass == "" || encPass == "" {
		return "", "", fmt.Errorf("密钥字符串不能为空")
	}

	// 使用SHA256生成认证密钥 (32字节)
	authHash := sha256.Sum256([]byte(authPass))
	authKey = "0x" + hex.EncodeToString(authHash[:])

	// 使用SHA256生成加密密钥 (32字节，AES-256)
	encHash := sha256.Sum256([]byte(encPass))
	encKey = "0x" + hex.EncodeToString(encHash[:])

	return authKey, encKey, nil
}

// 加载配置
func Load() (*Config, error) {
	configPath := filepath.Join(ConfigDir, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}

// 保存配置
func (c *Config) Save() error {
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	configPath := filepath.Join(ConfigDir, ConfigFile)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// 创建默认配置
func CreateDefault() *Config {
	return &Config{
		Routing: RoutingConfig{
			DefaultExit: "",
		},
	}
}
