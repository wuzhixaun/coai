package grsai

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"

	"github.com/goccy/go-json"
)

// surfaceBVideoServer 模拟 SurfaceB 视频接口：POST /v1/video/veo 返回 SSE 流，
// 先推一个 running 帧，再推终态 succeeded 帧（url 指向 resultURL）。结果直接来自流。
func surfaceBVideoServer(t *testing.T, gotBody *GenerateRequest, resultURL string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/video/veo" {
			t.Errorf("unexpected path %s", r.URL.Path)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"v1\",\"status\":\"running\",\"progress\":1}\n\n")
		fmt.Fprintf(w, "data: {\"id\":\"v1\",\"status\":\"succeeded\",\"progress\":100,\"url\":%q}\n\n", resultURL)
	}))
}

func TestCreateImageToVideoRequest(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("mp4-bytes"))
	}))
	defer dl.Close()

	var gotBody GenerateRequest
	srv := surfaceBVideoServer(t, &gotBody, dl.URL+"/out.mp4")
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
		Model:  "veo3.1-fast",
		Images: []string{"data:image/png;base64,AAAA"},
		Prompt: "make it move",
	}, hook)
	if err != nil {
		t.Fatalf("video: %v", err)
	}
	if gotBody.Model != "veo3.1-fast" || len(gotBody.Images) != 1 {
		t.Fatalf("body=%+v", gotBody)
	}
	if len(emitted) != 1 || emitted[0] == "" {
		t.Fatalf("emitted=%v", emitted)
	}
}

func TestCreateImageToVideoRequestFallbackSourceURL(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
	// dl 下载视频时返回 500，触发落地失败回退到源地址。
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dl.Close()

	srcURL := dl.URL + "/out.mp4"
	var gotBody GenerateRequest
	srv := surfaceBVideoServer(t, &gotBody, srcURL)
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
		Model:  "veo3.1-fast",
		Images: []string{"data:image/png;base64,AAAA"},
		Prompt: "make it move",
	}, hook)
	if err != nil {
		t.Fatalf("video: %v", err)
	}
	if len(emitted) != 1 || emitted[0] != srcURL {
		t.Fatalf("expected fallback to source url %q, got %v", srcURL, emitted)
	}
}
