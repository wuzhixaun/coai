package photo

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"chat/auth"
	"chat/manager"
	"chat/utils"

	"github.com/gin-gonic/gin"
)

// ── 一键成套素材工作流 ────────────────────────────────────────
//
// workflow = 有序的 feature 步骤；步骤间产物传递：上一步的结果图（本地 /storage/results）
// 作为下一步的输入。所有步骤产物聚合到同一个 photo_task（feature=workflow），
// 一次点击产出整套素材，可整套 ZIP 下载。

const FeatureWorkflow = "workflow"

// WorkflowStep 工作流中的一步
type WorkflowStep struct {
	Feature string                 `json:"feature"`
	Params  map[string]interface{} `json:"params"`
}

// WorkflowTemplate 预置成套模板
type WorkflowTemplate struct {
	Key   string         `json:"key"`
	Name  string         `json:"name"`
	Steps []WorkflowStep `json:"steps"`
}

// builtinTemplates 预置模板（P2.2）。步骤均为图→图链路，可被身份/品牌注入。
var builtinTemplates = []WorkflowTemplate{
	{
		Key:  "apparel_listing",
		Name: "服装上架套件",
		Steps: []WorkflowStep{
			{Feature: FeatureWhiteBg},
			{Feature: FeatureSceneGen, Params: map[string]interface{}{"prompt": "简约商业棚拍场景，柔和光线"}},
			{Feature: FeatureMarketing, Params: map[string]interface{}{"selling_point": "新品上市"}},
		},
	},
	{
		Key:  "product_main",
		Name: "商品主图套件",
		Steps: []WorkflowStep{
			{Feature: FeatureWhiteBg},
			{Feature: FeatureSceneGen, Params: map[string]interface{}{"prompt": "干净的商品展示背景"}},
		},
	},
}

func findTemplate(key string) (WorkflowTemplate, bool) {
	for _, t := range builtinTemplates {
		if t.Key == key {
			return t, true
		}
	}
	return WorkflowTemplate{}, false
}

// WorkflowRequest 提交工作流请求
type WorkflowRequest struct {
	Template   string         `json:"template"`     // 预置模板 key（与 steps 二选一）
	Steps      []WorkflowStep `json:"steps"`        // 自定义步骤（配方）
	ImageIds   []string       `json:"image_ids" binding:"required,min=1"`
	IdentityId string         `json:"identity_id"`
	BrandKitId string         `json:"brand_kit_id"`
	ChannelOverride string    `json:"channel_override"`
}

func ListWorkflowTemplatesAPI(c *gin.Context) {
	c.JSON(http.StatusOK, builtinTemplates)
}

func WorkflowAPI(c *gin.Context) {
	var req WorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请求参数错误"})
		return
	}

	// 解析步骤：优先自定义 steps，否则取预置模板
	steps := req.Steps
	if len(steps) == 0 {
		tpl, ok := findTemplate(req.Template)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "未知的工作流模板"})
			return
		}
		steps = tpl.Steps
	}
	if len(steps) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "工作流步骤为空"})
		return
	}

	db := getDBFromCtx(c)
	userID := getUserID(c)
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
		if img, _ := queryImageByID(db, id, userID); img != nil {
			filenames = append(filenames, img.Filename)
			if folderName == "" && img.FolderName != "" {
				folderName = img.FolderName
			}
		}
	}

	// 额度预检：工作流逐步出图，估算 Σ(各步单价 × 图片数)，余额不足则拒绝。
	var estimate float32
	for _, step := range steps {
		estimate += PhotoUnitPrice(step.Feature, req.ChannelOverride) * float32(len(req.ImageIds))
	}
	if estimate > 0 {
		if u := auth.GetUserById(db, userID); u != nil && u.GetQuota(db) < estimate {
			c.JSON(http.StatusPaymentRequired, gin.H{"status": "error", "message": "额度不足，请充值后重试"})
			return
		}
	}

	// 一致性身份 + 品牌资产解析（复用与 ProcessAPI 一致的注入参数）
	identityRefPaths, identitySeed, identitySubject := resolveInjection(db, userID, req.IdentityId, req.BrandKitId)

	taskID := generateImageID()
	_, e := db.Exec(`INSERT INTO photo_tasks
		(task_id, user_id, feature, status, image_ids, params, total_images, total_videos,
		 source_filenames, source_paths, folder_name)
		VALUES (?, ?, ?, 'pending', ?, ?, ?, 0, ?, ?, ?)`,
		taskID, userID, FeatureWorkflow,
		utils.Marshal(req.ImageIds), utils.Marshal(steps), len(req.ImageIds),
		utils.Marshal(filenames), utils.Marshal(imagePaths), folderName,
	)
	if e != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建任务失败"})
		return
	}

	go ProcessWorkflowTask(db, taskID, imagePaths, steps, req.ChannelOverride, group, identityRefPaths, identitySeed, identitySubject)

	c.JSON(http.StatusOK, TaskInfo{
		TaskId: taskID, Feature: FeatureWorkflow, Status: TaskStatusPending,
		ImageIds: req.ImageIds, TotalImages: len(req.ImageIds),
		SourceFilenames: filenames, CreatedAt: time.Now().Format(time.RFC3339), FolderName: folderName,
	})
}

