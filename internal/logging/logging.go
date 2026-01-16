package logging // 简单日志封装，方便全局统一控制日志级别和格式

import (
	"fmt"     // 标准库：格式化字符串
	"io"      // 标准库：通用 Writer 接口
	"log"     // 标准库：基础日志输出
	"strings" // 标准库：字符串处理
)

// Level 表示日志级别，数值越大表示级别越高
type Level int

const (
	Debug Level = iota // 调试信息，最详细
	Info               // 一般信息
	Warn               // 警告信息
	Error              // 错误信息
)

// Logger 封装标准库 log.Logger，并增加最小日志级别过滤
type Logger struct {
	l   *log.Logger // 底层日志对象
	min Level       // 最小输出级别，小于该级别的日志将被忽略
}

// New 创建一个新的 Logger，w 为输出目标（例如 os.Stdout），min 为最小日志级别
func New(w io.Writer, min Level) *Logger {
	return &Logger{
		l:   log.New(w, "", log.LstdFlags), // 使用标准时间戳前缀
		min: min,
	}
}

// log 根据级别输出一条日志，如果级别低于最小级别则直接丢弃
func (lg *Logger) log(level Level, format string, args ...interface{}) {
	if level < lg.min { // 小于最小级别的日志直接忽略
		return
	}
	var tag string
	switch level { // 不同级别对应不同标签
	case Debug:
		tag = "DEBUG"
	case Info:
		tag = "INFO"
	case Warn:
		tag = "WARN"
	case Error:
		tag = "ERROR"
	default:
		tag = "INFO"
	}
	msg := fmt.Sprintf(format, args...)                 // 格式化用户传入的消息
	lg.l.Printf("[%s] %s", tag, strings.TrimSpace(msg)) // 最终输出格式：[LEVEL] message
}

// Debug 输出 DEBUG 级别日志
func (lg *Logger) Debug(format string, args ...interface{}) { lg.log(Debug, format, args...) }

// Info 输出 INFO 级别日志
func (lg *Logger) Info(format string, args ...interface{}) { lg.log(Info, format, args...) }

// Warn 输出 WARN 级别日志
func (lg *Logger) Warn(format string, args ...interface{}) { lg.log(Warn, format, args...) }

// Error 输出 ERROR 级别日志
func (lg *Logger) Error(format string, args ...interface{}) { lg.log(Error, format, args...) }
