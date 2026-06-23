package jimengapi

import (
	"bytes"
	"chat/globals"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"golang.org/x/net/proxy"
)

const algorithm = "HMAC-SHA256"

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func canonicalEscape(s string) string {
	escaped := url.QueryEscape(s)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func canonicalQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0)
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		for _, val := range vals {
			pairs = append(pairs, canonicalEscape(key)+"="+canonicalEscape(val))
		}
	}
	return strings.Join(pairs, "&")
}

func normalizeSignedHeaders(headers map[string]string) ([]string, map[string]string) {
	lower := make(map[string]string, len(headers))
	names := make([]string, 0, len(headers))
	for key, value := range headers {
		name := strings.ToLower(strings.TrimSpace(key))
		if name == "" {
			continue
		}
		if _, ok := lower[name]; !ok {
			names = append(names, name)
		}
		lower[name] = strings.TrimSpace(value)
	}
	sort.Strings(names)
	return names, lower
}

func canonicalHeaders(headers map[string]string) (string, string) {
	names, lower := normalizeSignedHeaders(headers)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, name+":"+lower[name]+"\n")
	}
	return strings.Join(lines, ""), strings.Join(names, ";")
}

func credentialScopeFor(t time.Time, targetRegion string, targetService string) string {
	return fmt.Sprintf("%s/%s/%s/request", t.UTC().Format("20060102"), targetRegion, targetService)
}

func signingKeyFor(secret string, t time.Time, targetRegion string, targetService string) []byte {
	kDate := hmacSHA256([]byte(secret), t.UTC().Format("20060102"))
	kRegion := hmacSHA256(kDate, targetRegion)
	kService := hmacSHA256(kRegion, targetService)
	return hmacSHA256(kService, "request")
}

func signHeaders(method, uri string, query url.Values, headers map[string]string, body []byte, accessKey, secretKey string, t time.Time) map[string]string {
	return signHeadersWithScope(method, uri, query, headers, body, accessKey, secretKey, t, region, service)
}

func signHeadersWithScope(method, uri string, query url.Values, headers map[string]string, body []byte, accessKey, secretKey string, t time.Time, targetRegion string, targetService string) map[string]string {
	if uri == "" {
		uri = "/"
	}

	xDate := t.UTC().Format("20060102T150405Z")
	payloadHash := sha256Hex(body)

	signed := make(map[string]string, len(headers)+3)
	for key, value := range headers {
		signed[key] = value
	}
	signed["X-Date"] = xDate

	canonicalHeaderText, signedHeaderText := canonicalHeaders(signed)
	canonicalRequest := strings.Join([]string{
		method,
		uri,
		canonicalQuery(query),
		canonicalHeaderText,
		signedHeaderText,
		payloadHash,
	}, "\n")

	scope := credentialScopeFor(t, targetRegion, targetService)
	stringToSign := strings.Join([]string{
		algorithm,
		xDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signature := hex.EncodeToString(hmacSHA256(signingKeyFor(secretKey, t, targetRegion, targetService), stringToSign))
	signed["Authorization"] = fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", algorithm, accessKey, scope, signedHeaderText, signature)
	return signed
}

func (c *ImageGenerator) httpClient() *http.Client {
	client := &http.Client{
		Timeout: globals.HttpMaxTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	config := c.GetProxy()
	if config.ProxyType == globals.NoneProxyType {
		return client
	}

	if config.ProxyType == globals.HttpProxyType || config.ProxyType == globals.HttpsProxyType {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			globals.Warn(fmt.Sprintf("[jimeng-api] failed to parse proxy url: %s", err))
			return client
		}
		if config.Username != "" || config.Password != "" {
			proxyURL.User = url.UserPassword(config.Username, config.Password)
		}
		client.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		return client
	}

	if config.ProxyType == globals.Socks5ProxyType {
		var auth *proxy.Auth
		if config.Username != "" || config.Password != "" {
			auth = &proxy.Auth{User: config.Username, Password: config.Password}
		}
		dialer, err := proxy.SOCKS5("tcp", config.Proxy, auth, proxy.Direct)
		if err != nil {
			globals.Warn(fmt.Sprintf("[jimeng-api] failed to create socks5 proxy: %s", err))
			return client
		}
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return client
}

func (c *ImageGenerator) do(ctx context.Context, action string, body interface{}, out *APIResponse) error {
	if c.accessKey == "" || c.secretKey == "" {
		return fmt.Errorf("jimeng-api requires secret in AK|SK format")
	}

	endpoint := c.endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid jimeng-api endpoint: %w", err)
	}

	query := url.Values{}
	query.Set("Action", action)
	query.Set("Version", apiVersion)
	parsed.RawQuery = canonicalQuery(query)

	uri := parsed.EscapedPath()
	if uri == "" {
		uri = "/"
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	headers := signHeaders(http.MethodPost, uri, query, map[string]string{
		"Content-Type":     "application/json",
		"Host":             parsed.Host,
		"X-Content-Sha256": sha256Hex(bodyBytes),
	}, bodyBytes, c.accessKey, c.secretKey, time.Now().UTC())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Host = parsed.Host
	for key, value := range headers {
		if strings.EqualFold(key, "Host") {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("jimeng-api invalid response (http %d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		globals.Warn(fmt.Sprintf("[jimeng-api] http status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes))))
	}

	return nil
}

func (c *ImageGenerator) Submit(ctx context.Context, req SubmitTaskRequest) (*APIResponse, error) {
	var resp APIResponse
	if err := c.do(ctx, submitAction, req, &resp); err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return &resp, fmt.Errorf(resp.ErrorMessage("jimeng submit failed"))
	}
	if resp.Data == nil || resp.Data.TaskID == "" {
		return &resp, fmt.Errorf("jimeng submit failed: missing task_id")
	}
	return &resp, nil
}

