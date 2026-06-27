package grsai

import (
	"bytes"
	"chat/globals"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/goccy/go-json"
)

const defaultEndpoint = "https://grsaiapi.com"

// Generator 是 grsai 渠道的调用实例，同时实现多个图片处理接口。
type Generator struct {
	instance globals.ChannelConfig
	endpoint string
	apiKey   string
}

func newGenerator(conf globals.ChannelConfig) *Generator {
	endpoint := strings.TrimSpace(conf.GetEndpoint())
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Generator{
		instance: conf,
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   strings.TrimSpace(conf.GetRandomSecret()),
	}
}

func (c *Generator) GetProxy() globals.ProxyConfig {
	return c.instance.GetProxy()
}

func (c *Generator) httpClient() *http.Client {
	client := &http.Client{
		Timeout:   globals.HttpMaxTimeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	config := c.GetProxy()
	if config.ProxyType == globals.HttpProxyType || config.ProxyType == globals.HttpsProxyType {
		if proxyURL, err := url.Parse(config.Proxy); err == nil {
			if config.Username != "" || config.Password != "" {
				proxyURL.User = url.UserPassword(config.Username, config.Password)
			}
			client.Transport = &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}
	return client
}

// doRequest 执行请求并把响应解析到 out。非 2xx 时结合 out.error 返回错误。
func (c *Generator) doRequest(req *http.Request, out *TaskResponse) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("grsai invalid response (http %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("grsai http %d: %s", resp.StatusCode, out.ErrorMessage(strings.TrimSpace(string(raw))))
	}
	return nil
}

// postJSON 发送 JSON POST 请求（提交任务），带 Bearer 认证，解析响应到 out。
func (c *Generator) postJSON(ctx context.Context, path string, body interface{}, out *TaskResponse) error {
	if c.apiKey == "" {
		return fmt.Errorf("grsai requires an API key (secret)")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req, out)
}

// getResult 查询任务结果：GET /v1/api/result?id=<id>（无请求体）。
func (c *Generator) getResult(ctx context.Context, id string, out *TaskResponse) error {
	if c.apiKey == "" {
		return fmt.Errorf("grsai requires an API key (secret)")
	}
	u := c.endpoint + "/v1/api/result?id=" + url.QueryEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.doRequest(req, out)
}

// Submit 提交任务（async），返回带任务 id 的响应。
func (c *Generator) Submit(ctx context.Context, spec ModelSpec, body GenerateRequest) (*TaskResponse, error) {
	body.ReplyType = "async"
	var resp TaskResponse
	if err := c.postJSON(ctx, spec.Path, body, &resp); err != nil {
		return nil, err
	}
	// async 提交通常返回 running + id；少数情况下可能直接返回终态。
	if resp.ID == "" && !resp.IsTerminal() {
		return &resp, fmt.Errorf("grsai submit failed: missing task id")
	}
	if resp.Status == "failed" || resp.Status == "violation" {
		return &resp, fmt.Errorf("grsai submit failed: %s", resp.ErrorMessage("unknown error"))
	}
	return &resp, nil
}

const (
	initialPollInterval = 2 * time.Second
	pollBackoffFactor   = 3 // 每次 ×3/2 退避
)

// PollResult 轮询 /v1/draw/result 直到终态或超时。
func (c *Generator) PollResult(ctx context.Context, id string, maxWait, interval time.Duration) (*TaskResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("grsai poll: empty task id")
	}
	if maxWait <= 0 {
		maxWait = 10 * time.Minute
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	maxInterval := interval
	cur := initialPollInterval
	if cur > maxInterval {
		cur = maxInterval
	}

	start := time.Now()
	deadline := start.Add(maxWait)
	for attempt := 1; ; attempt++ {
		var resp TaskResponse
		if err := c.getResult(ctx, id, &resp); err != nil {
			return nil, err
		}
		globals.Debug(fmt.Sprintf("[grsai] poll #%d id=%s status=%q progress=%d elapsed=%s",
			attempt, id, resp.Status, resp.Progress, time.Since(start).Truncate(time.Second)))

		if resp.IsTerminal() {
			if !resp.IsSucceeded() {
				return &resp, fmt.Errorf("[grsai] task failed (id=%s): %s", id, resp.ErrorMessage("unknown error"))
			}
			globals.Info(fmt.Sprintf("[grsai] task done (id=%s, polls=%d, elapsed=%s)",
				id, attempt, time.Since(start).Truncate(time.Second)))
			return &resp, nil
		}

		if time.Now().Add(cur).After(deadline) {
			return &resp, fmt.Errorf("[grsai] task timeout after %s (id=%s, status=%s)", maxWait, id, resp.Status)
		}
		timer := time.NewTimer(cur)
		select {
		case <-ctx.Done():
			timer.Stop()
			return &resp, ctx.Err()
		case <-timer.C:
		}
		if cur < maxInterval {
			cur = cur * pollBackoffFactor / 2
			if cur > maxInterval {
				cur = maxInterval
			}
		}
	}
}
