package failover

import (
	"fmt"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile = "/etc/trueword_node/failover_daemon.yaml"
)

// FailoverConfig 守护进程配置
type FailoverConfig struct {
	Daemon   DaemonConfig     `yaml:"daemon"`
	Monitors []MonitorConfig  `yaml:"monitors"`
}

// DaemonConfig 全局配置
type DaemonConfig struct {
	CheckIntervalMs           int     `yaml:"check_interval_ms"`
	FailureThreshold          int     `yaml:"failure_threshold"`
	RecoveryThreshold         int     `yaml:"recovery_threshold"`
	ScoreThreshold            float64 `yaml:"score_threshold"`            // 评分差值阈值（避免频繁切换）
	SwitchConfirmationCount   int     `yaml:"switch_confirmation_count"`  // 切换确认次数（默认1）
	CheckMode                 string  `yaml:"check_mode"`                 // 全局默认检测模式：ping / dns
	DNSQueryDomain            string  `yaml:"dns_query_domain"`           // DNS 查询的默认域名
	LogFile                   string  `yaml:"log_file"`
}

// MonitorConfig 监控任务配置
type MonitorConfig struct {
	Name                    string   `yaml:"name"`
	Type                    string   `yaml:"type"` // policy_group 或 default_route
	Target                  string   `yaml:"target"`
	CheckTargets            []string `yaml:"check_targets"`              // ping 模式使用
	CandidateExits          []string `yaml:"candidate_exits"`
	CheckIntervalMs         int      `yaml:"check_interval_ms"`          // 可选，覆盖全局配置
	FailureThreshold        int      `yaml:"failure_threshold"`          // 可选，覆盖全局配置
	RecoveryThreshold       int      `yaml:"recovery_threshold"`         // 可选，覆盖全局配置
	ScoreThreshold          float64  `yaml:"score_threshold"`            // 可选，覆盖全局配置
	SwitchConfirmationCount int      `yaml:"switch_confirmation_count"`  // 可选，覆盖全局配置
	CheckMode               string   `yaml:"check_mode"`                 // 可选，覆盖全局检测模式：ping / dns
	DNSServers              []string `yaml:"dns_servers"`                // dns 模式使用
	DNSQueryDomain          string   `yaml:"dns_query_domain"`           // 可选，覆盖全局查询域名
}

// GetCheckInterval 获取检测间隔（优先使用局部配置）
func (m *MonitorConfig) GetCheckInterval(globalInterval int) int {
	if m.CheckIntervalMs > 0 {
		return m.CheckIntervalMs
	}
	return globalInterval
}

// GetFailureThreshold 获取失败阈值
func (m *MonitorConfig) GetFailureThreshold(globalThreshold int) int {
	if m.FailureThreshold > 0 {
		return m.FailureThreshold
	}
	return globalThreshold
}

// GetRecoveryThreshold 获取恢复阈值
func (m *MonitorConfig) GetRecoveryThreshold(globalThreshold int) int {
	if m.RecoveryThreshold > 0 {
		return m.RecoveryThreshold
	}
	return globalThreshold
}

// GetScoreThreshold 获取评分差值阈值
func (m *MonitorConfig) GetScoreThreshold(globalThreshold float64) float64 {
	if m.ScoreThreshold > 0 {
		return m.ScoreThreshold
	}
	return globalThreshold
}

// GetSwitchConfirmationCount 获取切换确认次数
func (m *MonitorConfig) GetSwitchConfirmationCount(globalCount int) int {
	if m.SwitchConfirmationCount > 0 {
		return m.SwitchConfirmationCount
	}
	if globalCount > 0 {
		return globalCount
	}
	return 1 // 默认值：1（向上兼容）
}

// GetCheckMode 获取检测模式（优先使用局部配置）
func (m *MonitorConfig) GetCheckMode(globalMode string) string {
	if m.CheckMode != "" {
		return m.CheckMode
	}
	if globalMode != "" {
		return globalMode
	}
	return "ping" // 默认值：ping
}

// GetDNSQueryDomain 获取 DNS 查询域名（优先使用局部配置）
func (m *MonitorConfig) GetDNSQueryDomain(globalDomain string) string {
	if m.DNSQueryDomain != "" {
		return m.DNSQueryDomain
	}
	if globalDomain != "" {
		return globalDomain
	}
	return "google.com" // 默认值
}

