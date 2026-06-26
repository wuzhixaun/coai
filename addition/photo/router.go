package photo

import (
	"chat/globals"

	"github.com/gin-gonic/gin"
)

// Register 注册图片处理相关的全部 API 路由
func Register(app *gin.RouterGroup) {
	// 静态文件服务：挂载点固定为 /storage/uploads、/storage/results，
	// 底层目录引用 globals（可经 config.yaml 覆盖），与生成/清理逻辑保持一致。
	app.Static("/storage/uploads", globals.StorageUploadDir)
	app.Static("/storage/results", globals.StorageResultDir)

	group := app.Group("/photo")

	// 图片管理
	group.POST("/upload", UploadImagesAPI)
	group.POST("/upload/folder", UploadFolderAPI)
	group.POST("/fetch-url", FetchURLAPI)
	group.GET("/images", ListImagesAPI)
	group.GET("/images/:id", GetImageAPI)
	group.DELETE("/images/:id", DeleteImageAPI)

	// 一致性身份（商品/模特）
	group.POST("/identity", CreateIdentityAPI)
	group.GET("/identity", ListIdentitiesAPI)
	group.DELETE("/identity/:id", DeleteIdentityAPI)

	// 处理功能 (统一入口)
	group.POST("/process", ProcessAPI)

	// 一键成套素材工作流
	group.GET("/workflow/templates", ListWorkflowTemplatesAPI)
	group.POST("/workflow", WorkflowAPI)

	// 配方（保存/复用工作流）
	group.POST("/recipe", CreateRecipeAPI)
	group.GET("/recipe", ListRecipesAPI)
	group.DELETE("/recipe/:id", DeleteRecipeAPI)

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
