package photo

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	adaptercommon "chat/adapter/common"
	"chat/admin"
	"chat/channel"
	"chat/globals"
	"chat/manager"
	"chat/utils"
)

const MaxWorkers = 4
const MaxGenerateCount = 6

// identityContext 承载一致性身份在处理期的派生数据：参考图 base64、锁定 seed、主体描述。
// 所有方法对 nil 安全，未应用身份时直接传 nil 即可。
type identityContext struct {
	refImages  []string // base64
	lockedSeed *int
	subject    string
}

func (i *identityContext) refB64() []string {
	if i == nil {
		return nil
	}
	return i.refImages
}

func (i *identityContext) seed() *int {
	if i == nil {
		return nil
	}
	return i.lockedSeed
}

// composeSubject 把身份主体描述前置到用户 prompt（主体在前，强化一致性约束）。
func composeSubject(i *identityContext, userPrompt string) string {
	if i == nil || i.subject == "" {
		return userPrompt
	}
	if userPrompt == "" {
		return i.subject
	}
	return i.subject + ", " + userPrompt
}

func ResolveImagePaths(db *sql.DB, imageIDs []string, userID int64) ([]string, error) {
	var paths []string
	for _, id := range imageIDs {
		img, err := queryImageByID(db, id, userID)
		if err != nil {
			return nil, fmt.Errorf("图片 %s 不存在: %w", id, err)
		}
		filename := filepath.Base(img.Url)
		diskPath := filepath.Join(UploadDir(), filename)
		paths = append(paths, diskPath)
	}
	return paths, nil
}

func ReadImageBytesAsBase64(path string) (string, error) {
	b64, err := utils.ConvertToBase64(path)
	if err != nil {
		data, err := utils.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("读取图片失败: %w", err)
		}
		return utils.Base64EncodeBytes([]byte(data)), nil
	}
	return b64, nil
}

type ProcessFunc func(imagePath string) (string, error)

func ParallelProcess(imagePaths []string, fn ProcessFunc, taskID string, db *sql.DB, totalSteps int) []string {
	if len(imagePaths) == 0 {
		return nil
	}
	sem := make(chan struct{}, MaxWorkers)
	results := make([]string, len(imagePaths))
	var wg sync.WaitGroup
	var completed int64
	for i, p := range imagePaths {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := fn(path)
			if err == nil && res != "" {
				results[idx] = res
			}
			cur := int(atomic.AddInt64(&completed, 1))
			if db != nil && taskID != "" && totalSteps > 0 {
				pct := cur * 100 / totalSteps
				if pct > 99 {
					pct = 99
				}
				updateTaskProgress(db, taskID, pct)
			}
		}(i, p)
	}
	wg.Wait()
	var final []string
	for _, r := range results {
		if r != "" {
			final = append(final, r)
		}
	}
	return final
}

func updateTaskProgress(db *sql.DB, taskID string, progress int) {
	if db != nil && taskID != "" {
		db.Exec("UPDATE photo_tasks SET progress = ? WHERE task_id = ?", progress, taskID)
	}
}

// extractResultURL 从 hook 内容中取出裸结果地址。适配器为兼容聊天渲染，会把图片
// 结果包成 Markdown(![image](url))；Photo 模块只需其中的 URL（视频为裸路径，原样返回）。
func extractResultURL(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "![") {
		if l := strings.Index(content, "("); l >= 0 {
			if r := strings.Index(content[l+1:], ")"); r >= 0 {
				return strings.TrimSpace(content[l+1 : l+1+r])
			}
		}
	}
	return content
}

func callImageEdit(imageBase64, prompt, model, userGroup string) (string, error) {
	return callImageEditMulti([]string{imageBase64}, prompt, model, userGroup, nil)
}

// callImageEditMulti 支持多张参考图的图生图编辑（如 Logo 定制：商品图 + Logo 图；
// 一致性身份：商品图 + 身份参考图组）。seed 非空时锁定 seed 以获得更一致的结果。
func callImageEditMulti(images []string, prompt, model, userGroup string, seed *int) (string, error) {
	props := adaptercommon.CreateImageEditProps(&adaptercommon.ImageEditProps{
		Model: model, Images: images, Prompt: prompt, Seed: seed,
	})
	props.OriginalModel = model
	var resultURL string
	err := channel.NewImageEditRequestWithChannel(userGroup, props, func(data *globals.Chunk) error {
		if data != nil && data.Content != "" {
			resultURL = extractResultURL(data.Content)
		}
		return nil
	})
	return resultURL, err
}

