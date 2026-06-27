package openai

import (
	"chat/globals"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newMediaServer 起一个对 /v1/chat/completions 返回固定 body 的 httptest server。
// body 应为完整的 JSON 字符串。
func newMediaServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// chatBody 构造 choices[0].message.content = content 的响应 JSON。
func chatBody(content string) string {
	// 用 %q 对 content 做 JSON 字符串转义（足够覆盖本测试的 ASCII / 中文用例）。
	return fmt.Sprintf(`{"choices":[{"index":0,"message":{"role":"assistant","content":%q}}]}`, content)
}

// recordHook 记录 hook 被调用的次数与拼接内容。
type recordHook struct {
	calls int
	buf   strings.Builder
}

func (h *recordHook) hook(chunk *globals.Chunk) error {
	h.calls++
	h.buf.WriteString(chunk.Content)
	return nil
}

// withImageStoreDisabled 确保 AcceptImageStore 在测试期间为 false（StoreImage 原样返回 url），
// 结尾恢复原值，避免污染其它测试。
func withImageStoreDisabled(t *testing.T) {
	t.Helper()
	prev := globals.AcceptImageStore
	globals.AcceptImageStore = false
	t.Cleanup(func() { globals.AcceptImageStore = prev })
}

func newTestGenerator(serverURL string) *Generator {
	return &Generator{ChatInstance: NewChatInstance(serverURL, "sk-test")}
}

func TestRunChatMediaBase64Image(t *testing.T) {
	withImageStoreDisabled(t)
	srv := newMediaServer(t, chatBody("![image](data:image/png;base64,AAA)"))
	g := newTestGenerator(srv.URL)

	var h recordHook
	if err := g.runChatMedia("nano-banana", "画只猫", nil, false, globals.ProxyConfig{}, h.hook); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if h.calls != 1 {
		t.Fatalf("hook calls=%d want 1", h.calls)
	}
	if !strings.Contains(h.buf.String(), "data:image/png;base64,AAA") {
		t.Fatalf("markdown missing base64: %q", h.buf.String())
	}
}

func TestRunChatMediaHTTPImage(t *testing.T) {
	withImageStoreDisabled(t)
	srv := newMediaServer(t, chatBody("done ![image](https://x.example.com/a.png)"))
	g := newTestGenerator(srv.URL)

	var h recordHook
	if err := g.runChatMedia("nano-banana", "画只猫", nil, false, globals.ProxyConfig{}, h.hook); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if h.calls != 1 {
		t.Fatalf("hook calls=%d want 1", h.calls)
	}
	// AcceptImageStore=false → StoreImage 原样返回 url，markdown 应为 ![image](url)
	if got := h.buf.String(); got != "![image](https://x.example.com/a.png)" {
		t.Fatalf("markdown=%q want ![image](https://x.example.com/a.png)", got)
	}
}

func TestRunChatMediaVideo(t *testing.T) {
	withImageStoreDisabled(t)
	srv := newMediaServer(t, chatBody("![video](https://x.example.com/v.mp4)"))
	g := newTestGenerator(srv.URL)

	var h recordHook
	if err := g.runChatMedia("veo", "生成视频", nil, true, globals.ProxyConfig{}, h.hook); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if h.calls != 1 {
		t.Fatalf("hook calls=%d want 1", h.calls)
	}
	// 视频分支回推裸 url（无 markdown 包裹）
	if got := h.buf.String(); got != "https://x.example.com/v.mp4" {
		t.Fatalf("video chunk=%q want bare url", got)
	}
}

func TestRunChatMediaUpstreamError(t *testing.T) {
	withImageStoreDisabled(t)
	// message 里故意含 % —— 验证 fmt.Errorf("%s", ...) 透传不乱码。
	srv := newMediaServer(t, `{"error":{"message":"boom %s percent"}}`)
	g := newTestGenerator(srv.URL)

	var h recordHook
	err := g.runChatMedia("nano-banana", "画只猫", nil, false, globals.ProxyConfig{}, h.hook)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%q want contains boom", err.Error())
	}
	if !strings.Contains(err.Error(), "%s") {
		t.Fatalf("err=%q want literal %%s preserved", err.Error())
	}
	if h.calls != 0 {
		t.Fatalf("hook calls=%d want 0 (不扣费/不部分回推)", h.calls)
	}
}

func TestRunChatMediaNoMedia(t *testing.T) {
	withImageStoreDisabled(t)
	srv := newMediaServer(t, chatBody("这是一段纯文本没有任何图片或视频"))
	g := newTestGenerator(srv.URL)

	var h recordHook
	err := g.runChatMedia("nano-banana", "画只猫", nil, false, globals.ProxyConfig{}, h.hook)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if h.calls != 0 {
		t.Fatalf("hook calls=%d want 0", h.calls)
	}
}
