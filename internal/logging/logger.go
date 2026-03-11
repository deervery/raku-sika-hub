package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
)

// ParseLevel converts a string to a Level. Defaults to LevelInfo.
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "ERROR":
		return LevelError
	case "WARN":
		return LevelWarn
	default:
		return LevelInfo
	}
}

// Logger provides monthly-rotated logging with automatic cleanup of files older than 1 year.
type Logger struct {
	mu       sync.Mutex
	dir      string
	level    Level
	file     *os.File
	writer   *log.Logger
	curMonth string
}

// LogDir returns the default log directory.
func LogDir() string {
	return "logs"
}

// New creates a new Logger writing to the given directory with the specified level.
func New(dir string, level Level) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	l := &Logger{
		dir:   dir,
		level: level,
	}
	if err := l.rotate(time.Now()); err != nil {
		return nil, err
	}

	l.cleanup()
	return l, nil
}

// Close closes the current log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Info logs at INFO level.
func (l *Logger) Info(format string, args ...any) {
	l.logf(LevelInfo, "INFO", format, args...)
}

// Warn logs at WARN level.
func (l *Logger) Warn(format string, args ...any) {
	l.logf(LevelWarn, "WARN", format, args...)
}

// Error logs at ERROR level.
func (l *Logger) Error(format string, args ...any) {
	l.logf(LevelError, "ERROR", format, args...)
}

// Writer returns the underlying io.Writer for use with standard log or other libraries.
func (l *Logger) Writer() io.Writer {
	return &logWriter{l: l, level: LevelInfo, prefix: "INFO"}
}

func (l *Logger) logf(level Level, prefix, format string, args ...any) {
	if level > l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	month := now.Format("2006-01")
	if month != l.curMonth {
		if err := l.rotate(now); err != nil {
			fmt.Fprintf(os.Stderr, "log rotate error: %v\n", err)
			return
		}
	}

	msg := fmt.Sprintf(format, args...)
	l.writer.Printf("[%s] %s", prefix, msg)
}

func (l *Logger) rotate(now time.Time) error {
	if l.file != nil {
		l.file.Close()
	}

	month := now.Format("2006-01")
	name := fmt.Sprintf("service-%s.log", month)
	path := filepath.Join(l.dir, name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	l.file = f
	l.curMonth = month
	l.writer = log.New(f, "", log.LstdFlags)
	return nil
}

func (l *Logger) cleanup() {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(-1, 0, 0)

	type logFile struct {
		name  string
		month time.Time
	}
	var files []logFile

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "service-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		monthStr := strings.TrimPrefix(name, "service-")
		monthStr = strings.TrimSuffix(monthStr, ".log")
		t, err := time.Parse("2006-01", monthStr)
		if err != nil {
			continue
		}
		files = append(files, logFile{name: name, month: t})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].month.Before(files[j].month)
	})

	for _, f := range files {
		if f.month.Before(cutoff) {
			os.Remove(filepath.Join(l.dir, f.name))
		}
	}
}

type logWriter struct {
	l      *Logger
	level  Level
	prefix string
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.l.logf(w.level, w.prefix, "%s", strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
