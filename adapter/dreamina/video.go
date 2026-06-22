package dreamina

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"fmt"
	"net/http"
	"time"
)

// ── 端点 ──────────────────────────────────────────────────────

func (c *ImageProcessor) getVideoCreateEndpoint() string {
	return fmt.Sprintf("%s/v1/multimodal2video", c.GetEndpoint())
}

func (c *ImageProcessor) getVideoQueryEndpoint(id string) string {
	return fmt.Sprintf("%s/v1/videos/%s", c.GetEndpoint(), id)
}

// ── 视频生成 (Seedance 2.0 multimodal2video) ──────────────────

// CreateImageToVideoRequest 图生视频：1-9 张参考图 → AI 推理生成视频
// prompt 可选，留空则由模型自动推理图片内容
func (c *ImageProcessor) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	images := props.Images
	if len(images) == 0 {
		return fmt.Errorf("video generation requires at least 1 reference image")
	}
	if len(images) > 9 {
		return fmt.Errorf("video generation supports max 9 reference images")
	}

	duration := props.Duration
	if duration <= 0 {
		duration = 5
	}
	if duration > 15 {
		duration = 15
	}

	// 1. 提交
	body := VideoSubmitRequest{
		Model:    props.Model,
		Images:   images,
		Prompt:   props.Prompt,
		Duration: duration,
	}

	res, err := utils.Post(c.getVideoCreateEndpoint(), c.GetHeader(), body, c.GetProxy())
	if err != nil || res == nil {
		return fmt.Errorf("dreamina video submit failed: %v", err)
	}

	submitResp := utils.MapToStruct[ImageSubmitResponse](res)
	if submitResp == nil || submitResp.SubmitId == "" {
		return fmt.Errorf("dreamina video submit: no submit_id in response")
	}

	submitId := submitResp.SubmitId

	// 2. 轮询等待完成 (视频最长 15 分钟)
	const maxTimeout = 15 * time.Minute
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	deadline := time.After(maxTimeout)
	beginProgress := false
	lastProgress := -1

	for {
		select {
		case <-ticker.C:
			queryRes, qErr := utils.Get(
				fmt.Sprintf("%s/v1/query_result?submit_id=%s", c.GetEndpoint(), submitId),
				c.GetHeader(), c.GetProxy(),
			)
			if qErr != nil || queryRes == nil {
				continue
			}

			result := utils.MapToStruct[QueryResultResponse](queryRes)
			if result == nil {
				continue
			}

			switch result.GenStatus {
			case "success":
				// 完成，输出结果 URL
				if beginProgress {
					hook(&globals.Chunk{Content: "\n"})
				}

				videoURL := result.VideoUrl
				if videoURL == "" {
					videoURL = result.ResultUrl
				}

				// 下载视频并存储
				storedPath := fmt.Sprintf("storage/results/video_%s_%s.mp4",
					submitId[:8], time.Now().Format("20060102_150405"))

				if videoURL != "" {
					// 尝试下载远程视频
					resp, err := http.Get(videoURL)
					if err == nil && resp.StatusCode == http.StatusOK {
						defer resp.Body.Close()
						if err := utils.DownloadImage(videoURL, storedPath); err == nil {
							videoURL = "/" + storedPath
						}
					}
				}

				return hook(&globals.Chunk{
					Content: utils.Marshal(VideoJobResponse{
						Id:        submitId,
						Status:    "completed",
						CreatedAt: time.Now().Unix(),
					}),
				})

			case "fail":
				if beginProgress {
					hook(&globals.Chunk{Content: "\n"})
				}
				reason := result.FailReason
				if reason == "" {
					reason = "unknown error"
				}
				return fmt.Errorf("dreamina video generation failed: %s", reason)
			}

			// 首次开始进度报告
			if !beginProgress {
				beginProgress = true
				hook(&globals.Chunk{Content: "Video generating..."})
			}

			// 简单进度脉冲
			if lastProgress < 0 {
				lastProgress = 0
				hook(&globals.Chunk{Content: "."})
			}

		case <-deadline:
			if beginProgress {
				hook(&globals.Chunk{Content: "\n"})
			}
			return fmt.Errorf("dreamina video generation timeout (> %v)", maxTimeout)
		}
	}
}
