package grsai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"

	"github.com/goccy/go-json"
)

func TestClassifyImages(t *testing.T) {
	in := []string{"  ", "https://a.com/x.png", "data:image/png;base64,AAAA", "BBBB"}
	out := classifyImages(in)
	want := []string{"https://a.com/x.png", "AAAA", "BBBB"}
	if len(out) != len(want) {
		t.Fatalf("got %v", out)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("got %v want %v", out, want)
		}
	}
}

func TestCreateImageEditRequest(t *testing.T) {
	globals.StorageResultDir = t.TempDir()

	// dl 提供结果图片的下载地址。
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("img"))
	}))
	defer dl.Close()

	var gotBody GenerateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t1", Status: "running"})
		case "/v1/api/result":
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t1", Status: "succeeded",
				Results: []TaskResult{{URL: dl.URL + "/a.png"}}})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageEditRequest(&adaptercommon.ImageEditProps{
		Model:  "nano-banana",
		Images: []string{"data:image/png;base64,AAAA"},
		Prompt: "make it white bg",
	}, hook)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if gotBody.Model != "nano-banana" || gotBody.Prompt == "" || len(gotBody.Images) != 1 || gotBody.ReplyType != "async" {
		t.Fatalf("body=%+v", gotBody)
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted=%v", emitted)
	}
}
