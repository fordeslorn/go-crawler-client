package api

import (
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	v1 := r.Group("/api/v1/crawler")
	{
		v1.POST("/start/:mode", StartTaskHandler)
		v1.GET("/status/:task_id", GetTaskStatusHandler)
		v1.GET("/logs/:task_id", GetTaskLogsHandler)
		v1.GET("/avatars/:pixiv_user_id", GetAvatarHandler)
		v1.GET("/health", HealthCheckHandler)
		v1.GET("/config", GetConfigHandler)
	}

	return r
}
