package photo

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chat/auth"
	"chat/globals"
	"chat/manager"
	"chat/utils"

	"github.com/gin-gonic/gin"
)

// ── 辅助 ────────────────────────────────────────────────────

func getDBFromCtx(c *gin.Context) *sql.DB { return utils.GetDBFromContext(c) }

func getUserID(c *gin.Context) int64 {
	db := getDBFromCtx(c)
	user := manager.ParseAuth(c, c.GetHeader("Authorization"))
	if user == nil {
		return 0
	}
	return auth.GetId(db, user)
}

func nullToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func parseJSONStringArray(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "null" || jsonStr == "[]" {
		return nil
	}
	arr, err := utils.UnmarshalString[[]string](jsonStr)
	if err != nil {
		return nil
	}
	return arr
}

// ── 配置 ────────────────────────────────────────────────────

func GetPromptsAPI(c *gin.Context) {
	config := GetFeaturesConfig()
	if config == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "提示词配置未加载"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// ── 图片上传 ────────────────────────────────────────────────

func UploadImagesAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	userID := getUserID(c)

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "不是有效的文件上传请求"})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "至少需要上传一张图片"})
		return
	}

	folderName := c.PostForm("folder_name")
	var results []ImageInfo
	for _, f := range files {
		info, err := SaveUploadFile(f, db, userID, folderName)
		if err != nil {
			continue
		}
		if info != nil {
			if info.CreatedAt == "" {
				info.CreatedAt = time.Now().Format(time.RFC3339)
			}
			results = append(results, *info)
		}
	}
	for i := range results {
		db.Exec("UPDATE photo_images SET created_at = CURRENT_TIMESTAMP WHERE id = ?", results[i].Id)
	}
	c.JSON(http.StatusOK, results)
}

func UploadFolderAPI(c *gin.Context) { UploadImagesAPI(c) }

func ListImagesAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	userID := getUserID(c)
	images, err := queryImagesByUser(db, userID, 100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询失败"})
		return
	}
	if images == nil {
		images = []ImageInfo{}
	}
	c.JSON(http.StatusOK, images)
}

func GetImageAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	img, err := queryImageByID(db, c.Param("id"), getUserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "图片不存在"})
		return
	}
	c.JSON(http.StatusOK, img)
}

func DeleteImageAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	if err := DeleteImageFile(db, c.Param("id"), getUserID(c)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": c.Param("id")})
}

// ── 处理入口 ────────────────────────────────────────────────

