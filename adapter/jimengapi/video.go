package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"context"
	"encoding/json"
	"fmt"
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

	return hook(&globals.Chunk{Content: videoURL})
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
