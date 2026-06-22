package dreamina

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"fmt"
	"net/http"
	"path"
	"time"
)

// ── 端点地址 ──────────────────────────────────────────────────

func (c *ImageProcessor) getImage2ImageEndpoint() string {
	return fmt.Sprintf("%s/v1/image2image", c.GetEndpoint())
}

func (c *ImageProcessor) getUpscaleEndpoint() string {
	return fmt.Sprintf("%s/v1/image_upscale", c.GetEndpoint())
}

func (c *ImageProcessor) getQueryResultEndpoint(submitId string) string {
	return fmt.Sprintf("%s/v1/query_result?submit_id=%s", c.GetEndpoint(), submitId)
}

// ── 提交 → 轮询 → 下载 核心流程 ────────────────────────────────

// submitAndPoll 提交异步任务并轮询等待完成，返回结果 URL
func (c *ImageProcessor) submitAndPoll(submitURL string, submitBody interface{}, maxWait time.Duration, progressCallback func(elapsed, maxWait int64)) (string, error) {
	// 1. 提交
	res, err := utils.Post(submitURL, c.GetHeader(), submitBody, c.GetProxy())
	if err != nil || res == nil {
		return "", fmt.Errorf("dreamina submit failed: %v", err)
	}

	submitResp := utils.MapToStruct[ImageSubmitResponse](res)
	if submitResp == nil || submitResp.SubmitId == "" {
		return "", fmt.Errorf("dreamina submit: no submit_id in response")
	}

	submitId := submitResp.SubmitId

	// 2. 轮询等待完成
	pollInterval := 10 * time.Second
	if maxWait == 0 {
		maxWait = 15 * time.Minute
	}

	elapsed := time.Duration(0)
	for elapsed < maxWait {
		time.Sleep(pollInterval)
		elapsed += pollInterval

		queryRes, qErr := utils.Get(c.getQueryResultEndpoint(submitId), c.GetHeader(), c.GetProxy())
		if qErr != nil || queryRes == nil {
			continue
		}

		result := utils.MapToStruct[QueryResultResponse](queryRes)
		if result == nil {
			continue
		}

		switch result.GenStatus {
		case "success":
			// 返回结果 URL
			if result.VideoUrl != "" {
				return result.VideoUrl, nil
			}
			if result.ResultUrl != "" {
				return result.ResultUrl, nil
			}
			return result.Result, nil
		case "fail":
			reason := result.FailReason
			if reason == "" {
				reason = "unknown"
			}
			return "", fmt.Errorf("dreamina generation failed: %s", reason)
		}

		// 仍在生成中，更新进度
		if progressCallback != nil {
			progressCallback(int64(elapsed.Seconds()), int64(maxWait.Seconds()))
		}
	}

	return "", fmt.Errorf("dreamina generation timeout (> %v)", maxWait)
}

// ── 图片编辑 (image2image) ────────────────────────────────────

// CreateImageEditRequest 图生图：1-10 张参考图 + prompt → 生成新图
func (c *ImageProcessor) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	prompt := props.Prompt
	if prompt == "" {
		prompt = "enhance product image, professional studio quality"
	}

	body := ImageSubmitRequest{
		Model:  props.Model,
		Images: props.Images,
		Prompt: prompt,
	}

	resultURL, err := c.submitAndPoll(c.getImage2ImageEndpoint(), body, 10*time.Minute, nil)
	if err != nil {
		return err
	}

	// 下载结果并保存
	storedURL := utils.StoreImage(resultURL)
	if storedURL == "" {
		storedURL = resultURL
	}

	return hook(&globals.Chunk{Content: utils.GetImageMarkdown(storedURL)})
}

// ── 超清放大 ──────────────────────────────────────────────────

// CreateImageUpscaleRequest 图片超清放大 (2k/4k/8k)
func (c *ImageProcessor) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	resolution := props.ResolutionType
	if resolution == "" {
		resolution = "2k"
	}

	body := UpscaleSubmitRequest{
		Model:          props.Model,
		Image:          props.Image,
		ResolutionType: resolution,
	}

	resultURL, err := c.submitAndPoll(c.getUpscaleEndpoint(), body, 5*time.Minute, nil)
	if err != nil {
		return err
	}

	storedURL := utils.StoreImage(resultURL)
	if storedURL == "" {
		storedURL = resultURL
	}

	return hook(&globals.Chunk{Content: utils.GetImageMarkdown(storedURL)})
}

// ── 画布扩展 (outpaint) ───────────────────────────────────────

// CreateImageOutpaintRequest 画布扩展：通过 image2image + expand prompt 实现
func (c *ImageProcessor) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	prompt := props.Prompt
	if prompt == "" {
		prompt = fmt.Sprintf(
			"expand the image canvas to %s aspect ratio, intelligently fill the extended areas to match the original scene seamlessly, keep the original content centered and unchanged, all visible text must be in English only",
			props.TargetRatio,
		)
	}

	body := ImageSubmitRequest{
		Model:  props.Model,
		Images: []string{props.Image},
		Prompt: prompt,
		Ratio:  props.TargetRatio,
	}

	resultURL, err := c.submitAndPoll(c.getImage2ImageEndpoint(), body, 10*time.Minute, nil)
	if err != nil {
		return err
	}

	storedURL := utils.StoreImage(resultURL)
	if storedURL == "" {
		storedURL = resultURL
	}

	return hook(&globals.Chunk{Content: utils.GetImageMarkdown(storedURL)})
}

// ── 下载 ──────────────────────────────────────────────────────

// downloadImage 从 URL 下载图片并保存到 storage/
func (c *ImageProcessor) downloadImage(url string) (string, error) {
	// 使用 HTTP GET 获取图片数据
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	imageName := fmt.Sprintf("dreamina_%d%s", time.Now().UnixNano(), path.Ext(url))
	if path.Ext(url) == "" {
		imageName += ".png"
	}

	filePath := fmt.Sprintf("storage/results/%s", imageName)
	if err := utils.DownloadImage(url, filePath); err != nil {
		return "", err
	}

	return "/storage/results/" + imageName, nil
}