// processDetailImage 细节图：用官方即梦生成商品材质/工艺特写（替代旧的本地裁剪）。
func processDetailImage(imagePath, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("detail_image", nil)
	return callImageEdit(b64, prompt, resolveModel("detail_image", channelOverride), userGroup)
}

// processLogoCustom Logo 定制：商品图 + Logo 图作为双参考图，用 AI 自然合成（替代旧的本地叠加）。
func processLogoCustom(productPath, logoPath, position, channelOverride, userGroup string) (string, error) {
	productB64, err := ReadImageBytesAsBase64(productPath)
	if err != nil {
		return "", err
	}
	if logoPath == "" {
		return "", fmt.Errorf("Logo 定制需要先上传并指定 Logo 图片")
	}
	logoB64, err := ReadImageBytesAsBase64(logoPath)
	if err != nil {
		return "", fmt.Errorf("读取 Logo 图片失败: %w", err)
	}
	prompt := GetSystemPrompt("logo_custom", map[string]string{"position": position})
	return callImageEditMulti([]string{productB64, logoB64}, prompt, resolveModel("logo_custom", channelOverride), userGroup, nil)
}

func clampGenerateCount(count int) int {
	if count <= 0 {
		return 1
	}
	if count > MaxGenerateCount {
		return MaxGenerateCount
	}
	return count
}

func supportsGenerateCount(feature string) bool {
	switch feature {
	case FeatureHdUpscale, FeatureResize, FeatureVideoGen, FeatureDetailImage, FeatureLogoCustom,
		FeatureMaterialExtract, FeatureProductExtract:
		return false
	default:
		return true
	}
}

func appendGenerated(resultURLs *[]string, errRef *error, url string, e error) {
	if e != nil {
		*errRef = e
		return
	}
	if url != "" {
		*resultURLs = append(*resultURLs, url)
	}
}

func processWhiteBg(imagePath, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("white_bg", nil)
	return callImageEdit(b64, prompt, resolveModel("white_bg", channelOverride), userGroup)
}

func processSceneGen(imagePath, userPrompt, channelOverride, userGroup string, idt *identityContext) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("scene_gen", map[string]string{"user_prompt": composeSubject(idt, userPrompt)})
	images := append([]string{b64}, idt.refB64()...)
	return callImageEditMulti(images, prompt, resolveModel("scene_gen", channelOverride), userGroup, idt.seed())
}

func processImageErase(imagePath, erasePrompt, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := BuildPrompt("image_erase", "", map[string]string{})
	if erasePrompt != "" {
		prompt = erasePrompt
	}
	return callImageEdit(b64, prompt, resolveModel("image_erase", channelOverride), userGroup)
}

func processColorChange(imagePath, targetColor, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("color_change", map[string]string{"target_color": targetColor})
	return callImageEdit(b64, prompt, resolveModel("color_change", channelOverride), userGroup)
}

func processMarketing(imagePath, sellingPoint, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("marketing", map[string]string{"selling_point": sellingPoint})
	return callImageEdit(b64, prompt, resolveModel("marketing", channelOverride), userGroup)
}

func processImageTranslate(imagePath, targetLang, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("image_translate", map[string]string{"target_lang": targetLang})
	return callImageEdit(b64, prompt, resolveModel("image_translate", channelOverride), userGroup)
}

func processHdUpscale(imagePath, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	model := resolveModel("hd_upscale", channelOverride)
	props := adaptercommon.CreateImageUpscaleProps(&adaptercommon.ImageUpscaleProps{
		Model: model, Image: b64, ResolutionType: "2k",
	})
	props.OriginalModel = model
	var resultURL string
	err = channel.NewImageUpscaleRequestWithChannel(userGroup, props, func(data *globals.Chunk) error {
		if data != nil && data.Content != "" {
			resultURL = extractResultURL(data.Content)
		}
		return nil
	})
	return resultURL, err
}

func processModelImage(imagePath, prompt, channelOverride, userGroup string, idt *identityContext) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	fullPrompt := BuildPrompt("model_image", composeSubject(idt, prompt), map[string]string{})
	images := append([]string{b64}, idt.refB64()...)
	return callImageEditMulti(images, fullPrompt, resolveModel("model_image", channelOverride), userGroup, idt.seed())
}

