package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// videoPollMaxWait 视频生成较慢，给足轮询时长。
const videoPollMaxWait = 20 * time.Minute

// CreateImageToVideoRequest 图生视频。复用官方 jimeng-api 渠道的凭证与
// CVSync2AsyncSubmitTask / CVSync2AsyncGetResult 异步流程，仅 req_key 不同。
func (c *ImageGenerator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	model := props.Model
	if model == "" {
		model = props.OriginalModel
	}
	spec, ok := GetModelSpec(model)
	if !ok {
		return fmt.Errorf("jimeng-api 不支持的视频模型: %s", model)
	}
	if spec.Capability != CapabilityVideo {
		return fmt.Errorf("jimeng-api 模型 %s 不是视频模型", model)
	}
	if len(props.Images) == 0 {
		return fmt.Errorf("视频生成需要至少 1 张参考图")
	}

	images := props.Images
	if spec.MaxImages > 0 && len(images) > spec.MaxImages {
		images = images[:spec.MaxImages]
	}

	req := SubmitTaskRequest{
		ReqKey:           spec.ReqKey,
		BinaryDataBase64: images,
		Prompt:           strings.TrimSpace(props.Prompt),
	}
	if spec.DefaultSeed >= 0 {
		seed := spec.DefaultSeed
		req.Seed = &seed
	}
	// 时长：火山视频接口以帧数控制 (24fps)，5s→121、10s→241。
	if frames := durationToFrames(props.Duration); frames > 0 {
		req.Frames = &frames
	}

	ctx := context.Background()
	submitResp, err := c.Submit(ctx, req)
	if err != nil {
		return err
	}
	if submitResp.Data == nil || submitResp.Data.TaskID == "" {
		return fmt.Errorf(submitResp.ErrorMessage("jimeng 视频任务提交失败"))
	}
	taskID := submitResp.Data.TaskID
	globals.Info(fmt.Sprintf("[jimeng-api] video task submitted (task_id=%s, model=%s, images=%d)",
		taskID, model, len(images)))

	resultResp, err := c.Poll(ctx, spec.ReqKey, taskID, videoPollMaxWait, 10*time.Second)
	if err != nil {
		return err
	}
	if resultResp.Data == nil {
		return fmt.Errorf("jimeng 视频任务无结果 (task_id=%s)", taskID)
	}

	videoURL := extractVideoURL(resultResp.Data)
	if videoURL == "" {
		return fmt.Errorf("jimeng 未返回视频结果 (task_id=%s)", taskID)
	}

	// 下载并落地为本地 .mp4，使下载接口能按 video/mp4 提供、前端可正常预览/下载
	// （否则返回火山临时 URL，下载接口当作本地文件找不到、退化成 octet-stream/txt）。
	stored, err := c.storeVideoURL(videoURL)
	if err != nil {
		globals.Warn(fmt.Sprintf("[jimeng-api] 视频结果落地失败，回退使用源地址 (task_id=%s): %s", taskID, err))
		stored = videoURL
	}

	return hook(&globals.Chunk{Content: stored})
}

// storeVideoURL 下载视频结果并存到 storage/results，返回本地公开路径。
func (c *ImageGenerator) storeVideoURL(videoURL string) (string, error) {
	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		return "", fmt.Errorf("empty video url")
	}
	parsed, err := url.Parse(videoURL)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	switch ext {
	case ".mp4", ".webm", ".mov":
	default:
		ext = ".mp4"
	}

	if err := os.MkdirAll(globals.StorageResultDir, 0755); err != nil {
		return "", err
	}
	filename := resultFilename(videoURL, ext)
	savePath := filepath.Join(globals.StorageResultDir, filename)

	req, err := http.NewRequest(http.MethodGet, videoURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download jimeng video failed: http %d", resp.StatusCode)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(savePath)
		return "", err
	}

	return publicResultURL(filename), nil
}

// durationToFrames 将秒数换算为火山视频接口的帧数 (24fps + 1)。
// 当前 i2v 模型主要支持 5s / 10s；<=0 时返回 0 表示不下发，由模型用默认时长。
func durationToFrames(seconds int) int {
	switch {
	case seconds <= 0:
		return 0
	case seconds <= 5:
		return 121 // 5s
	default:
		return 241 // 10s
	}
}

// extractVideoURL 从结果里取视频地址：优先 video_url，再尝试 image_urls，
// 最后回退解析 resp_data JSON。
func extractVideoURL(data *TaskPayload) string {
	if data == nil {
		return ""
	}
	if v := strings.TrimSpace(data.VideoURL); v != "" {
		return v
	}
	if len(data.ImageURLs) > 0 {
		if v := strings.TrimSpace(data.ImageURLs[0]); v != "" {
			return v
		}
	}
	if data.RespData != "" {
		var parsed struct {
			VideoURL  string   `json:"video_url"`
			URL       string   `json:"url"`
			VideoURLs []string `json:"video_urls"`
		}
		if err := json.Unmarshal([]byte(data.RespData), &parsed); err == nil {
			if v := strings.TrimSpace(parsed.VideoURL); v != "" {
				return v
			}
			if v := strings.TrimSpace(parsed.URL); v != "" {
				return v
			}
			if len(parsed.VideoURLs) > 0 {
				return strings.TrimSpace(parsed.VideoURLs[0])
			}
		}
	}
	return ""
}

var _ adaptercommon.ImageToVideoFactory = (*ImageGenerator)(nil)