// resolveInjection 解析一致性身份 + 品牌资产为注入参数（参考图路径 / seed / 主体描述）。
// 与 ProcessAPI 的逻辑保持一致，供工作流复用。
func resolveInjection(db *sql.DB, userID int64, identityID, brandKitID string) ([]string, *int, string) {
	var refPaths []string
	var seed *int
	subject := ""
	if identityID != "" {
		if idt, e := queryIdentityByID(db, identityID, userID); e == nil && idt != nil {
			if paths, e2 := ResolveImagePaths(db, idt.RefImageIds, userID); e2 == nil {
				refPaths = paths
			}
			s := idt.Seed
			seed = &s
			subject = idt.SubjectPrompt
		}
	}
	if brandKitID != "" {
		if bk, e := queryIdentityByID(db, brandKitID, userID); e == nil && bk != nil && bk.Type == IdentityTypeBrandKit {
			if paths, e2 := ResolveImagePaths(db, bk.RefImageIds, userID); e2 == nil {
				refPaths = append(refPaths, paths...)
			}
			phrase := "保持品牌一致：叠加品牌 Logo"
			if bk.Color != "" {
				phrase += "，主色调 " + bk.Color
			}
			if subject == "" {
				subject = phrase
			} else {
				subject += "；" + phrase
			}
		}
	}
	return refPaths, seed, subject
}

// ProcessWorkflowTask 顺序执行工作流：上一步结果作下一步输入，聚合所有产物到任务。
func ProcessWorkflowTask(db *sql.DB, taskID string, inputPaths []string, steps []WorkflowStep, channelOverride, userGroup string, identityRefPaths []string, identitySeed *int, identitySubject string) {
	defer func() {
		if r := recover(); r != nil {
			println("[ProcessWorkflowTask] PANIC:", fmt.Sprintf("%v", r))
			db.Exec("UPDATE photo_tasks SET status = ?, error_message = ? WHERE task_id = ?",
				TaskStatusFailed, fmt.Sprintf("panic: %v", r), taskID)
		}
	}()
	db.Exec("UPDATE photo_tasks SET status = ? WHERE task_id = ?", TaskStatusProcessing, taskID)

	idt := buildIdentityContext(identityRefPaths, identitySeed, identitySubject)

	var allResults []string
	var firstErr error
	current := inputPaths

	for i, step := range steps {
		results, err := runFeature(step.Feature, current, step.Params, channelOverride, userGroup, idt)
		allResults = append(allResults, results...)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		// 下一步输入 = 本步结果的本地路径（/storage/results/xxx → ResultDir/xxx）
		if len(results) > 0 {
			next := make([]string, 0, len(results))
			for _, u := range results {
				next = append(next, filepath.Join(ResultDir(), filepath.Base(u)))
			}
			current = next
		}
		pct := (i + 1) * 100 / len(steps)
		if pct > 99 {
			pct = 99
		}
		db.Exec("UPDATE photo_tasks SET progress = ?, processed_images = ? WHERE task_id = ?", pct, len(allResults), taskID)
	}

	resultJSON := utils.Marshal(allResults)
	now := time.Now().Format(time.RFC3339)
	if firstErr != nil {
		// 任一步出错即标记失败，但保留已产出的部分结果（前端按"部分成功"展示）
		db.Exec("UPDATE photo_tasks SET status = ?, error_message = ?, result_urls = ?, processed_images = ?, completed_at = ? WHERE task_id = ?",
			TaskStatusFailed, firstErr.Error(), resultJSON, len(allResults), now, taskID)
	} else {
		db.Exec(`UPDATE photo_tasks SET status = ?, result_urls = ?, progress = 100,
			processed_images = ?, completed_at = ? WHERE task_id = ?`,
			TaskStatusSuccess, resultJSON, len(allResults), now, taskID)
	}

	recordPhotoGeneration(db, taskID, FeatureWorkflow, channelOverride, len(allResults), firstErr)
}
