package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger provides structured logging with levels.
type Logger struct {
	level  LogLevel
	output io.Writer
	mu     sync.Mutex
}

// DefaultLogger is the package-level logger.
var DefaultLogger = NewLogger(LevelInfo)

// NewLogger creates a new logger with the specified level.
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		output: os.Stdout,
	}
}

// SetLevel changes the logging level.
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Debugf logs a debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.log("DEBUG", format, args...)
	}
}

// Infof logs an info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.log("INFO", format, args...)
	}
}

// Warnf logs a warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.log("WARN", format, args...)
	}
}

// Errorf logs an error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.level <= LevelError {
		l.log("ERROR", format, args...)
	}
}

func (l *Logger) log(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.output, "[%s] %s\n", level, msg)
}

// SetVerbose enables or disables verbose (debug) logging.
func SetVerbose(verbose bool) {
	if verbose {
		DefaultLogger.SetLevel(LevelDebug)
	} else {
		DefaultLogger.SetLevel(LevelInfo)
	}
}

// Debugf logs a debug message using the default logger.
func Debugf(format string, args ...interface{}) {
	DefaultLogger.Debugf(format, args...)
}

// Infof logs an info message using the default logger.
func Infof(format string, args ...interface{}) {
	DefaultLogger.Infof(format, args...)
}

// Warnf logs a warning message using the default logger.
func Warnf(format string, args ...interface{}) {
	DefaultLogger.Warnf(format, args...)
}
