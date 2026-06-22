package photo

import (
	"github.com/gin-gonic/gin"
)

// Register 注册图片处理相关的全部 API 路由
func Register(app *gin.RouterGroup) {
	// 静态文件服务：/storage/uploads/* → 本地 storage/uploads/ 目录
	app.Static("/storage/uploads", "./storage/uploads")
	app.Static("/storage/results", "./storage/results")

	group := app.Group("/photo")

	// 图片管理
	group.POST("/upload", UploadImagesAPI)
	group.POST("/upload/folder", UploadFolderAPI)
	group.GET("/images", ListImagesAPI)
	group.GET("/images/:id", GetImageAPI)
	group.DELETE("/images/:id", DeleteImageAPI)

	// 处理功能 (统一入口)
	group.POST("/process", ProcessAPI)

	// 任务管理
	group.GET("/tasks", ListTasksAPI)
	group.GET("/tasks/:id", GetTaskAPI)
	group.DELETE("/tasks/:id", DeleteTaskAPI)
	group.POST("/tasks/:id/retry", RetryTaskAPI)

	// 配置
	group.GET("/prompts", GetPromptsAPI)

	// 下载
	group.GET("/download/file", DownloadFileAPI)
	group.GET("/download/zip", DownloadZipAPI)
}
