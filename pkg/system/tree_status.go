package system

import (
	"fmt"

	"trueword_node/pkg/network"
)

// ANSI 颜色代码
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
	colorBold    = "\033[1m"

	// 高亮颜色
	colorBrightGreen  = "\033[92m"
	colorBrightYellow = "\033[93m"
	colorBrightCyan   = "\033[96m"
)

// InterfaceNode 接口节点（物理接口或隧道）
type InterfaceNode struct {
	Name            string           // 接口名称
	Type            string           // 类型: "physical" 或 "tunnel"
	IsPhysical      bool             // 是否是物理接口
	Config          interface{}      // 配置信息（*network.PhysicalInterface 或 *network.TunnelConfig）
	Children        []*InterfaceNode // 子节点（基于此接口的隧道）
	CheckResult     *network.CheckResult
	IsDefaultExit   bool // 是否是默认路由出口
}

// BuildInterfaceTree 构建接口树结构
func BuildInterfaceTree(checkResults *network.AllCheckResults, defaultExit string) ([]*InterfaceNode, error) {
	// 加载所有物理接口
	ifaceConfig, err := network.LoadInterfaceConfig()
	if err != nil {
		ifaceConfig = &network.InterfaceConfig{
			Interfaces: []network.PhysicalInterface{},
		}
	}

	// 加载所有隧道配置
	tunnelConfigs, err := network.ListTunnelConfigs()
	if err != nil {
		tunnelConfigs = []*network.TunnelConfig{}
	}

	// 创建节点映射（用于快速查找）
	nodeMap := make(map[string]*InterfaceNode)

	// 1. 创建所有物理接口节点
	for i := range ifaceConfig.Interfaces {
		iface := &ifaceConfig.Interfaces[i]
		node := &InterfaceNode{
			Name:       iface.Name,
			Type:       "physical",
			IsPhysical: true,
			Config:     iface,
			Children:   make([]*InterfaceNode, 0),
		}
		if checkResults != nil {
			node.CheckResult = checkResults.Results[iface.Name]
		}
		if defaultExit != "" && iface.Name == defaultExit {
			node.IsDefaultExit = true
		}
		nodeMap[iface.Name] = node
	}

	// 2. 创建所有隧道节点
	for _, tunnel := range tunnelConfigs {
		node := &InterfaceNode{
			Name:       tunnel.Name,
			Type:       "tunnel",
			IsPhysical: false,
			Config:     tunnel,
			Children:   make([]*InterfaceNode, 0),
		}
		if checkResults != nil {
			node.CheckResult = checkResults.Results[tunnel.Name]
		}
		if defaultExit != "" && tunnel.Name == defaultExit {
			node.IsDefaultExit = true
		}
		nodeMap[tunnel.Name] = node
	}

	// 3. 建立父子关系
	for _, tunnel := range tunnelConfigs {
		childNode := nodeMap[tunnel.Name]
		parentNode, exists := nodeMap[tunnel.ParentInterface]

		if exists && childNode != nil {
			// 将子节点添加到父节点的children列表
			parentNode.Children = append(parentNode.Children, childNode)
		}
	}

	// 4. 收集根节点（物理接口 + 没有父节点的隧道）
	rootNodes := make([]*InterfaceNode, 0)

	// 添加所有物理接口作为根节点
	for i := range ifaceConfig.Interfaces {
		iface := &ifaceConfig.Interfaces[i]
		if node, ok := nodeMap[iface.Name]; ok {
			rootNodes = append(rootNodes, node)
		}
	}

	// 添加孤立的隧道（父接口不存在的）
	for _, tunnel := range tunnelConfigs {
		_, parentExists := nodeMap[tunnel.ParentInterface]
		if !parentExists {
			if node, ok := nodeMap[tunnel.Name]; ok {
				rootNodes = append(rootNodes, node)
			}
		}
	}

	return rootNodes, nil
}

// PrintInterfaceTree 打印接口树
func PrintInterfaceTree(roots []*InterfaceNode) {
	fmt.Println("【接口拓扑】")
	fmt.Println()

	if len(roots) == 0 {
		fmt.Println("  (无接口)")
		return
	}

	for i, root := range roots {
		isLast := (i == len(roots)-1)
		printNode(root, "", isLast)
	}
}

