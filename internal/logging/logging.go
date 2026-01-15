package logging

import (
	"fmt"
	"io"
	"log"
	"strings"
)

type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

type Logger struct {
	l   *log.Logger
	min Level
}

func New(w io.Writer, min Level) *Logger {
	return &Logger{
		l:   log.New(w, "", log.LstdFlags),
		min: min,
	}
}

func (lg *Logger) log(level Level, format string, args ...interface{}) {
	if level < lg.min {
		return
	}
	var tag string
	switch level {
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
	msg := fmt.Sprintf(format, args...)
	lg.l.Printf("[%s] %s", tag, strings.TrimSpace(msg))
}

func (lg *Logger) Debug(format string, args ...interface{}) { lg.log(Debug, format, args...) }
func (lg *Logger) Info(format string, args ...interface{})  { lg.log(Info, format, args...) }
func (lg *Logger) Warn(format string, args ...interface{})  { lg.log(Warn, format, args...) }
func (lg *Logger) Error(format string, args ...interface{}) { lg.log(Error, format, args...) }
