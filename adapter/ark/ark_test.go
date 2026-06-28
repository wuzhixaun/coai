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
	srv := httptest.NewServer(h)
	g := newGenerator(stubConfig{endpoint: srv.URL})
	return g, srv.Close
}

func TestRunImageURLResult(t *testing.T) {
	defer withImageStoreDisabled()()
	g, closeFn := newHTTPGen(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/images/generations") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn/x.png"}]}`))
	})
	defer closeFn()

	var got string
	err := g.runImage("m", "p", nil, "", func(c *globals.Chunk) error { got += c.Content; return nil })
	if err != nil {
		t.Fatalf("runImage err: %v", err)
	}
	if !strings.Contains(got, "![image](https://cdn/x.png)") {
		t.Fatalf("markdown got %q", got)
	}
}

func TestRunImageB64Result(t *testing.T) {
	defer withImageStoreDisabled()()
	g, closeFn := newHTTPGen(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAAA"}]}`))
	})
	defer closeFn()

	var got string
	err := g.runImage("m", "p", nil, "", func(c *globals.Chunk) error { got += c.Content; return nil })
	if err != nil {
		t.Fatalf("runImage err: %v", err)
	}
	if !strings.Contains(got, "data:image/png;base64,AAAA") {
		t.Fatalf("inline b64 got %q", got)
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