func ProcessAPI(c *gin.Context) {
	var req ProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请求参数错误"})
		return
	}
	if len(req.ImageIds) == 0 || len(req.Features) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请选择图片和功能"})
		return
	}

	db := getDBFromCtx(c)
	userID := getUserID(c)

	// 单用户在途任务并发限制：防止同一用户批量提交（图片数×功能数×生成数）压垮后端。
	// 上限为 0 表示不限制。
	if limit := globals.ImageMaxConcurrentPerUser; limit > 0 {
		var inflight int64
		_ = db.QueryRow(
			`SELECT COUNT(*) FROM photo_tasks WHERE user_id = ? AND status IN ('pending','processing')`,
			userID,
		).Scan(&inflight)
		if inflight+int64(len(req.Features)) > limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":  "error",
				"message": "当前进行中的图片任务过多，请等待部分任务完成后再试",
			})
			return
		}
	}

	// 获取用户 group（用于渠道匹配）
	user := manager.ParseAuth(c, c.GetHeader("Authorization"))
	group := auth.GetGroup(db, user)

	imagePaths, err := ResolveImagePaths(db, req.ImageIds, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var filenames []string
	folderName := ""
	for _, id := range req.ImageIds {
		img, _ := queryImageByID(db, id, userID)
		if img != nil {
			filenames = append(filenames, img.Filename)
			if folderName == "" && img.FolderName != "" {
				folderName = img.FolderName
			}
		}
	}

	// Logo 定制：把前端传入的 logo 图片 id 解析成本地路径，供处理时读取为第二张参考图。
	if logoID := getStringParam(req.Params, "logo_image_id", ""); logoID != "" {
		if logoPaths, e := ResolveImagePaths(db, []string{logoID}, userID); e == nil && len(logoPaths) > 0 {
			if req.Params == nil {
				req.Params = map[string]interface{}{}
			}
			req.Params["logo_path"] = logoPaths[0]
		}
	}

	// 一致性身份 + 品牌资产：解析为注入参数（参考图路径 / 锁定 seed / 主体描述），全局套用。
	identityRefPaths, identitySeed, identitySubject := resolveInjection(db, userID, req.IdentityId, req.BrandKitId)

	responses := make([]TaskInfo, 0, len(req.Features))
	for _, feature := range req.Features {
		isVideo := feature == FeatureVideoGen
		totalVideos := 0
		totalImages := len(req.ImageIds)
		if isVideo {
			totalVideos = 1
		} else if IsAIFeature(feature) && supportsGenerateCount(feature) {
			totalImages = len(req.ImageIds) * clampGenerateCount(getIntParam(req.Params, "image_count", 1))
		}
		taskID := generateImageID()

		_, err := db.Exec(`INSERT INTO photo_tasks
			(task_id, user_id, feature, status, image_ids, params, total_images, total_videos,
			 source_filenames, source_paths, folder_name)
			VALUES (?, ?, ?, 'pending', ?, ?, ?, ?, ?, ?, ?)`,
			taskID, userID, feature,
			utils.Marshal(req.ImageIds), utils.Marshal(req.Params),
			totalImages, totalVideos,
			utils.Marshal(filenames), utils.Marshal(imagePaths), folderName,
		)
		if err != nil {
			responses = append(responses, TaskInfo{TaskId: taskID, Feature: feature, Status: TaskStatusFailed, ErrorMessage: err.Error()})
			continue
		}

		responses = append(responses, TaskInfo{
			TaskId: taskID, Feature: feature, Status: TaskStatusPending,
			ImageIds: req.ImageIds, TotalImages: totalImages, TotalVideos: totalVideos,
			SourceFilenames: filenames, CreatedAt: time.Now().Format(time.RFC3339), FolderName: folderName,
		})
		go ProcessTask(nil, db, taskID, feature, imagePaths, req.Params, req.ChannelOverride, group, identityRefPaths, identitySeed, identitySubject)
	}
	c.JSON(http.StatusOK, responses)
}

// ── 任务 scan 复用 ──────────────────────────────────────────

func scanTaskRow(scanner interface{ Scan(...interface{}) error }) (TaskInfo, error) {
	var t TaskInfo
	var n1, n2, n3, n4, n5, n6 sql.NullString
	var n7, n8, n9 sql.NullString

	err := scanner.Scan(
		&t.TaskId, &t.Feature, &t.Status, &n1, &n2, &n6, &t.Progress,
		&t.TotalImages, &t.ProcessedImages, &t.TotalVideos, &t.ProcessedVideos,
		&n3, &n4, &n5, &t.FolderName, &n7, &n8, &n9,
	)
	if err != nil {
		return t, err
	}

	t.ErrorMessage = nullToString(n6)
	t.ImageIds = parseJSONStringArray(nullToString(n1))
	t.ResultUrls = parseJSONStringArray(nullToString(n2))
	t.SubmitIds = parseJSONStringArray(nullToString(n3))
	t.SourceFilenames = parseJSONStringArray(nullToString(n4))
	if n7.Valid {
		t.CreatedAt = n7.String
	}
	if n8.Valid {
		t.CompletedAt = n8.String
	}
	if items, e := utils.UnmarshalString[[]ItemStatus](nullToString(n9)); e == nil {
		t.ItemStatus = items
	}
	return t, nil
}

// ── 任务管理 ────────────────────────────────────────────────

func ListTasksAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	rows, err := db.Query(taskSelectSQL+" WHERE user_id = ? ORDER BY created_at DESC LIMIT 100", getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error"})
		return
	}
	defer rows.Close()

	tasks := make([]TaskInfo, 0)
	for rows.Next() {
		if t, err := scanTaskRow(rows); err == nil {
			tasks = append(tasks, t)
		}
	}
	c.JSON(http.StatusOK, tasks)
}

func GetTaskAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	t, err := scanTaskRow(db.QueryRow(taskSelectSQL+" WHERE task_id = ?", c.Param("id")))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, t)
}

func DeleteTaskAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	taskID := c.Param("id")

	// 清理结果文件
	var urlsNS sql.NullString
	db.QueryRow("SELECT result_urls FROM photo_tasks WHERE task_id = ?", taskID).Scan(&urlsNS)
	for _, url := range parseJSONStringArray(nullToString(urlsNS)) {
		os.Remove(filepath.Join(ResultDir(), filepath.Base(url)))
	}

	if _, err := db.Exec("DELETE FROM photo_tasks WHERE task_id = ?", taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "task_id": taskID})
}

func RetryTaskAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	userID := getUserID(c)
	taskID := c.Param("id")

	var feature, imgIDsStr, pathsStr string
	var paramsNS, itemNS sql.NullString
	err := db.QueryRow(
		"SELECT feature, image_ids, params, source_paths, item_status FROM photo_tasks WHERE task_id = ?",
		taskID,
	).Scan(&feature, &imgIDsStr, &paramsNS, &pathsStr, &itemNS)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "任务不存在"})
		return
	}
	// 复原原始参数（此前重试传 nil 会丢参数）与逐图状态（用于只重试失败项）
	params, _ := utils.UnmarshalString[map[string]interface{}](nullToString(paramsNS))
	prevItems, _ := utils.UnmarshalString[[]ItemStatus](nullToString(itemNS))

	// 解析图片路径
	var imagePaths []string
	savedPaths := parseJSONStringArray(pathsStr)
	allExist := true
	for _, p := range savedPaths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			allExist = false
			break
		}
	}
	if allExist && len(savedPaths) > 0 {
		imagePaths = savedPaths
	} else {
		paths, err := ResolveImagePaths(db, parseJSONStringArray(imgIDsStr), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "源图片已丢失"})
			return
		}
		imagePaths = paths
	}

	// 获取用户 group
	user := manager.ParseAuth(c, c.GetHeader("Authorization"))
	userGroup := auth.GetGroup(db, user)

	// 重置并异步重试（只重跑失败项；保留原参数）
	// 注：当前 photo_tasks 未持久化 identity_id，重试暂不重新套用一致性身份（后续增强）。
	db.Exec("UPDATE photo_tasks SET status = ?, progress = 0, error_message = '' WHERE task_id = ?",
		TaskStatusPending, taskID)
	go ProcessRetryTask(db, taskID, feature, imagePaths, params, "", userGroup, prevItems)

	t, _ := scanTaskRow(db.QueryRow(taskSelectSQL+" WHERE task_id = ?", taskID))
	c.JSON(http.StatusOK, t)
}

// ── 下载 ──────────────────────────────────────────────────

func DownloadFileAPI(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少 url 参数"})
		return
	}

	// 解析文件路径：/storage/results/xxx.png → 转为相对项目根目录的路径
	filePath := filepath.Join(".", url)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "文件不存在"})
		return
	}

	filename := filepath.Base(filePath)
	ext := filepath.Ext(filename)
	mime := map[string]string{
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".webp": "image/webp",
	}[ext]
	if mime == "" {
		mime = "application/octet-stream"
	}

	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.File(filePath)
}

func DownloadZipAPI(c *gin.Context) {
	urlsParam := c.Query("urls")
	if urlsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少 urls 参数"})
		return
	}

	// 自定义文件名前缀（SKU/平台/序号），默认 result；做基础清洗防止路径穿越
	zipPrefix := strings.TrimSpace(c.Query("prefix"))
	if zipPrefix == "" {
		zipPrefix = "result"
	}
	zipPrefix = strings.NewReplacer("/", "_", "\\", "_", "..", "_", " ", "_").Replace(zipPrefix)

	// 解析逗号分隔的 URL 列表
	urlParts := strings.Split(urlsParam, ",")
	var urls []string
	for _, u := range urlParts {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "至少需要一个文件 URL"})
		return
	}

	// 创建 ZIP
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for i, url := range urls {
		filePath := filepath.Join(".", url)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		f, err := os.Open(filePath)
		if err != nil {
			continue
		}

		entryName := fmt.Sprintf("%s_%d%s", zipPrefix, i+1, filepath.Ext(filePath))
		entry, err := w.Create(entryName)
		if err != nil {
			f.Close()
			continue
		}

		io.Copy(entry, f)
		f.Close()
	}
	w.Close()

	if buf.Len() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "没有找到可下载的文件"})
		return
	}

	filename := fmt.Sprintf("results_%s.zip", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// ── SQL 常量 ────────────────────────────────────────────────

const taskSelectSQL = `
	SELECT task_id, feature, status, image_ids, result_urls, error_message, progress,
	       total_images, processed_images, total_videos, processed_videos,
	       submit_ids, source_filenames, source_paths, folder_name, created_at, completed_at,
	       item_status
	FROM photo_tasks`
