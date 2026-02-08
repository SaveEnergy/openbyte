package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger struct {
	level  Level
	logger *log.Logger
	mu     sync.RWMutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func Init(level Level) {
	once.Do(func() {
		defaultLogger = &Logger{
			level:  level,
			logger: log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds),
		}
	})
}

func GetLogger() *Logger {
	Init(LevelInfo) // once.Do is a no-op if already initialized; provides memory barrier
	return defaultLogger
}

func NewLogger(name string) *Logger {
	return &Logger{
		level:  GetLogger().level,
		logger: log.New(os.Stderr, "["+name+"] ", log.LstdFlags|log.Lmicroseconds),
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

func (l *Logger) log(level Level, msg string, fields ...Field) {
	l.mu.RLock()
	currentLevel := l.level
	l.mu.RUnlock()

	if level < currentLevel {
		return
	}

	levelStr := levelString(level)
	fieldStr := formatFields(fields)

	if fieldStr != "" {
		l.logger.Printf("[%s] %s %s", levelStr, msg, fieldStr)
	} else {
		l.logger.Printf("[%s] %s", levelStr, msg)
	}
}

func levelString(level Level) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Field struct {
	Key   string
	Value interface{}
}

func formatFields(fields []Field) string {
	if len(fields) == 0 {
		return ""
	}

	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(f.Key)
		b.WriteByte('=')
		b.WriteString(FormatValue(f.Value))
	}
	return b.String()
}

func FormatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int64:
		return formatInt(val)
	case uint, uint64:
		return formatUint(val)
	case float32:
		return formatFloat(float64(val))
	case float64:
		return formatFloat(val)
	case time.Duration:
		return val.String()
	case time.Time:
		return val.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return val.String()
	case error:
		return val.Error()
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatInt(v interface{}) string {
	switch val := v.(type) {
	case int:
		return formatInt64(int64(val))
	case int64:
		return formatInt64(val)
	default:
		return ""
	}
}

func formatInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}

func formatUint(v interface{}) string {
	switch val := v.(type) {
	case uint:
		return formatUint64(uint64(val))
	case uint64:
		return formatUint64(val)
	default:
		return ""
	}
}

func formatUint64(v uint64) string {
	return fmt.Sprintf("%d", v)
}

func formatFloat(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func Debug(msg string, fields ...Field) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	GetLogger().Error(msg, fields...)
}
