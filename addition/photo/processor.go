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
	"chat/channel"
	"chat/globals"
	"chat/utils"
)

const MaxWorkers = 4
const MaxGenerateCount = 6

func ResolveImagePaths(db *sql.DB, imageIDs []string, userID int64) ([]string, error) {
	var paths []string
	for _, id := range imageIDs {
		img, err := queryImageByID(db, id, userID)
		if err != nil {
			return nil, fmt.Errorf("图片 %s 不存在: %w", id, err)
		}
		filename := filepath.Base(img.Url)
		diskPath := filepath.Join(UploadDir, filename)
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

func callImageEdit(imageBase64, prompt, model, userGroup string) (string, error) {
	props := adaptercommon.CreateImageEditProps(&adaptercommon.ImageEditProps{
		Model: model, Images: []string{imageBase64}, Prompt: prompt,
	})
	props.OriginalModel = model
	var resultURL string
	err := channel.NewImageEditRequestWithChannel(userGroup, props, func(data *globals.Chunk) error {
		if data != nil && data.Content != "" {
			resultURL = data.Content
		}
		return nil
	})
	return resultURL, err
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

func processSceneGen(imagePath, userPrompt, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("scene_gen", map[string]string{"user_prompt": userPrompt})
	return callImageEdit(b64, prompt, resolveModel("scene_gen", channelOverride), userGroup)
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
			resultURL = data.Content
		}
		return nil
	})
	return resultURL, err
}

func processModelImage(imagePath, prompt, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	fullPrompt := BuildPrompt("model_image", prompt, map[string]string{})
	return callImageEdit(b64, fullPrompt, resolveModel("model_image", channelOverride), userGroup)
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

func processMaterialExtract(imagePath, category, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
	if err != nil {
		return "", err
	}
	prompt := GetSystemPrompt("material_extract", map[string]string{"category": category})
	return callImageEdit(b64, prompt, resolveModel("material_extract", channelOverride), userGroup)
}

func processProductExtract(imagePath, category, channelOverride, userGroup string) (string, error) {
	b64, err := ReadImageBytesAsBase64(imagePath)
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
	prompt := GetSystemPrompt("resize", map[string]string{"target_ratio": ratio})
	return callImageEdit(b64, prompt, resolveModel("resize", channelOverride), userGroup)
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
			resultURL = data.Content
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

func ProcessTask(ctx context.Context, db *sql.DB, taskID, feature string, imagePaths []string, params map[string]interface{}, channelOverride, userGroup string) {
	defer func() {
		if r := recover(); r != nil {
			println("[ProcessTask] PANIC:", fmt.Sprintf("%v", r))
			db.Exec("UPDATE photo_tasks SET status = ?, error_message = ? WHERE task_id = ?",
				TaskStatusFailed, fmt.Sprintf("panic: %v", r), taskID)
		}
	}()
	db.Exec("UPDATE photo_tasks SET status = ? WHERE task_id = ?", TaskStatusProcessing, taskID)

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
					url, e := processSceneGen(p, getStringParam(params, "prompt", ""), channelOverride, userGroup)
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
					url, e := processModelImage(p, getStringParam(params, "prompt", ""), channelOverride, userGroup)
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
