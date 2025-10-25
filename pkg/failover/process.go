package failover

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

const (
	PIDFile = "/var/run/twnode-failover.pid"
)

// AcquirePIDLock 获取PID文件锁，防止多实例运行
func AcquirePIDLock() error {
	// 检查PID文件是否存在
	if pidBytes, err := os.ReadFile(PIDFile); err == nil {
		oldPIDStr := strings.TrimSpace(string(pidBytes))
		oldPID, err := strconv.Atoi(oldPIDStr)
		if err == nil {
			// 检查进程是否还在运行
			if processExists(oldPID) {
				return fmt.Errorf("守护进程已在运行 (PID: %d)", oldPID)
			} else {
				// 僵尸PID文件，删除
				os.Remove(PIDFile)
			}
		}
	}

	// 写入当前PID
	pid := os.Getpid()
	pidStr := fmt.Sprintf("%d\n", pid)
	err := os.WriteFile(PIDFile, []byte(pidStr), 0644)
	if err != nil {
		return fmt.Errorf("写入PID文件失败: %v", err)
	}

	return nil
}

// ReleasePIDLock 释放PID文件锁
func ReleasePIDLock() {
	os.Remove(PIDFile)
}

// GetRunningPID 获取正在运行的守护进程PID
func GetRunningPID() (int, error) {
	pidBytes, err := os.ReadFile(PIDFile)
	if err != nil {
		return 0, fmt.Errorf("PID文件不存在，守护进程未运行")
	}

	pidStr := strings.TrimSpace(string(pidBytes))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("PID文件格式错误: %v", err)
	}

	// 检查进程是否还在运行
	if !processExists(pid) {
		// 僵尸PID文件
		os.Remove(PIDFile)
		return 0, fmt.Errorf("PID文件存在但进程不存在，守护进程未运行")
	}

	return pid, nil
}

// SendSignal 向守护进程发送信号
func SendSignal(sig syscall.Signal) error {
	pid, err := GetRunningPID()
	if err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("查找进程失败: %v", err)
	}

	err = process.Signal(sig)
	if err != nil {
		return fmt.Errorf("发送信号失败: %v", err)
	}

	return nil
}

// processExists 检查进程是否存在
func processExists(pid int) bool {
	// 发送信号0检测进程是否存在（不会真正发送信号）
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	return true
}
