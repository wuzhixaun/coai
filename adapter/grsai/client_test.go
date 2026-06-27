package grsai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chat/globals"

	"github.com/goccy/go-json"
)

// fakeConfig 实现 globals.ChannelConfig，仅用于测试。
type fakeConfig struct {
	endpoint string
	secret   string
	proxy    globals.ProxyConfig
}

func (f fakeConfig) GetType() string                  { return "grsai" }
func (f fakeConfig) GetModelReflect(m string) string  { return m }
func (f fakeConfig) GetRetry() int                    { return 0 }
func (f fakeConfig) GetRandomSecret() string          { return f.secret }
func (f fakeConfig) SplitRandomSecret(n int) []string { return []string{f.secret} }
func (f fakeConfig) GetEndpoint() string              { return f.endpoint }
func (f fakeConfig) ProcessError(err error) error     { return err }
func (f fakeConfig) GetId() int                       { return 1 }
func (f fakeConfig) GetProxy() globals.ProxyConfig    { return f.proxy }

func TestSubmitAndPoll(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("auth header=%q", got)
		}
		switch r.URL.Path {
		case "/v1/api/generate":
			if r.Method != http.MethodPost {
				t.Errorf("generate method=%s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "task-1", Status: "running"})
		case "/v1/api/result":
			if r.Method != http.MethodGet {
				t.Errorf("result method=%s", r.Method)
			}
			if got := r.URL.Query().Get("id"); got != "task-1" {
				t.Errorf("result id query=%q", got)
			}
			polls++
			if polls < 2 {
				_ = json.NewEncoder(w).Encode(TaskResponse{ID: "task-1", Status: "running", Progress: 50})
				return
			}
			_ = json.NewEncoder(w).Encode(TaskResponse{
				ID: "task-1", Status: "succeeded", Progress: 100,
				Results: []TaskResult{{URL: "https://example.com/a.png"}},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	spec, _ := GetModelSpec("nano-banana")
	submit, err := c.Submit(context.Background(), spec, GenerateRequest{Model: "nano-banana", Prompt: "x", ReplyType: "async"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submit.ID != "task-1" {
		t.Fatalf("id=%q", submit.ID)
	}
	res, err := c.PollResult(context.Background(), submit.ID, 10*time.Second, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !res.IsSucceeded() || len(res.Results) != 1 || res.Results[0].URL == "" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestPollFailedReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t", Status: "failed", Error: "generate failed"})
	}))
	defer srv.Close()
	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	_, err := c.PollResult(context.Background(), "t", 5*time.Second, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected error on failed status")
	}
}
