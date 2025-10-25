package failover

import (
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Logger 日志管理器
type Logger struct {
	level      LogLevel
	file       *os.File
	debugMode  bool
}

// NewLogger 创建日志管理器
func NewLogger(logFile string, debugMode bool) (*Logger, error) {
	logger := &Logger{
		level:     LogLevelInfo,
		debugMode: debugMode,
	}

	if debugMode {
		logger.level = LogLevelDebug
	}

	// 只有指定了日志文件才打开文件
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %v", err)
		}
		logger.file = file
	}

	return logger, nil
}

// Close 关闭日志文件
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// Debug 输出调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= LogLevelDebug {
		l.log("DEBUG", format, args...)
	}
}

// Info 输出信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= LogLevelInfo {
		l.log("INFO", format, args...)
	}
}

// Warn 输出警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= LogLevelWarn {
		l.log("WARN", format, args...)
	}
}

// Error 输出错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= LogLevelError {
		l.log("ERROR", format, args...)
	}
}

// log 内部日志输出函数
func (l *Logger) log(level string, format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("%s [%s] %s\n", timestamp, level, message)

	// Debug模式：输出到stdout（前台运行）
	if l.debugMode {
		fmt.Print(logLine)
	}

	// 如果指定了日志文件，写入文件
	if l.file != nil {
		l.file.WriteString(logLine)
	}

	// 如果既不是debug模式，也没有日志文件，则不输出任何内容
	// （守护进程静默运行）
}

// Writer 返回一个io.Writer，用于标准库日志
func (l *Logger) Writer() io.Writer {
	if l.debugMode {
		return os.Stdout
	}
	if l.file != nil {
		return l.file
	}
	return io.Discard
}