// printNode 递归打印节点
func printNode(node *InterfaceNode, prefix string, isLast bool) {
	// 构建当前行的前缀
	var connector, childPrefix string
	if prefix == "" {
		// 根节点
		connector = ""
		if isLast {
			childPrefix = "    "
		} else {
			childPrefix = "│   "
		}
	} else {
		// 子节点
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		} else {
			connector = "├── "
			childPrefix = prefix + "│   "
		}
	}

	// 获取节点信息
	var statusIcon, statusColor, typeStr, typeColor, infoStr string

	if node.IsPhysical {
		// 物理接口
		iface := node.Config.(*network.PhysicalInterface)
		typeStr = "物理"
		typeColor = colorBlue

		if !iface.Enabled {
			statusIcon = "⊗"
			statusColor = colorGray
			infoStr = "已禁用"
		} else if node.CheckResult != nil {
			switch node.CheckResult.Status {
			case "UP":
				statusIcon = "✓"
				statusColor = colorBrightGreen
				infoStr = fmt.Sprintf("%.1fms, 丢包率%.0f%%", node.CheckResult.Latency, node.CheckResult.PacketLoss)
			case "DOWN":
				statusIcon = "✗"
				statusColor = colorRed
				infoStr = fmt.Sprintf("%.1fms, 丢包率%.0f%%", node.CheckResult.Latency, node.CheckResult.PacketLoss)
			case "IDLE":
				statusIcon = "○"
				statusColor = colorYellow
				infoStr = "空闲"
			default:
				statusIcon = "?"
				statusColor = colorGray
				infoStr = "未知"
			}
		} else {
			statusIcon = "○"
			statusColor = colorCyan
			infoStr = "未检查"
		}

		// 显示IP地址
		if iface.IP != "" {
			infoStr = fmt.Sprintf("%s | IP: %s%s%s", infoStr, colorBrightCyan, iface.IP, colorReset)
		}
	} else {
		// 隧道接口
		tunnel := node.Config.(*network.TunnelConfig)
		typeStr = "隧道"
		typeColor = colorMagenta

		if !tunnel.Enabled {
			statusIcon = "⊗"
			statusColor = colorGray
			infoStr = "已禁用"
		} else if node.CheckResult != nil {
			switch node.CheckResult.Status {
			case "UP":
				statusIcon = "✓"
				statusColor = colorBrightGreen
				infoStr = fmt.Sprintf("%.1fms, 丢包率%.0f%%", node.CheckResult.Latency, node.CheckResult.PacketLoss)
			case "DOWN":
				statusIcon = "✗"
				statusColor = colorRed
				infoStr = fmt.Sprintf("%.1fms, 丢包率%.0f%%", node.CheckResult.Latency, node.CheckResult.PacketLoss)
			case "IDLE":
				statusIcon = "○"
				statusColor = colorYellow
				infoStr = "空闲"
			default:
				statusIcon = "?"
				statusColor = colorGray
				infoStr = "未知"
			}
		} else {
			statusIcon = "○"
			statusColor = colorCyan
			infoStr = "未检查"
		}

		// 显示远程IP和VIP
		if tunnel.RemoteIP != "" {
			infoStr = fmt.Sprintf("%s | 远程: %s%s%s", infoStr, colorBrightYellow, tunnel.RemoteIP, colorReset)
		}
		if tunnel.RemoteVIP != "" {
			infoStr = fmt.Sprintf("%s, VIP: %s%s%s", infoStr, colorCyan, tunnel.RemoteVIP, colorReset)
		}
	}

	// 构建节点名称（如果是默认路由出口，加上星号）
	nodeName := node.Name
	nameColor := colorBold
	if node.IsDefaultExit {
		nodeName = nodeName + " " + colorBrightYellow + "★" + colorReset
	}

	// 打印当前节点（带颜色）
	line := fmt.Sprintf("%s%s%s%s%s%s%s [%s%s%s] %s%s%s (%s)",
		colorGray, prefix, connector, colorReset,
		statusColor, statusIcon, colorReset,
		nameColor, nodeName, colorReset,
		typeColor, typeStr, colorReset,
		infoStr)

	fmt.Println(line)

	// 递归打印子节点
	for i, child := range node.Children {
		isLastChild := (i == len(node.Children)-1)
		printNode(child, childPrefix, isLastChild)
	}
}
