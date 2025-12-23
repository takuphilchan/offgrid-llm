// Package logging provides structured logging for OffGrid LLM.
// This is a lightweight wrapper that can be gradually adopted across the codebase.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
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

// Entry represents a structured log entry
type Entry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// Logger provides structured logging
type Logger struct {
	mu       sync.Mutex
	output   io.Writer
	level    Level
	jsonMode bool
	fields   map[string]any
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Default returns the default logger
func Default() *Logger {
	once.Do(func() {
		defaultLogger = New(os.Stderr)
	})
	return defaultLogger
}

// New creates a new logger
func New(output io.Writer) *Logger {
	return &Logger{
		output: output,
		level:  LevelInfo,
		fields: make(map[string]any),
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) *Logger {
	l.level = level
	return l
}

// SetLevelFromString sets level from string (debug, info, warn, error)
func (l *Logger) SetLevelFromString(level string) *Logger {
	switch level {
	case "debug":
		l.level = LevelDebug
	case "info":
		l.level = LevelInfo
	case "warn", "warning":
		l.level = LevelWarn
	case "error":
		l.level = LevelError
	}
	return l
}

// SetJSON enables JSON output mode
func (l *Logger) SetJSON(enabled bool) *Logger {
	l.jsonMode = enabled
	return l
}

// With returns a new logger with additional fields
func (l *Logger) With(fields map[string]any) *Logger {
	newLogger := &Logger{
		output:   l.output,
		level:    l.level,
		jsonMode: l.jsonMode,
		fields:   make(map[string]any),
	}
	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...map[string]any) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...map[string]any) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...map[string]any) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...map[string]any) {
	l.log(LevelError, msg, fields...)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...any) {
	l.log(LevelDebug, fmt.Sprintf(format, args...))
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...any) {
	l.log(LevelInfo, fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...any) {
	l.log(LevelWarn, fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...any) {
	l.log(LevelError, fmt.Sprintf(format, args...))
}

func (l *Logger) log(level Level, msg string, fields ...map[string]any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Merge fields
	allFields := make(map[string]any)
	for k, v := range l.fields {
		allFields[k] = v
	}
	for _, f := range fields {
		for k, v := range f {
			allFields[k] = v
		}
	}

	if l.jsonMode {
		entry := Entry{
			Time:    time.Now().UTC(),
			Level:   level.String(),
			Message: msg,
			Fields:  allFields,
		}
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.output, string(data))
	} else {
		// Human-readable format
		timestamp := time.Now().Format("15:04:05")
		levelStr := fmt.Sprintf("%-5s", level.String())

		if len(allFields) > 0 {
			fieldStr := ""
			for k, v := range allFields {
				fieldStr += fmt.Sprintf(" %s=%v", k, v)
			}
			fmt.Fprintf(l.output, "%s %s %s%s\n", timestamp, levelStr, msg, fieldStr)
		} else {
			fmt.Fprintf(l.output, "%s %s %s\n", timestamp, levelStr, msg)
		}
	}
}

// Package-level convenience functions using the default logger

// Debug logs a debug message
func Debug(msg string, fields ...map[string]any) {
	Default().Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...map[string]any) {
	Default().Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...map[string]any) {
	Default().Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...map[string]any) {
	Default().Error(msg, fields...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...any) {
	Default().Debugf(format, args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...any) {
	Default().Infof(format, args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...any) {
	Default().Warnf(format, args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...any) {
	Default().Errorf(format, args...)
}

// SetLevel sets the default logger level
func SetLevel(level Level) {
	Default().SetLevel(level)
}

// SetJSON enables JSON mode on the default logger
func SetJSON(enabled bool) {
	Default().SetJSON(enabled)
}