func (c *ImageGenerator) GetResult(ctx context.Context, req GetResultRequest) (*APIResponse, error) {
	var resp APIResponse
	if err := c.do(ctx, getAction, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// initialPollInterval 是首次轮询的间隔。任务通常几秒内完成，先用短间隔快速取回结果，
// 之后按 pollBackoffFactor 退避，最高不超过传入的 interval（上限）。
const (
	initialPollInterval = 2 * time.Second
	pollBackoffFactor   = 3 // 每次 ×3/2 退避
)

func (c *ImageGenerator) Poll(ctx context.Context, reqKey, taskID string, maxWait, interval time.Duration) (*APIResponse, error) {
	if maxWait <= 0 {
		maxWait = 10 * time.Minute
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}

	// interval 作为退避上限；从较短的初始间隔起步以更快返回结果。
	maxInterval := interval
	cur := initialPollInterval
	if cur > maxInterval {
		cur = maxInterval
	}

	reqJSON, _ := json.Marshal(GetResultOptions{ReturnURL: true})
	start := time.Now()
	deadline := start.Add(maxWait)

	for attempt := 1; ; attempt++ {
		resp, err := c.GetResult(ctx, GetResultRequest{
			ReqKey:  reqKey,
			TaskID:  taskID,
			ReqJSON: string(reqJSON),
		})
		if err != nil {
			globals.Warn(fmt.Sprintf("[jimeng-api] poll #%d query failed (task_id=%s, elapsed=%s): %s",
				attempt, taskID, time.Since(start).Truncate(time.Second), err))
			return nil, err
		}

		status := ""
		if resp.Data != nil {
			status = resp.Data.Status
		}
		globals.Debug(fmt.Sprintf("[jimeng-api] poll #%d task_id=%s status=%q elapsed=%s",
			attempt, taskID, status, time.Since(start).Truncate(time.Second)))

		switch status {
		case "done":
			if !resp.IsSuccess() {
				return resp, fmt.Errorf(resp.ErrorMessage("jimeng task failed"))
			}
			globals.Info(fmt.Sprintf("[jimeng-api] task done (task_id=%s, polls=%d, elapsed=%s)",
				taskID, attempt, time.Since(start).Truncate(time.Second)))
			return resp, nil
		case "not_found", "expired":
			return resp, fmt.Errorf("jimeng task %s: %s", status, taskID)
		case "in_queue", "generating", "":
			if status == "" && !resp.IsSuccess() {
				return resp, fmt.Errorf(resp.ErrorMessage("jimeng task failed"))
			}
		default:
			if !resp.IsSuccess() {
				return resp, fmt.Errorf(resp.ErrorMessage("jimeng task failed"))
			}
		}

		if time.Now().Add(cur).After(deadline) {
			globals.Warn(fmt.Sprintf("[jimeng-api] task timeout after %s (task_id=%s, status=%s, polls=%d)",
				maxWait, taskID, status, attempt))
			return resp, fmt.Errorf("jimeng task timeout after %s (task_id=%s, status=%s)", maxWait, taskID, status)
		}

		timer := time.NewTimer(cur)
		select {
		case <-ctx.Done():
			timer.Stop()
			return resp, ctx.Err()
		case <-timer.C:
		}

		// 退避到下一个间隔，封顶 maxInterval。
		if cur < maxInterval {
			cur = cur * pollBackoffFactor / 2
			if cur > maxInterval {
				cur = maxInterval
			}
		}
	}
}
