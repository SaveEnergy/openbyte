package logging

import (
	"context"
	"log/slog"
)

// Field is a structured log attribute.
type Field struct {
	Key   string
	Value any
}

func Info(msg string, fields ...Field) {
	log(slog.LevelInfo, msg, fields)
}

func Warn(msg string, fields ...Field) {
	log(slog.LevelWarn, msg, fields)
}

func Error(msg string, fields ...Field) {
	log(slog.LevelError, msg, fields)
}

func log(level slog.Level, msg string, fields []Field) {
	attrs := make([]slog.Attr, len(fields))
	for i, field := range fields {
		attrs[i] = slog.Any(field.Key, field.Value)
	}
	slog.LogAttrs(context.Background(), level, msg, attrs...)
}