func processMaterialChange(imagePath, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("material_change", nil)
	return callImageEdit(b64, prompt, resolveModel("material_change", channelOverride), userGroup)
}

func processInstructionGen(imagePath, userPrompt, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("instruction_gen", map[string]string{"user_prompt": userPrompt})
	return callImageEdit(b64, prompt, resolveModel("instruction_gen", channelOverride), userGroup)
}

func processProductionFlow(imagePath, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("production_flow", nil)
	return callImageEdit(b64, prompt, resolveModel("production_flow", channelOverride), userGroup)
}

// 即梦素材/商品提取要求输入边长 1024–4096，小图会被服务端直接拒绝（50500/50207）。
const (
	extractMinSide = 1024
	extractMaxSide = 4096
)

func processMaterialExtract(imagePath, category, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBase64EnsureMinSide(imagePath, extractMinSide, extractMaxSide)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("material_extract", map[string]string{"category": category})
	return callImageEdit(b64, prompt, resolveModel("material_extract", channelOverride), userGroup)
}

func processProductExtract(imagePath, category, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBase64EnsureMinSide(imagePath, extractMinSide, extractMaxSide)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("product_extract", map[string]string{"category": category})
	return callImageEdit(b64, prompt, resolveModel("product_extract", channelOverride), userGroup)
}

func processResizeItem(imagePath, ratio, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	// 改尺寸=智能扩图：jimeng-outpaint 是 Outpaint 能力，必须走扩图路径（按目标比例
	// 自动算四向扩展），不能走 callImageEdit（edit 不支持 outpaint 能力会直接报错）。
	model := resolveModel("resize", channelOverride)
	prompt := GetSystemPrompt("resize", map[string]string{"target_ratio": ratio})
	props := adaptercommon.CreateImageOutpaintProps(&adaptercommon.ImageOutpaintProps{
		Model: model, Image: b64, TargetRatio: ratio, Prompt: prompt,
	})
	props.OriginalModel = model
	var resultURL string
	err = channel.NewImageOutpaintRequestWithChannel(userGroup, props, func(data *globals.Chunk) error {
		if data != nil && data.Content != "" {
			resultURL = extractResultURL(data.Content)
		}
		return nil
	})
	return resultURL, err
}

func processVideoGenSingle(imagePaths []string, prompt string, duration int, channelOverride, userGroup string) (string, error) {
	if len(imagePaths) == 0 {
		return "", fmt.Errorf("视频生成需要至少 1 张参考图")
	}
	if len(imagePaths) > 9 {
		return "", fmt.Errorf("视频生成最多支持 9 张参考图")
	}
	var b64Images []string
	for _, p := range imagePaths {
		b64, err := ReadImageBytesAsBase64(p)
		if err != nil {
			return "", fmt.Errorf("读取图片失败: %w", err)
		}
		b64Images = append(b64Images, b64)
	}
	promptText := ""
	if prompt != "" {
		promptText = GetSystemPrompt("video_gen", map[string]string{"user_prompt": prompt})
	}
	model := resolveModel("video_gen", channelOverride)
	if duration <= 0 {
		duration = 10
	}
	props := adaptercommon.CreateImageToVideoProps(&adaptercommon.ImageToVideoProps{
		Model: model, Images: b64Images, Prompt: promptText, Duration: duration,
	})
	props.OriginalModel = model
	var resultURL string
	err := channel.NewImageToVideoRequestWithChannel(userGroup, props, func(data *globals.Chunk) error {
		if data != nil && data.Content != "" {
			resultURL = extractResultURL(data.Content)
		}
		return nil
	})
	return resultURL, err
}

func resolveModel(feature, channelOverride string) string {
	defaultModel := "jimeng-v2"
	if fc := GetFeatureConfig(feature); fc != nil {
		defaultModel = fc.Model
	}
	// 仅「生图类」功能允许被前端模型下拉覆盖，且只能切换到另一个生图模型；
	// 高清/扩图/提取/视频等能力功能必须使用各自专属模型，否则会被错误地路由到
	// 不支持该能力的模型（例如视频被强制用 seedream → jimeng-api 不支持视频）。
	if channelOverride != "" && isGenerationModel(defaultModel) && isGenerationModel(channelOverride) {
		return channelOverride
	}
	return defaultModel
}

