package api

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-crawler-client/config"
	"go-crawler-client/internal/auth"
	"go-crawler-client/internal/crawler"
	"go-crawler-client/internal/model"
	"go-crawler-client/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var TokenValidator *auth.TokenValidator

func StartTaskHandler(c *gin.Context) {
	mode := c.Param("mode")
	if mode != "image" && mode != "data" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mode"})
		return
	}

	var req model.StartTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Customize error message for missing Token
		if strings.Contains(err.Error(), "'Token' failed on the 'required' tag") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required field: token. Please initiate tasks via the backend API."})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Validate Token
	if TokenValidator != nil {
		claims, err := TokenValidator.ValidateTaskToken(req.Token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token: " + err.Error()})
			return
		}
		// Optional: Check if claims match request
		if claims.UserID != "" && claims.UserID != req.PixivUserID {
			// Note: In the backend we used the system UserID for the token, but here we are crawling a PixivUserID.
			// If the token was bound to a specific PixivUserID, we should check it.
			// For now, we just verify the token is valid and issued by our backend.
		}
	} else {
		// If validator is not initialized (e.g. missing public key), we might want to fail safe or warn
		// For security, better to fail if we expect auth
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server authentication configuration error"})
		return
	}

	// Get User Info (Sync)
	userInfo, err := crawler.GetUserInfo(req.PixivUserID, req.Cookie)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info: " + err.Error()})
		return
	}

	// Generate Task ID
	taskID := uuid.New().String()

	// Create Task
	task, err := service.GlobalTaskManager.AddTask(taskID, mode, userInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task: " + err.Error()})
		return
	}

	// Start Crawler (Async)
	go crawler.StartCrawler(task, req.Cookie)

	// Return Response immediately
	c.JSON(http.StatusOK, model.StartTaskResponse{
		Status:   "running",
		TaskID:   taskID,
		UserInfo: userInfo,
	})
}

func GetTaskStatusHandler(c *gin.Context) {
	taskID := c.Param("task_id")
	// search task in global manager
	task, ok := service.GlobalTaskManager.GetTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// return task snapshot
	// The GetSnapshot method packages all task information (status, logs, results) into JSON
	c.JSON(http.StatusOK, task.GetSnapshot())
}

func GetTaskLogsHandler(c *gin.Context) {
	taskID := c.Param("task_id")
	tailStr := c.Query("tail")
	tail := 50
	if tailStr != "" {
		if t, err := strconv.Atoi(tailStr); err == nil {
			tail = t
		}
	}

	task, ok := service.GlobalTaskManager.GetTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	logs, total := task.Logger.GetLogs(tail)
	c.JSON(http.StatusOK, model.LogResponse{
		TaskID:     taskID,
		Logs:       logs,
		TotalLines: total,
	})
}

func GetAvatarHandler(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	baseDir := config.GetBaseDir()
	// Assuming jpg for simplicity, in real app check file existence
	avatarPath := filepath.Join(baseDir, "crawl-datas", userID, ".avatars", userID+".jpg")
	c.File(avatarPath)
}

func HealthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, model.HealthResponse{
		Status:     "ok",
		BaseDir:    config.GetBaseDir(),
		TasksCount: service.GlobalTaskManager.Count(),
	})
}

func GetConfigHandler(c *gin.Context) {
	c.JSON(http.StatusOK, model.ConfigResponse{
		User: model.UserConfig{
			ServerURL: config.GlobalConfig.ServerURL,
			ProxyHost: config.GlobalConfig.ProxyHost,
			ProxyPort: config.GlobalConfig.ProxyPort,
		},
		Client: model.ClientConfig{
			Port:      config.GlobalConfig.Port,
			BaseDir:   config.GetBaseDir(),
			StartedAt: time.Now().Format(time.RFC3339), // This should be app start time
		},
	})
}
