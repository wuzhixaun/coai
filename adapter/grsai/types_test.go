package grsai

import "testing"

func TestGetModelSpec(t *testing.T) {
	cases := []struct {
		model   string
		ok      bool
		path    string
		surface Surface
		cap     Capability
	}{
		{"nano-banana", true, "/v1/api/generate", SurfaceA, CapabilityGenerate},
		{"nano-banana-2", true, "/v1/api/generate", SurfaceA, CapabilityGenerate},
		{"gpt-image-2", true, "/v1/draw/completions", SurfaceB, CapabilityGenerate},
		{"veo3.1-fast", true, "/v1/video/veo", SurfaceB, CapabilityVideo},
		{"veo3.1-pro", true, "/v1/video/veo", SurfaceB, CapabilityVideo},
		{"gpt-image", false, "", SurfaceA, CapabilityGenerate}, // 旧名已下线
		{"veo", false, "", SurfaceA, CapabilityVideo},          // 旧名已下线
		{"unknown-model", false, "", SurfaceA, CapabilityGenerate},
	}
	for _, c := range cases {
		spec, ok := GetModelSpec(c.model)
		if ok != c.ok {
			t.Fatalf("%s: ok=%v want %v", c.model, ok, c.ok)
		}
		if ok && (spec.Path != c.path || spec.Surface != c.surface || spec.Capability != c.cap) {
			t.Fatalf("%s: spec=%+v", c.model, spec)
		}
	}
}

func TestTaskResponseState(t *testing.T) {
	if !(&TaskResponse{Status: "succeeded"}).IsSucceeded() {
		t.Fatal("succeeded should be succeeded")
	}
	if !(&TaskResponse{Status: "failed"}).IsTerminal() {
		t.Fatal("failed should be terminal")
	}
	if (&TaskResponse{Status: "running"}).IsTerminal() {
		t.Fatal("running should not be terminal")
	}
	if got := (&TaskResponse{Status: "failed", Error: "boom"}).ErrorMessage("def"); got != "boom" {
		t.Fatalf("ErrorMessage=%q", got)
	}
	if got := (&TaskResponse{Status: "failed"}).ErrorMessage("def"); got != "def" {
		t.Fatalf("ErrorMessage fallback=%q", got)
	}
	// violation 为终态但非成功；Error 为空时回退到 "content violation"。
	if !(&TaskResponse{Status: "violation"}).IsTerminal() {
		t.Fatal("violation should be terminal")
	}
	if (&TaskResponse{Status: "violation"}).IsSucceeded() {
		t.Fatal("violation should not be succeeded")
	}
	if got := (&TaskResponse{Status: "violation"}).ErrorMessage("def"); got != "content violation" {
		t.Fatalf("violation ErrorMessage=%q", got)
	}
}

func TestTaskResponseBState(t *testing.T) {
	if !(&TaskResponseB{Status: "succeeded"}).IsSucceeded() {
		t.Fatal("succeeded should be succeeded")
	}
	if !(&TaskResponseB{Status: "violation"}).IsTerminal() {
		t.Fatal("violation should be terminal")
	}
	if (&TaskResponseB{Status: "running"}).IsTerminal() {
		t.Fatal("running should not be terminal")
	}
	// failure_reason 在 error 缺失时作为兜底。
	if got := (&TaskResponseB{Status: "failed", FailureReason: "boom"}).ErrorMessage("def"); got != "boom" {
		t.Fatalf("ErrorMessage=%q", got)
	}
	// URLs 优先取 results[].url。
	rb := &TaskResponseB{Results: []TaskResult{{URL: "https://a/x.png"}}, URL: "https://a/top.png"}
	if got := rb.URLs(); len(got) != 1 || got[0] != "https://a/x.png" {
		t.Fatalf("URLs results-first=%v", got)
	}
	// results 为空时回退顶层 url。
	rb2 := &TaskResponseB{URL: "https://a/top.png"}
	if got := rb2.URLs(); len(got) != 1 || got[0] != "https://a/top.png" {
		t.Fatalf("URLs fallback=%v", got)
	}
}
