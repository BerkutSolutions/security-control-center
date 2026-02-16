package utils

import (
	"fmt"
	"log/slog"
	"os"
)

type Logger struct {
	slog *slog.Logger
}

func NewLogger() *Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	})
	return &Logger{slog: slog.New(handler)}
}

func (l *Logger) Printf(format string, v ...any) {
	if l == nil || l.slog == nil {
		return
	}
	l.slog.Info(fmt.Sprintf(format, v...))
}

func (l *Logger) Println(v ...any) {
	if l == nil || l.slog == nil {
		return
	}
	l.slog.Info(fmt.Sprint(v...))
}

func (l *Logger) Errorf(format string, v ...any) {
	if l == nil || l.slog == nil {
		return
	}
	l.slog.Error(fmt.Sprintf(format, v...))
}

func (l *Logger) Fatalf(format string, v ...any) {
	if l == nil || l.slog == nil {
		os.Exit(1)
	}
	l.slog.Error(fmt.Sprintf("FATAL: "+format, v...))
	os.Exit(1)
}
