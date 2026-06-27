package grsai

import (
	"bufio"
	"bytes"
	"chat/globals"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goccy/go-json"
)

// streamB 提交并消费 SurfaceB（gpt-image-2 / veo3-*）任务。
// 这套接口提交后以 SSE（data: 帧）持续推送进度，最终帧带 status=succeeded 与 results[].url，
// 结果直接来自流本身（/v1/draw/result 轮询不可靠），故全程消费流直到终态帧。
// 总时长由 ctx + maxWait 控制；客户端不设整体 Timeout，避免长任务（视频）连接被切断。
func (c *Generator) streamB(ctx context.Context, spec ModelSpec, body GenerateRequest, maxWait time.Duration) (*TaskResponseB, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("grsai requires an API key (secret)")
	}
	if maxWait <= 0 {
		maxWait = 10 * time.Minute
	}
	body.ReplyType = "async"
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+spec.Path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := c.httpClient()
	client.Timeout = 0 // 流式：由 ctx 控制总时长

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("grsai submit http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var last *TaskResponseB
	start := time.Now()
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// 兼容 "data: {...}" 的 SSE 帧与直接的 "{...}"。
		jsonPart := line
		if strings.HasPrefix(jsonPart, "data:") {
			jsonPart = strings.TrimSpace(strings.TrimPrefix(jsonPart, "data:"))
		}
		if !strings.HasPrefix(jsonPart, "{") {
			continue
		}
		var ev TaskResponseB
		if err := json.Unmarshal([]byte(jsonPart), &ev); err != nil {
			continue
		}
		last = &ev
		globals.Debug(fmt.Sprintf("[grsai] stream id=%s status=%q progress=%d elapsed=%s",
			ev.ID, ev.Status, ev.Progress, time.Since(start).Truncate(time.Second)))
		if ev.IsTerminal() {
			if !ev.IsSucceeded() {
				return &ev, fmt.Errorf("[grsai] task failed (id=%s): %s", ev.ID, ev.ErrorMessage("unknown error"))
			}
			globals.Info(fmt.Sprintf("[grsai] task done (id=%s, elapsed=%s)", ev.ID, time.Since(start).Truncate(time.Second)))
			return &ev, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return last, fmt.Errorf("grsai stream error: %w", err)
	}
	if last != nil && last.IsSucceeded() {
		return last, nil
	}
	return last, fmt.Errorf("grsai stream ended without success result")
}

// runTask 按模型所属接口面提交并取回结果地址列表。
func (c *Generator) runTask(ctx context.Context, spec ModelSpec, body GenerateRequest, maxWait, interval time.Duration) ([]string, error) {
	if spec.Surface == SurfaceB {
		res, err := c.streamB(ctx, spec, body, maxWait)
		if err != nil {
			return nil, err
		}
		return res.URLs(), nil
	}

	// SurfaceA：提交 + GET /v1/api/result 轮询。
	submit, err := c.Submit(ctx, spec, body)
	if err != nil {
		return nil, err
	}
	res := submit
	if !(submit.IsSucceeded() && len(submit.Results) > 0) {
		res, err = c.PollResult(ctx, submit.ID, maxWait, interval)
		if err != nil {
			return nil, err
		}
	}
	urls := make([]string, 0, len(res.Results))
	for _, it := range res.Results {
		if u := strings.TrimSpace(it.URL); u != "" {
			urls = append(urls, u)
		}
	}
	return urls, nil
}
