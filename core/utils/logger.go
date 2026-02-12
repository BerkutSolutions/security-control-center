package utils

import (
	"log"
	"os"
)

type Logger struct {
	*log.Logger
}

func NewLogger() *Logger {
	return &Logger{Logger: log.New(os.Stdout, "[berkut] ", log.LstdFlags|log.Lshortfile)}
}

func (l *Logger) Errorf(format string, v ...any) {
	l.Printf("ERROR: "+format, v...)
}

func (l *Logger) Fatalf(format string, v ...any) {
	l.Logger.Fatalf("FATAL: "+format, v...)
}
