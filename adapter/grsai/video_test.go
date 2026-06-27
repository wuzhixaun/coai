package grsai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"

	"github.com/goccy/go-json"
)

func TestCreateImageToVideoRequest(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("mp4-bytes"))
	}))
	defer dl.Close()

	var gotBody GenerateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "v1", Status: "running"})
		case "/v1/api/result":
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "v1", Status: "succeeded",
				Results: []TaskResult{{URL: dl.URL + "/out.mp4"}}})
		}
	}))
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
		Model:  "veo",
		Images: []string{"data:image/png;base64,AAAA"},
		Prompt: "make it move",
	}, hook)
	if err != nil {
		t.Fatalf("video: %v", err)
	}
	if gotBody.Model != "veo" || len(gotBody.Images) != 1 {
		t.Fatalf("body=%+v", gotBody)
	}
	if len(emitted) != 1 || emitted[0] == "" {
		t.Fatalf("emitted=%v", emitted)
	}
}
