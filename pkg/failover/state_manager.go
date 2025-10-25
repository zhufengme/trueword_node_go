package failover

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	StateFile = "/var/lib/trueword_node/failover_state.json"
)

// InterfaceState 接口状态
type InterfaceState struct {
	Name             string    `json:"name"`
	Latency          float64   `json:"latency"`           // 平均延迟（ms）
	PacketLoss       float64   `json:"packet_loss"`       // 丢包率（%）
	BaseScore        float64   `json:"base_score"`        // 基础评分
	Cost             int       `json:"cost"`              // 成本
	FinalScore       float64   `json:"final_score"`       // 最终评分
	LastCheckTime    time.Time `json:"last_check_time"`
	LastTarget       string    `json:"last_target"`        // 最后使用的目标IP
	InitialCheckDone bool      `json:"initial_check_done"` // 是否完成初始检测
}

// FailoverEvent 故障转移事件
type FailoverEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	MonitorName string    `json:"monitor_name"`
	EventType   string    `json:"event_type"` // "failover", "check"
	Message     string    `json:"message"`
}

// RuntimeState 运行时状态
type RuntimeState struct {
	StartTime       time.Time                  `json:"start_time"`
	InterfaceStates map[string]*InterfaceState `json:"interface_states"`
	RecentEvents    []FailoverEvent            `json:"recent_events"` // 最近20条
}

// StateManager 状态管理器
type StateManager struct {
	states    map[string]*InterfaceState
	events    []FailoverEvent
	startTime time.Time
	mutex     sync.RWMutex
}

// NewStateManager 创建状态管理器
func NewStateManager() *StateManager {
	return &StateManager{
		states:    make(map[string]*InterfaceState),
		events:    make([]FailoverEvent, 0),
		startTime: time.Now(),
	}
}

// GetState 获取接口状态
func (sm *StateManager) GetState(iface string) *InterfaceState {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	state, exists := sm.states[iface]
	if !exists {
		// 初始化状态
		state = &InterfaceState{
			Name:             iface,
			Latency:          0,
			PacketLoss:       100.0,
			BaseScore:        0,
			Cost:             0,
			FinalScore:       0,
			LastCheckTime:    time.Time{},
			InitialCheckDone: false,
		}
		sm.states[iface] = state
	}

	return state
}

// UpdateState 更新接口状态
// 返回: 是否完成初始检测（第一次检测）
func (sm *StateManager) UpdateState(iface string, checkResult *CheckResult, cost int) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	state, exists := sm.states[iface]
	if !exists {
		state = &InterfaceState{
			Name:             iface,
			InitialCheckDone: false,
		}
		sm.states[iface] = state
	}

	// 更新检查结果
	if checkResult.Success {
		state.Latency = checkResult.Latency
		state.PacketLoss = checkResult.PacketLoss
		state.LastTarget = checkResult.TargetIP
	} else {
		// 检测失败
		state.Latency = 0
		state.PacketLoss = 100.0
		state.LastTarget = ""
	}

	// 更新成本和评分
	state.Cost = cost
	state.BaseScore = calculateBaseScore(state.Latency, state.PacketLoss)
	state.FinalScore = calculateFinalScore(state.BaseScore, cost)
	state.LastCheckTime = time.Now()

	// 标记初始检测完成
	initialCheckDone := !state.InitialCheckDone
	if !state.InitialCheckDone {
		state.InitialCheckDone = true
	}

	return initialCheckDone
}

// AllInitialChecksDone 检查所有接口是否完成初始检测
func (sm *StateManager) AllInitialChecksDone(ifaces []string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for _, iface := range ifaces {
		state, exists := sm.states[iface]
		if !exists || !state.InitialCheckDone {
			return false
		}
	}
	return true
}

// GetAllStates 获取所有接口状态
func (sm *StateManager) GetAllStates() map[string]*InterfaceState {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	states := make(map[string]*InterfaceState)
	for k, v := range sm.states {
		// 深拷贝
		stateCopy := *v
		states[k] = &stateCopy
	}
	return states
}

// ResetAllStates 重置所有状态（配置重载时使用）
func (sm *StateManager) ResetAllStates() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.states = make(map[string]*InterfaceState)
}

// RecordEvent 记录事件
func (sm *StateManager) RecordEvent(monitorName, eventType, message string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	event := FailoverEvent{
		Timestamp:   time.Now(),
		MonitorName: monitorName,
		EventType:   eventType,
		Message:     message,
	}

	sm.events = append(sm.events, event)

	// 只保留最近20条
	if len(sm.events) > 20 {
		sm.events = sm.events[len(sm.events)-20:]
	}
}

// GetRecentEvents 获取最近的事件
func (sm *StateManager) GetRecentEvents(count int) []FailoverEvent {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if count > len(sm.events) {
		count = len(sm.events)
	}

	events := make([]FailoverEvent, count)
	start := len(sm.events) - count
	copy(events, sm.events[start:])

	return events
}

// SaveState 保存状态到文件
func (sm *StateManager) SaveState() error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	runtimeState := &RuntimeState{
		StartTime:       sm.startTime,
		InterfaceStates: sm.states,
		RecentEvents:    sm.events,
	}

	data, err := json.MarshalIndent(runtimeState, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %v", err)
	}

	// 确保目录存在
	dir := "/var/lib/trueword_node"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建状态目录失败: %v", err)
	}

	err = os.WriteFile(StateFile, data, 0644)
	if err != nil {
		return fmt.Errorf("写入状态文件失败: %v", err)
	}

	return nil
}

// LoadState 从文件加载状态
func LoadState() (*RuntimeState, error) {
	data, err := os.ReadFile(StateFile)
	if err != nil {
		return nil, fmt.Errorf("读取状态文件失败: %v", err)
	}

	var state RuntimeState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("解析状态文件失败: %v", err)
	}

	return &state, nil
}

// calculateBaseScore 计算基础评分（不含成本）
// 与 pkg/routing/policy.go 中的算法完全一致
func calculateBaseScore(latency, packetLoss float64) float64 {
	// 特殊情况：完全失败（100% 丢包）
	// 此时接口完全不可达，评分应该为 0
	if packetLoss >= 100.0 {
		return 0
	}

	var score float64

	// 丢包率评分（0-60分）
	if packetLoss == 0 {
		score += 60
	} else if packetLoss <= 5 {
		score += 45
	} else if packetLoss <= 10 {
		score += 30
	} else if packetLoss <= 20 {
		score += 15
	} else {
		score += 0
	}

	// 延迟评分（0-40分）
	// 注意：延迟为 0 说明没有成功的 ping，但已经被上面的丢包率检查过滤了
	if latency < 50 {
		score += 40
	} else if latency < 100 {
		score += 35
	} else if latency < 150 {
		score += 30
	} else if latency < 200 {
		score += 25
	} else if latency < 300 {
		score += 15
	} else {
		score += 5
	}

	return score
}

// calculateFinalScore 计算最终评分（含成本惩罚）
// 与 pkg/routing/policy.go 中的算法完全一致
func calculateFinalScore(baseScore float64, cost int) float64 {
	// 成本惩罚 = cost * 0.5
	costPenalty := float64(cost) * 0.5
	finalScore := baseScore - costPenalty

	// 确保最终评分不为负数
	if finalScore < 0 {
		finalScore = 0
	}

	return finalScore
}
