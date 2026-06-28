package ark

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

const (
	videoPollMaxWait  = 20 * time.Minute
	videoPollInterval = 10 * time.Second
)

// buildVideoPrompt 把时长等参数以方舟文本指令形式拼到提示词尾部（如 "... --duration 5"）。
// 方舟视频接口（Seedance）的时长/比例等通过 text 内容里的 --flag 传递。
func buildVideoPrompt(prompt string, duration int) string {
	p := strings.TrimSpace(prompt)
	if duration > 0 {
		p = strings.TrimSpace(fmt.Sprintf("%s --duration %d", p, duration))
	}
	return p
}

// CreateImageToVideoRequest 图/文生视频：提交异步任务并轮询，成功后落地 mp4 回推。
func (g *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	client := arkruntime.NewClientWithApiKey(g.apiKey, arkruntime.WithBaseUrl(g.endpoint))
	ctx := context.Background()

	content := make([]*model.CreateContentGenerationContentItem, 0, len(props.Images)+1)
	if text := buildVideoPrompt(props.Prompt, props.Duration); text != "" {
		t := text
		content = append(content, &model.CreateContentGenerationContentItem{
			Type: model.ContentGenerationContentItemTypeText,
			Text: &t,
		})
	}
	for _, img := range normalizeImages(props.Images) {
		u := img
		content = append(content, &model.CreateContentGenerationContentItem{
			Type:     model.ContentGenerationContentItemTypeImage,
			ImageURL: &model.ImageURL{URL: u},
		})
	}
	if len(content) == 0 {
		return fmt.Errorf("视频生成需要提示词或参考图")
	}

	createResp, err := client.CreateContentGenerationTask(ctx, model.CreateContentGenerationTaskRequest{
		Model:   props.Model,
		Content: content,
	})
	if err != nil {
		return fmt.Errorf("ark 视频任务提交失败: %s", err.Error())
	}
	taskID := strings.TrimSpace(createResp.ID)
	if taskID == "" {
		return fmt.Errorf("ark 视频任务未返回 task id")
	}
	globals.Info(fmt.Sprintf("[ark] video task submitted (task_id=%s, model=%s, images=%d)",
		taskID, props.Model, len(normalizeImages(props.Images))))

	deadline := time.Now().Add(videoPollMaxWait)
	for {
		task, err := client.GetContentGenerationTask(ctx, model.GetContentGenerationTaskRequest{ID: taskID})
		if err != nil {
			return fmt.Errorf("ark 视频任务查询失败: %s", err.Error())
		}

		switch task.Status {
		case model.StatusSucceeded:
			videoURL := strings.TrimSpace(task.Content.VideoURL)
			if videoURL == "" {
				return fmt.Errorf("ark 视频任务成功但未返回视频地址 (task_id=%s)", taskID)
			}
			stored, err := g.storeVideoURL(videoURL)
			if err != nil {
				globals.Warn(fmt.Sprintf("[ark] 视频结果落地失败，回退使用源地址 (task_id=%s): %s", taskID, err.Error()))
				stored = videoURL
			}
			return hook(&globals.Chunk{Content: stored})
		case model.StatusFailed, model.StatusCancelled:
			msg := "未知原因"
			if task.Error != nil && strings.TrimSpace(task.Error.Message) != "" {
				msg = task.Error.Message
			}
			return fmt.Errorf("ark 视频任务失败 (task_id=%s): %s", taskID, msg)
		default: // queued / running
			if time.Now().After(deadline) {
				return fmt.Errorf("ark 视频任务超时 (task_id=%s)", taskID)
			}
			time.Sleep(videoPollInterval)
		}
	}
}