// isGenerationModel 判断是否为生图（文/图生图）基础模型。
func isGenerationModel(model string) bool {
	return strings.HasPrefix(model, "jimeng-seedream")
}

func ProcessTask(ctx context.Context, db *sql.DB, taskID, feature string, imagePaths []string, params map[string]interface{}, channelOverride, userGroup string, identityRefPaths []string, identitySeed *int, identitySubject string) {
	defer func() {
		if r := recover(); r != nil {
			println("[ProcessTask] PANIC:", fmt.Sprintf("%v", r))
			db.Exec("UPDATE photo_tasks SET status = ?, error_message = ? WHERE task_id = ?",
				TaskStatusFailed, fmt.Sprintf("panic: %v", r), taskID)
		}
	}()
	db.Exec("UPDATE photo_tasks SET status = ? WHERE task_id = ?", TaskStatusProcessing, taskID)

	// 一致性身份：读取参考图为 base64（仅一次），供 scene_gen/model_image 注入。
	var idt *identityContext
	if len(identityRefPaths) > 0 || identitySeed != nil || identitySubject != "" {
		var refB64 []string
		for _, p := range identityRefPaths {
			if b, e := ReadImageBytesAsBase64(p); e == nil {
				refB64 = append(refB64, b)
			}
		}
		idt = &identityContext{refImages: refB64, lockedSeed: identitySeed, subject: identitySubject}
	}

	var resultURLs []string
	var err error
	isVideo := feature == FeatureVideoGen
	isLocal := !IsAIFeature(feature)

	if isLocal {
		for _, p := range imagePaths {
			switch feature {
			case FeatureDetailImage:
				url, e := ProcessDetailImage(p)
				if e != nil {
					err = e
					continue
				}
				if url != "" {
					resultURLs = append(resultURLs, url)
				}
			case FeatureLogoCustom:
				logoPath := getStringParam(params, "logo_path", "")
				position := getStringParam(params, "position", "bottom-right")
				url, e := ProcessLogoCustom(p, logoPath, position)
				if e != nil {
					err = e
					continue
				}
				if url != "" {
					resultURLs = append(resultURLs, url)
				}
			}
		}
	} else if isVideo {
		prompt := getStringParam(params, "prompt", "")
		duration := getIntParam(params, "duration", 10)
		url, e := processVideoGenSingle(imagePaths, prompt, duration, channelOverride, userGroup)
		if e != nil {
			err = e
		} else {
			resultURLs = append(resultURLs, url)
		}
	} else {
		for _, p := range imagePaths {
			imageCount := 1
			if supportsGenerateCount(feature) {
				imageCount = clampGenerateCount(getIntParam(params, "image_count", 1))
			}
			switch feature {
			case FeatureWhiteBg:
				for i := 0; i < imageCount; i++ {
					url, e := processWhiteBg(p, channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureSceneGen:
				for i := 0; i < imageCount; i++ {
					url, e := processSceneGen(p, getStringParam(params, "prompt", ""), channelOverride, userGroup, idt)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureImageErase:
				for i := 0; i < imageCount; i++ {
					url, e := processImageErase(p, getStringParam(params, "prompt", ""), channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureColorChange:
				for i := 0; i < imageCount; i++ {
					url, e := processColorChange(p, getStringParam(params, "target_color", "red"), channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureMarketing:
				for i := 0; i < imageCount; i++ {
					url, e := processMarketing(p, getStringParam(params, "selling_point", "Premium Quality"), channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureImageTranslate:
				for i := 0; i < imageCount; i++ {
					url, e := processImageTranslate(p, getStringParam(params, "target_lang", "en"), channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureHdUpscale:
				url, e := processHdUpscale(p, channelOverride, userGroup)
				appendGenerated(&resultURLs, &err, url, e)
			case FeatureModelImage:
				for i := 0; i < imageCount; i++ {
					url, e := processModelImage(p, getStringParam(params, "prompt", ""), channelOverride, userGroup, idt)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureMaterialChange:
				for i := 0; i < imageCount; i++ {
					url, e := processMaterialChange(p, channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureInstructionGen:
				for i := 0; i < imageCount; i++ {
					url, e := processInstructionGen(p, getStringParam(params, "prompt", ""), channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureProductionFlow:
				for i := 0; i < imageCount; i++ {
					url, e := processProductionFlow(p, channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureResize:
				for _, ratio := range getStringSliceParam(params, "target_sizes", []string{"1:1", "16:9", "4:3"}) {
					url, e := processResizeItem(p, ratio, channelOverride, userGroup)
					appendGenerated(&resultURLs, &err, url, e)
				}
			case FeatureMaterialExtract:
				url, e := processMaterialExtract(p, getStringParam(params, "category", "图案"), channelOverride, userGroup)
				appendGenerated(&resultURLs, &err, url, e)
			case FeatureProductExtract:
				url, e := processProductExtract(p, getStringParam(params, "category", "服装"), channelOverride, userGroup)
				appendGenerated(&resultURLs, &err, url, e)
			case FeatureDetailImage:
				url, e := processDetailImage(p, channelOverride, userGroup)
				appendGenerated(&resultURLs, &err, url, e)
			case FeatureLogoCustom:
				url, e := processLogoCustom(p, getStringParam(params, "logo_path", ""), getStringParam(params, "position", "bottom-right"), channelOverride, userGroup)
				appendGenerated(&resultURLs, &err, url, e)
			}
		}
	}

	resultJSON := utils.Marshal(resultURLs)
	now := time.Now().Format(time.RFC3339)
	if err != nil {
		db.Exec("UPDATE photo_tasks SET status = ?, error_message = ?, result_urls = ?, completed_at = ? WHERE task_id = ?",
			TaskStatusFailed, err.Error(), resultJSON, now, taskID)
	} else {
		processedVideos := 0
		processedImages := len(resultURLs)
		if isVideo && len(resultURLs) > 0 {
			processedVideos = 1
		}
		db.Exec(`UPDATE photo_tasks SET status = ?, result_urls = ?, progress = 100,
			processed_images = ?, processed_videos = ?, completed_at = ? WHERE task_id = ?`,
			TaskStatusSuccess, resultJSON, processedImages, processedVideos, now, taskID)
	}

	recordPhotoGeneration(db, taskID, feature, channelOverride, len(resultURLs), err)
}

// recordPhotoGeneration 把 Photo 流水线的一次生成同时计入两处：
//  1. image_generation 观测表（与聊天 / API 入口共用，便于后台「图片用量」统计与排查）。
//  2. 与聊天 / API 一致的分析计数器（请求量/模型使用/错误），使「数据分析」面板能反映图片处理活动。
//
// 之前 Photo 链路两者都未接入，导致 6/24 有 25 条记录但数据分析页请求量仍为 0。
func recordPhotoGeneration(db *sql.DB, taskID, feature, channelOverride string, imageCount int, genErr error) {
	if db == nil {
		return
	}

	model := resolveModel(feature, channelOverride)

	// 分析计数器不依赖用户信息，独立先行，避免用户查询失败时漏统计。
	admin.AnalyseImageRequest(model, imageCount, genErr)

	var userID int64
	var username string
	if err := db.QueryRow("SELECT user_id FROM photo_tasks WHERE task_id = ?", taskID).Scan(&userID); err != nil {
		return
	}
	if userID > 0 {
		_ = db.QueryRow("SELECT username FROM auth WHERE id = ?", userID).Scan(&username)
	}
	manager.RecordImageOutcome(db, userID, username, manager.ImageSourcePhoto, model, 0, "jimeng-api", imageCount, 0, 0, genErr)
}

func handleResult(resultURLs *[]string, url string, err error) {
	if err == nil && url != "" {
		*resultURLs = append(*resultURLs, url)
	}
}

func getStringParam(params map[string]interface{}, key, defaultVal string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return defaultVal
}

func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case string:
			var i int
			fmt.Sscanf(n, "%d", &i)
			return i
		}
	}
	return defaultVal
}

func getStringSliceParam(params map[string]interface{}, key string, defaultVal []string) []string {
	if v, ok := params[key]; ok {
		switch arr := v.(type) {
		case []string:
			return arr
		case []interface{}:
			var result []string
			for _, item := range arr {
				result = append(result, fmt.Sprintf("%v", item))
			}
			return result
		}
	}
	return defaultVal
}
