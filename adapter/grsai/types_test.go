package grsai

import "testing"

func TestGetModelSpec(t *testing.T) {
	cases := []struct {
		model string
		ok    bool
		path  string
		cap   Capability
	}{
		{"nano-banana", true, "/v1/api/generate", CapabilityGenerate},
		{"nano-banana-2", true, "/v1/api/generate", CapabilityGenerate},
		{"gpt-image", true, "/v1/api/generate", CapabilityGenerate},
		{"veo", true, "/v1/api/generate", CapabilityVideo},
		{"unknown-model", false, "", CapabilityGenerate},
	}
	for _, c := range cases {
		spec, ok := GetModelSpec(c.model)
		if ok != c.ok {
			t.Fatalf("%s: ok=%v want %v", c.model, ok, c.ok)
		}
		if ok && (spec.Path != c.path || spec.Capability != c.cap) {
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
