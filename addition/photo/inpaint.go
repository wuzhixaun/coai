package photo

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"chat/admin"
	"chat/auth"
	"chat/channel"
	"chat/manager"
	"chat/utils"

	"github.com/gin-gonic/gin"
)

// ── 画布内局部重绘（P6.3）─────────────────────────────────────
//
// 源图 + 灰度 mask（白=重绘区，黑=保留）+ prompt → jimeng-inpaint（CapabilityInpaint）。
// 复用 edit.go 的 inpaint 分支（要求恰好 2 张图：源图 + mask）。

const InpaintModel = "jimeng-inpaint"

type inpaintRequest struct {
	ImageUrl   string `json:"image_url" binding:"required"`   // 本地 /storage/... 路径（上传图或结果图）
	MaskBase64 string `json:"mask_base64" binding:"required"` // mask PNG base64（可带 data: 前缀）
	Prompt     string `json:"prompt"`
}

func InpaintAPI(c *gin.Context) {
	var req inpaintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请求参数错误"})
		return
	}

	// 仅允许本站 storage 下的图片，防路径穿越
	clean := filepath.Clean(req.ImageUrl)
	if !strings.HasPrefix(clean, "/storage/") || strings.Contains(clean, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "无效的图片地址"})
		return
	}
	sourcePath := filepath.Join(".", clean)

	db := getDBFromCtx(c)
	userID := getUserID(c)
	user := manager.ParseAuth(c, c.GetHeader("Authorization"))
	group := auth.GetGroup(db, user)

	// 额度预检
	if ch := channel.ChargeInstance.GetCharge(InpaintModel); ch != nil && ch.IsBilling() {
		if u := auth.GetUserById(db, userID); u != nil && u.GetQuota(db) < ch.GetLimit() {
			c.JSON(http.StatusPaymentRequired, gin.H{"status": "error", "message": "额度不足，请充值后重试"})
			return
		}
	}

	mask := req.MaskBase64
	if i := strings.Index(mask, "base64,"); i >= 0 {
		mask = mask[i+len("base64,"):]
	}

	taskID := generateImageID()
	_, e := db.Exec(`INSERT INTO photo_tasks
		(task_id, user_id, feature, status, image_ids, params, total_images, total_videos, source_filenames, source_paths, folder_name)
		VALUES (?, ?, ?, 'pending', ?, ?, 1, 0, ?, ?, '')`,
		taskID, userID, FeatureImageErase,
		utils.Marshal([]string{}), utils.Marshal(map[string]interface{}{"prompt": req.Prompt}),
		utils.Marshal([]string{filepath.Base(sourcePath)}), utils.Marshal([]string{sourcePath}),
	)
	if e != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建任务失败"})
		return
	}

	go ProcessInpaintTask(db, taskID, sourcePath, mask, req.Prompt, group)

	c.JSON(http.StatusOK, TaskInfo{
		TaskId: taskID, Feature: FeatureImageErase, Status: TaskStatusPending,
		TotalImages: 1, SourceFilenames: []string{filepath.Base(sourcePath)},
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

func ProcessInpaintTask(db *sql.DB, taskID, sourcePath, maskBase64, prompt, userGroup string) {
	defer func() {
		if r := recover(); r != nil {
			db.Exec("UPDATE photo_tasks SET status = ?, error_message = ? WHERE task_id = ?",
				TaskStatusFailed, fmt.Sprintf("panic: %v", r), taskID)
		}
	}()
	db.Exec("UPDATE photo_tasks SET status = ? WHERE task_id = ?", TaskStatusProcessing, taskID)

	srcB64, err := ReadImageBytesAsBase64(sourcePath)
	var url string
	if err == nil {
		p := strings.TrimSpace(prompt)
		if p == "" {
			p = "删除" // 默认执行消除
		}
		url, err = callImageEditMulti([]string{srcB64, maskBase64}, p, InpaintModel, userGroup, nil)
	}

	now := time.Now().Format(time.RFC3339)
	if err != nil || url == "" {
		msg := "局部重绘失败"
		if err != nil {
			msg = err.Error()
		}
		db.Exec("UPDATE photo_tasks SET status = ?, error_message = ?, completed_at = ? WHERE task_id = ?",
			TaskStatusFailed, msg, now, taskID)
		recordInpaint(db, taskID, 0, fmt.Errorf("%s", msg))
		return
	}
	db.Exec(`UPDATE photo_tasks SET status = ?, result_urls = ?, progress = 100,
		processed_images = 1, completed_at = ? WHERE task_id = ?`,
		TaskStatusSuccess, utils.Marshal([]string{url}), now, taskID)
	recordInpaint(db, taskID, 1, nil)
}

// recordInpaint 按 inpaint 模型计费并记录（与 recordPhotoGeneration 同构，但模型固定为 inpaint）。
func recordInpaint(db *sql.DB, taskID string, count int, genErr error) {
	admin.AnalyseImageRequest(InpaintModel, count, genErr)
	var userID int64
	var username string
	if err := db.QueryRow("SELECT user_id FROM photo_tasks WHERE task_id = ?", taskID).Scan(&userID); err != nil {
		return
	}
	if userID > 0 {
		_ = db.QueryRow("SELECT username FROM auth WHERE id = ?", userID).Scan(&username)
	}
	var quota float32
	if genErr == nil && count > 0 {
		if ch := channel.ChargeInstance.GetCharge(InpaintModel); ch != nil && ch.IsBilling() {
			quota = ch.GetLimit() * float32(count)
			if u := auth.GetUserById(db, userID); u != nil {
				u.UseQuota(db, quota)
			}
		}
	}
	manager.RecordImageOutcome(db, userID, username, manager.ImageSourcePhoto, InpaintModel, 0, "jimeng-api", count, quota, 0, genErr)
}
