package logger

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type TaskLogger struct {
	mu       sync.RWMutex // read-write lock
	logs     []string     // log list in memory
	file     *os.File     // opened file handle
	filePath string
}

func NewTaskLogger(filePath string) (*TaskLogger, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &TaskLogger{
		logs:     make([]string, 0),
		file:     f,
		filePath: filePath,
	}, nil
}

func (l *TaskLogger) Info(format string, v ...any) {
	l.log("INFO", format, v...)
}

func (l *TaskLogger) Error(format string, v ...any) {
	l.log("ERROR", format, v...)
}

func (l *TaskLogger) log(level, format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("%s [%s] %s", timestamp, level, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.logs = append(l.logs, logLine)
	if l.file != nil {
		l.file.WriteString(logLine + "\n")
	}
	// Print to stdout as well for debugging
	// fmt.Println(logLine)
}

func (l *TaskLogger) GetLogs(tail int) ([]string, int) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := len(l.logs)
	if tail <= 0 || tail > total {
		// Return all if tail is 0 or larger than total
		// But usually tail means "last N lines"
		// If tail is 0, maybe return all? The doc says default 50.
		// Let's assume the caller handles the default.
		// If tail > total, return all.
		return l.logs, total
	}

	return l.logs[total-tail:], total
}

func (l *TaskLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}
