package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go-crawler-client/config"
	"go-crawler-client/internal/model"
	"go-crawler-client/internal/pkg/logger"
)

type Task struct {
	ID       string
	Status   string // running, completed, failed
	Mode     string // image or data
	UserInfo model.UserInfo
	Logger   *logger.TaskLogger // every task has its own logger
	Results  []model.TaskResult // crawled data results
	Images   []model.ImageInfo  // downloaded image information
	mu       sync.RWMutex       // task-level lock to protect concurrent read/write of Results and Images
}

// TaskManager manages all tasks
type TaskManager struct {
	// sync.Map is a concurrent safe map provided by Go standard library
	// Suitable for scenarios with many reads and few writes, here it stores the mapping from taskID to *Task
	tasks sync.Map
}

var GlobalTaskManager *TaskManager

func InitTaskManager() {
	GlobalTaskManager = &TaskManager{}
	// Initialize directories
	dirs := []string{
		".avatars",
		".task_data",
		".task_logs",
		".download_imgs",
		".task_results",
	}
	baseDir := config.GetBaseDir()
	for _, d := range dirs {
		path := filepath.Join(baseDir, d)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.MkdirAll(path, 0755)
		}
	}
}

func (tm *TaskManager) AddTask(taskID string, mode string, userInfo model.UserInfo) (*Task, error) {
	baseDir := config.GetBaseDir()
	logPath := filepath.Join(baseDir, ".task_logs", fmt.Sprintf("task_%s.log", taskID))

	// Initialize logger
	l, err := logger.NewTaskLogger(logPath)
	if err != nil {
		return nil, err
	}

	task := &Task{
		ID:       taskID,
		Status:   "running",
		Mode:     mode,
		UserInfo: userInfo,
		Logger:   l,
		Results:  make([]model.TaskResult, 0),
		Images:   make([]model.ImageInfo, 0),
	}

	// Store in sync.Map
	tm.tasks.Store(taskID, task)
	return task, nil
}

func (tm *TaskManager) GetTask(taskID string) (*Task, bool) {
	val, ok := tm.tasks.Load(taskID)
	if !ok {
		return nil, false
	}
	return val.(*Task), true
}

func (tm *TaskManager) Count() int {
	count := 0
	tm.tasks.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (t *Task) UpdateStatus(status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
}

func (t *Task) AddResult(result model.TaskResult) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Results = append(t.Results, result)
}

func (t *Task) AddImage(image model.ImageInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Images = append(t.Images, image)
}

func (t *Task) GetSnapshot() model.TaskStatusResponse {
	// Acquire read lock: prevent conflicts when reading data while the crawler is writing new data
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Get logs (last 50 by default for snapshot, but here we just return empty or let the specific log endpoint handle it)
	// The requirement says "logs": ["log line 1", "log line 2"], // 返回最近 50 条日志
	logs, _ := t.Logger.GetLogs(50)

	resp := model.TaskStatusResponse{
		Status:   t.Status,
		Mode:     t.Mode,
		UserInfo: t.UserInfo,
		Logs:     logs,
	}

	// Only return full results when the task is completed
	// This helps reduce the amount of data transferred and avoids returning huge JSON while running
	if t.Status == "completed" {
		resp.Results = t.Results
		resp.Images = t.Images
	} else {
		// Initialize empty slices to avoid null in JSON
		resp.Results = []model.TaskResult{}
		resp.Images = []model.ImageInfo{}
	}

	return resp
}
