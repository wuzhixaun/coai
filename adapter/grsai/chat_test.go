package grsai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"

	"github.com/goccy/go-json"
)

// TestCreateStreamChatRequestGenerate：纯文本聊天消息 → 文生图（SurfaceA nano-banana）。
func TestCreateStreamChatRequestGenerate(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
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
	err := c.CreateStreamChatRequest(&adaptercommon.ChatProps{
		Model:   "nano-banana",
		Message: []globals.Message{{Role: globals.User, Content: "生成一个苹果"}},
	}, hook)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if gotBody.Prompt != "生成一个苹果" {
		t.Fatalf("prompt=%q", gotBody.Prompt)
	}
	if len(emitted) != 1 || emitted[0] == "" {
		t.Fatalf("emitted=%v", emitted)
	}
}

// TestCreateStreamChatRequestEditWithImage：消息含图片 → 走图生图（带 images）。
func TestCreateStreamChatRequestEditWithImage(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
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
	hook := func(ch *globals.Chunk) error { return nil }
	err := c.CreateStreamChatRequest(&adaptercommon.ChatProps{
		Model: "nano-banana",
		Message: []globals.Message{
			{Role: globals.User, Content: "改成白底 ![image](https://a.com/x.png)"},
		},
	}, hook)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if len(gotBody.Images) != 1 || gotBody.Images[0] != "https://a.com/x.png" {
		t.Fatalf("images=%v", gotBody.Images)
	}
	if gotBody.Prompt != "改成白底" {
		t.Fatalf("prompt=%q", gotBody.Prompt)
	}
}
