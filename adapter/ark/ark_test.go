package ark

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeInputImage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  ", ""},
		{"https://x/a.png", "https://x/a.png"},
		{"http://x/a.png", "http://x/a.png"},
		{"data:image/jpeg;base64,abc", "data:image/jpeg;base64,abc"},
		{"iVBORw0KGgo=", "data:image/png;base64,iVBORw0KGgo="},
	}
	for _, c := range cases {
		if got := normalizeInputImage(c.in); got != c.want {
			t.Errorf("normalizeInputImage(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeImagesDropsBlank(t *testing.T) {
	got := normalizeImages([]string{"", "  ", "https://x/a.png", "raw"})
	if len(got) != 2 || got[0] != "https://x/a.png" || got[1] != "data:image/png;base64,raw" {
		t.Fatalf("normalizeImages got %v", got)
	}
}

func TestBuildVideoPrompt(t *testing.T) {
	if got := buildVideoPrompt("  转一圈  ", 5); got != "转一圈 --duration 5" {
		t.Errorf("with duration got %q", got)
	}
	if got := buildVideoPrompt("转一圈", 0); got != "转一圈" {
		t.Errorf("no duration got %q", got)
	}
	if got := buildVideoPrompt("", 10); got != "--duration 10" {
		t.Errorf("empty prompt got %q", got)
	}
}

func TestModelCapabilityDetection(t *testing.T) {
	if !isImageModel("doubao-seedream-4-0-250828") {
		t.Error("seedream should be image model")
	}
	if !isVideoModel("doubao-seedance-2-0-260128") {
		t.Error("seedance should be video model")
	}
	if isImageModel("doubao-seed-1-6-250615") || isVideoModel("doubao-seed-1-6-250615") {
		t.Error("doubao-seed-1-6 should be a chat model (neither image nor video)")
	}
}

func TestLatestPromptAndImages(t *testing.T) {
	prompt, imgs := latestPromptAndImages(&adaptercommon.ChatProps{
		Message: []globals.Message{
			{Role: globals.User, Content: "第一条"},
			{Role: globals.Assistant, Content: "回复"},
			{Role: globals.User, Content: "做个视频 ![image](https://x/a.png)"},
		},
	})
	if prompt != "做个视频" {
		t.Errorf("prompt=%q want 做个视频", prompt)
	}
	if len(imgs) != 1 || imgs[0] != "https://x/a.png" {
		t.Errorf("imgs=%v", imgs)
	}
}

func TestImageRole(t *testing.T) {
	if got := imageRole(0); got != "first_frame" {
		t.Errorf("0 images role=%q want first_frame", got)
	}
	if got := imageRole(1); got != "first_frame" {
		t.Errorf("1 image role=%q want first_frame", got)
	}
	if got := imageRole(3); got != "reference_image" {
		t.Errorf("3 images role=%q want reference_image", got)
	}
}

func TestResultFilename(t *testing.T) {
	a := resultFilename("https://x/v.mp4", ".mp4")
	if !strings.HasPrefix(a, "ark_") || !strings.HasSuffix(a, ".mp4") {
		t.Fatalf("resultFilename shape: %s", a)
	}
	if resultFilename("https://x/v.mp4", "") == a {
		t.Fatal("expected unique filenames across calls")
	}
}

func TestInterfaceCompliance(t *testing.T) {
	g := newGenerator(stubConfig{})
	if _, ok := interface{}(g).(adaptercommon.ImageGenerationFactory); !ok {
		t.Error("not ImageGenerationFactory")
	}
	if _, ok := interface{}(g).(adaptercommon.ImageEditFactory); !ok {
		t.Error("not ImageEditFactory")
	}
	if _, ok := interface{}(g).(adaptercommon.ImageUpscaleFactory); !ok {
		t.Error("not ImageUpscaleFactory")
	}
	if _, ok := interface{}(g).(adaptercommon.ImageOutpaintFactory); !ok {
		t.Error("not ImageOutpaintFactory")
	}
	if _, ok := interface{}(g).(adaptercommon.ImageToVideoFactory); !ok {
		t.Error("not ImageToVideoFactory")
	}
}

func TestImageEditRequiresInputImage(t *testing.T) {
	g := newGenerator(stubConfig{})
	calls := 0
	err := g.CreateImageEditRequest(&adaptercommon.ImageEditProps{Model: "m", Prompt: "p"},
		func(*globals.Chunk) error { calls++; return nil })
	if err == nil {
		t.Fatal("expected error when no input image")
	}
	if calls != 0 {
		t.Fatalf("hook must not be called, got %d", calls)
	}
}

// ---- httptest 覆盖 /images/generations 关键分支（含「错误/无图→不扣费」不变量）----

func newHTTPGen(t *testing.T, h http.HandlerFunc) (*Generator, func()) {
	t.Helper()
	prevDir := globals.StorageResultDir
	globals.StorageResultDir = t.TempDir() // 结果落地到临时目录，避免污染仓库
	srv := httptest.NewServer(h)
	g := newGenerator(stubConfig{endpoint: srv.URL})
	return g, func() { srv.Close(); globals.StorageResultDir = prevDir }
}

func TestRunImageURLResult(t *testing.T) {
	g, closeFn := newHTTPGen(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/images/generations") {
			// 返回指向同一测试服务器的图片 URL，供 storeImageURL 下载落地
			_, _ = w.Write([]byte(`{"data":[{"url":"http://` + r.Host + `/pic.png"}]}`))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake")) // 假 PNG 字节
	})
	defer closeFn()

	var got string
	err := g.runImage("m", "p", nil, "", func(c *globals.Chunk) error { got += c.Content; return nil })
	if err != nil {
		t.Fatalf("runImage err: %v", err)
	}
	if !strings.Contains(got, "![image](/storage/results/ark_") {
		t.Fatalf("expected local stored markdown, got %q", got)
	}
}

func TestRunImageB64Result(t *testing.T) {
	g, closeFn := newHTTPGen(t, func(w http.ResponseWriter, r *http.Request) {
		// "AAAA" 是合法 base64（解码为 3 字节），落地为本地文件
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAAA"}]}`))
	})
	defer closeFn()

	var got string
	err := g.runImage("m", "p", nil, "", func(c *globals.Chunk) error { got += c.Content; return nil })
	if err != nil {
		t.Fatalf("runImage err: %v", err)
	}
	if !strings.Contains(got, "![image](/storage/results/ark_") {
		t.Fatalf("expected local stored markdown, got %q", got)
	}
}

func TestRunImageErrorBodyNoCharge(t *testing.T) {
	defer withImageStoreDisabled()()
	g, closeFn := newHTTPGen(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"model not enabled","code":"AccessDenied"}}`))
	})
	defer closeFn()

	calls := 0
	err := g.runImage("m", "p", nil, "", func(*globals.Chunk) error { calls++; return nil })
	if err == nil || !strings.Contains(err.Error(), "model not enabled") {
		t.Fatalf("expected upstream error, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("hook must not be called on error, got %d", calls)
	}
}

func TestRunImageEmptyDataNoCharge(t *testing.T) {
	defer withImageStoreDisabled()()
	g, closeFn := newHTTPGen(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer closeFn()

	calls := 0
	err := g.runImage("m", "p", nil, "", func(*globals.Chunk) error { calls++; return nil })
	if err == nil {
		t.Fatal("expected error on empty data")
	}
	if calls != 0 {
		t.Fatalf("hook must not be called, got %d", calls)
	}
}

// ---- 测试桩 ----

func withImageStoreDisabled() func() {
	prev := globals.AcceptImageStore
	globals.AcceptImageStore = false
	return func() { globals.AcceptImageStore = prev }
}

type stubConfig struct{ endpoint string }

func (s stubConfig) GetType() string                   { return globals.ArkChannelType }
func (s stubConfig) GetModelReflect(model string) string { return model }
func (s stubConfig) GetRetry() int                     { return 1 }
func (s stubConfig) GetRandomSecret() string           { return "test-key" }
func (s stubConfig) SplitRandomSecret(num int) []string {
	out := make([]string, num)
	for i := range out {
		out[i] = "test-key"
	}
	return out
}
func (s stubConfig) GetEndpoint() string            { return s.endpoint }
func (s stubConfig) ProcessError(err error) error   { return err }
func (s stubConfig) GetId() int                     { return 0 }
func (s stubConfig) GetProxy() globals.ProxyConfig  { return globals.ProxyConfig{} }
