package ark

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	videoPollMaxWait  = 20 * time.Minute
	videoPollInterval = 10 * time.Second
)

// 方舟视频任务状态。
const (
	statusSucceeded = "succeeded"
	statusFailed    = "failed"
	statusCancelled = "cancelled"
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

// imageRole 方舟视频接口要求图片内容必须带 role：
//   - 单图：first_frame（首帧驱动，1.0 Pro / 2.0 均支持）
//   - 多图：reference_image（多参考图，仅 2.0 等支持 r2v 的模型）
func imageRole(count int) string {
	if count <= 1 {
		return "first_frame"
	}
	return "reference_image"
}

type videoImageURL struct {
	URL string `json:"url"`
}

type videoContentItem struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *videoImageURL `json:"image_url,omitempty"`
	Role     string         `json:"role,omitempty"`
}

type videoCreateRequest struct {
	Model   string             `json:"model"`
	Content []videoContentItem `json:"content"`
}

type arkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type videoCreateResponse struct {
	ID    string    `json:"id"`
	Error *arkError `json:"error"`
}

type videoTaskResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Error *arkError `json:"error"`
}

// CreateImageToVideoRequest 图/文生视频：提交异步任务并轮询，成功后落地 mp4 回推。
func (g *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	imgs := normalizeImages(props.Images)
	role := imageRole(len(imgs))

	content := make([]videoContentItem, 0, len(imgs)+1)
	if text := buildVideoPrompt(props.Prompt, props.Duration); text != "" {
		content = append(content, videoContentItem{Type: "text", Text: text})
	}
	for _, u := range imgs {
		content = append(content, videoContentItem{
			Type:     "image_url",
			ImageURL: &videoImageURL{URL: u},
			Role:     role,
		})
	}
	if len(content) == 0 {
		return fmt.Errorf("视频生成需要提示词或参考图")
	}

	res, err := utils.Post(g.endpoint+"/contents/generations/tasks", g.header(),
		videoCreateRequest{Model: props.Model, Content: content}, g.GetProxy())
	if err != nil || res == nil {
		if err != nil {
			return fmt.Errorf("ark 视频任务提交失败: %s", err.Error())
		}
		return fmt.Errorf("ark 视频任务提交失败: 空响应")
	}
	created := utils.MapToStruct[videoCreateResponse](res)
	if created == nil {
		return fmt.Errorf("ark 视频任务提交失败: 无法解析响应")
	}
	if created.Error != nil && strings.TrimSpace(created.Error.Message) != "" {
		return fmt.Errorf("ark 视频任务提交失败: %s", created.Error.Message)
	}
	taskID := strings.TrimSpace(created.ID)
	if taskID == "" {
		return fmt.Errorf("ark 视频任务未返回 task id")
	}
	globals.Info(fmt.Sprintf("[ark] video task submitted (task_id=%s, model=%s, images=%d, role=%s)",
		taskID, props.Model, len(imgs), role))

	deadline := time.Now().Add(videoPollMaxWait)
	for {
		task, err := g.getVideoTask(taskID)
		if err != nil {
			return fmt.Errorf("ark 视频任务查询失败: %s", err.Error())
		}

		switch task.Status {
		case statusSucceeded:
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
		case statusFailed, statusCancelled:
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

// getVideoTask 查询单个视频任务状态。
func (g *Generator) getVideoTask(taskID string) (*videoTaskResponse, error) {
	req, err := http.NewRequest(http.MethodGet, g.endpoint+"/contents/generations/tasks/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range g.header() {
		req.Header.Set(k, v)
	}
	resp, err := g.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var task videoTaskResponse
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("解析任务响应失败: %s", err.Error())
	}
	if task.Error != nil && strings.TrimSpace(task.Error.Message) != "" && task.Status == "" {
		return nil, fmt.Errorf("%s", task.Error.Message)
	}
	return &task, nil
}