// Equals 比较两个MonitorConfig是否相等
func (m *MonitorConfig) Equals(other *MonitorConfig) bool {
	if m.Name != other.Name || m.Type != other.Type || m.Target != other.Target {
		return false
	}
	if !stringSliceEqual(m.CheckTargets, other.CheckTargets) {
		return false
	}
	if !stringSliceEqual(m.CandidateExits, other.CandidateExits) {
		return false
	}
	if m.CheckIntervalMs != other.CheckIntervalMs {
		return false
	}
	if m.FailureThreshold != other.FailureThreshold {
		return false
	}
	if m.RecoveryThreshold != other.RecoveryThreshold {
		return false
	}
	if m.ScoreThreshold != other.ScoreThreshold {
		return false
	}
	if m.SwitchConfirmationCount != other.SwitchConfirmationCount {
		return false
	}
	return true
}

// LoadConfig 加载配置文件
func LoadConfig(configFile string) (*FailoverConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config FailoverConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}

// SaveConfig 保存配置文件
func SaveConfig(configFile string, config *FailoverConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	err = os.WriteFile(configFile, data, 0644)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// Validate 验证配置
func (config *FailoverConfig) Validate() error {
	errors := []string{}

	// 验证全局配置
	if config.Daemon.CheckIntervalMs < 100 || config.Daemon.CheckIntervalMs > 60000 {
		errors = append(errors, "daemon.check_interval_ms 必须在 100-60000 范围内")
	}
	// failure_threshold 和 recovery_threshold 已废弃（v1.4），仅在设置时验证范围
	if config.Daemon.FailureThreshold != 0 {
		if config.Daemon.FailureThreshold < 1 || config.Daemon.FailureThreshold > 20 {
			errors = append(errors, "daemon.failure_threshold 必须在 1-20 范围内（已废弃，建议删除）")
		}
	}
	if config.Daemon.RecoveryThreshold != 0 {
		if config.Daemon.RecoveryThreshold < 1 || config.Daemon.RecoveryThreshold > 20 {
			errors = append(errors, "daemon.recovery_threshold 必须在 1-20 范围内（已废弃，建议删除）")
		}
	}
	if config.Daemon.ScoreThreshold < 0 || config.Daemon.ScoreThreshold > 100 {
		errors = append(errors, "daemon.score_threshold 必须在 0-100 范围内")
	}
	// 切换确认次数验证（如果为0则设置默认值1）
	if config.Daemon.SwitchConfirmationCount == 0 {
		config.Daemon.SwitchConfirmationCount = 1 // 默认值，向上兼容
	} else if config.Daemon.SwitchConfirmationCount < 1 || config.Daemon.SwitchConfirmationCount > 10 {
		errors = append(errors, "daemon.switch_confirmation_count 必须在 1-10 范围内")
	}

	// 验证每个monitor
	monitorNames := make(map[string]bool)
	for i, monitor := range config.Monitors {
		prefix := fmt.Sprintf("monitors[%d]", i)

		// 名称不能为空
		if monitor.Name == "" {
			errors = append(errors, prefix+".name 不能为空")
		} else {
			// 检查名称重复
			if monitorNames[monitor.Name] {
				errors = append(errors, fmt.Sprintf("%s.name: 监控任务名称 '%s' 重复", prefix, monitor.Name))
			}
			monitorNames[monitor.Name] = true
		}

		// 类型必须是 policy_group 或 default_route
		if monitor.Type != "policy_group" && monitor.Type != "default_route" {
			errors = append(errors, prefix+".type 必须是 policy_group 或 default_route")
		}

		// 目标不能为空
		if monitor.Target == "" {
			errors = append(errors, prefix+".target 不能为空")
		}

		// 检测模式验证
		checkMode := monitor.GetCheckMode(config.Daemon.CheckMode)
		if checkMode != "ping" && checkMode != "dns" {
			errors = append(errors, fmt.Sprintf("%s.check_mode 必须是 'ping' 或 'dns'", prefix))
		}

		if checkMode == "dns" {
			// DNS 模式：必须有 dns_servers
			if len(monitor.DNSServers) < 1 {
				errors = append(errors, prefix+".dns_servers 至少需要 1 个 DNS 服务器（DNS 模式）")
			}
			for j, dnsServer := range monitor.DNSServers {
				if net.ParseIP(dnsServer) == nil {
					errors = append(errors, fmt.Sprintf("%s.dns_servers[%d] 不是有效的IP: %s", prefix, j, dnsServer))
				}
			}
		} else {
			// ping 模式：必须有 check_targets（1-3个）
			if len(monitor.CheckTargets) < 1 || len(monitor.CheckTargets) > 3 {
				errors = append(errors, prefix+".check_targets 必须有 1-3 个IP（ping 模式）")
			}
			for j, ip := range monitor.CheckTargets {
				if net.ParseIP(ip) == nil {
					errors = append(errors, fmt.Sprintf("%s.check_targets[%d] 不是有效的IP: %s", prefix, j, ip))
				}
			}
		}

		// 候选出口（至少2个）
		if len(monitor.CandidateExits) < 2 {
			errors = append(errors, prefix+".candidate_exits 至少需要 2 个接口")
		}

		// 覆盖参数验证
		if monitor.CheckIntervalMs != 0 {
			if monitor.CheckIntervalMs < 100 || monitor.CheckIntervalMs > 60000 {
				errors = append(errors, prefix+".check_interval_ms 必须在 100-60000 范围内")
			}
		}
		// failure_threshold 和 recovery_threshold 已废弃（v1.4）
		if monitor.FailureThreshold != 0 {
			if monitor.FailureThreshold < 1 || monitor.FailureThreshold > 20 {
				errors = append(errors, prefix+".failure_threshold 必须在 1-20 范围内（已废弃，建议删除）")
			}
		}
		if monitor.RecoveryThreshold != 0 {
			if monitor.RecoveryThreshold < 1 || monitor.RecoveryThreshold > 20 {
				errors = append(errors, prefix+".recovery_threshold 必须在 1-20 范围内（已废弃，建议删除）")
			}
		}
		if monitor.ScoreThreshold != 0 {
			if monitor.ScoreThreshold < 0 || monitor.ScoreThreshold > 100 {
				errors = append(errors, prefix+".score_threshold 必须在 0-100 范围内")
			}
		}
		if monitor.SwitchConfirmationCount != 0 {
			if monitor.SwitchConfirmationCount < 1 || monitor.SwitchConfirmationCount > 10 {
				errors = append(errors, prefix+".switch_confirmation_count 必须在 1-10 范围内")
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("配置验证失败:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// GetMonitorNames 获取所有监控任务名称
func (config *FailoverConfig) GetMonitorNames() []string {
	names := make([]string, len(config.Monitors))
	for i, monitor := range config.Monitors {
		names[i] = monitor.Name
	}
	return names
}

// GetMonitor 获取指定名称的监控任务
func (config *FailoverConfig) GetMonitor(name string) *MonitorConfig {
	for i := range config.Monitors {
		if config.Monitors[i].Name == name {
			return &config.Monitors[i]
		}
	}
	return nil
}

// AddMonitor 添加监控任务
func (config *FailoverConfig) AddMonitor(monitor MonitorConfig) error {
	// 检查名称是否重复
	if config.GetMonitor(monitor.Name) != nil {
		return fmt.Errorf("监控任务 '%s' 已存在", monitor.Name)
	}

	config.Monitors = append(config.Monitors, monitor)
	return nil
}

// RemoveMonitor 删除监控任务
func (config *FailoverConfig) RemoveMonitor(name string) error {
	for i, monitor := range config.Monitors {
		if monitor.Name == name {
			config.Monitors = append(config.Monitors[:i], config.Monitors[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("监控任务 '%s' 不存在", name)
}

// UpdateMonitor 更新监控任务
func (config *FailoverConfig) UpdateMonitor(name string, newMonitor MonitorConfig) error {
	for i := range config.Monitors {
		if config.Monitors[i].Name == name {
			// 保持名称不变
			newMonitor.Name = name
			config.Monitors[i] = newMonitor
			return nil
		}
	}
	return fmt.Errorf("监控任务 '%s' 不存在", name)
}

// 辅助函数：比较字符串切片
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 辅助函数：求差集
func difference(a, b []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}
	var diff []string
	for _, x := range a {
		if !mb[x] {
			diff = append(diff, x)
		}
	}
	return diff
}

// 辅助函数：求交集
func intersection(a, b []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}
	var inter []string
	for _, x := range a {
		if mb[x] {
			inter = append(inter, x)
		}
	}
	return inter
}
