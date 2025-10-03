package ipsec

import (
	"fmt"
	"os"
	"strings"

	"trueword_node/pkg/config"
	"trueword_node/pkg/network"
)

// StartAllTunnels 启动所有隧道
func StartAllTunnels() error {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    批量启动所有隧道                        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 加载所有隧道配置
	tunnelDir := config.ConfigDir + "/tunnels"
	entries, err := os.ReadDir(tunnelDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("未找到任何隧道配置")
			return nil
		}
		return err
	}

	tunnelNames := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".yaml") {
			tunnelNames = append(tunnelNames, strings.TrimSuffix(entry.Name(), ".yaml"))
		}
	}

	if len(tunnelNames) == 0 {
		fmt.Println("未找到任何隧道配置")
		return nil
	}

	fmt.Printf("找到 %d 个隧道，开始启动...\n\n", len(tunnelNames))

	successCount := 0
	failedCount := 0

	for _, tunnelName := range tunnelNames {
		// 加载隧道配置
		tunnelConfig, err := network.LoadTunnelConfig(tunnelName)
		if err != nil {
			fmt.Printf("  加载隧道 %s 配置失败: %v\n", tunnelName, err)
			failedCount++
			continue
		}

		// 跳过未启用的隧道
		if !tunnelConfig.Enabled {
			fmt.Printf("  跳过隧道: %s (已禁用)\n", tunnelName)
			continue
		}

		// 启动隧道
		tm := NewTunnelManager(tunnelConfig)
		if err := tm.Start(); err != nil {
			failedCount++
		} else {
			successCount++
		}
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  启动完成: 成功 %d 个, 失败 %d 个                           ║\n", successCount, failedCount)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}

// StopAllTunnels 停止所有隧道（无论是否启用）
func StopAllTunnels() error {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    批量停止所有隧道                        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 加载所有隧道配置
	tunnelDir := config.ConfigDir + "/tunnels"
	entries, err := os.ReadDir(tunnelDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("未找到任何隧道配置")
			return nil
		}
		return err
	}

	tunnelNames := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".yaml") {
			tunnelNames = append(tunnelNames, strings.TrimSuffix(entry.Name(), ".yaml"))
		}
	}

	if len(tunnelNames) == 0 {
		fmt.Println("未找到任何隧道配置")
		return nil
	}

	fmt.Printf("找到 %d 个隧道，开始停止...\n\n", len(tunnelNames))

	successCount := 0
	failedCount := 0

	for _, tunnelName := range tunnelNames {
		// 加载隧道配置
		tunnelConfig, err := network.LoadTunnelConfig(tunnelName)
		if err != nil {
			fmt.Printf("  加载隧道 %s 配置失败: %v\n", tunnelName, err)
			failedCount++
			continue
		}

		// 停止隧道（包括禁用的隧道）
		tm := NewTunnelManager(tunnelConfig)
		if err := tm.Stop(); err != nil {
			failedCount++
		} else {
			successCount++
		}
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  停止完成: 成功 %d 个, 失败 %d 个                           ║\n", successCount, failedCount)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}
