package grsai

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"chat/globals"
)

func TestStoreRemoteURL(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n-fake-bytes"))
	}))
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: "https://grsaiapi.com", secret: "sk-test"})
	public, err := c.storeRemoteURL(srv.URL+"/x.png", true)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if !strings.HasPrefix(public, globals.ResultPublicURL("")) || public == "" {
		t.Fatalf("public url=%q", public)
	}
	// 文件应真实落地
	entries, _ := os.ReadDir(globals.StorageResultDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
}

func TestResultFilenameExt(t *testing.T) {
	if name := resultFilename("src", ""); !strings.HasSuffix(name, ".png") {
		t.Fatalf("default ext should be png: %s", name)
	}
	if name := resultFilename("src", ".mp4"); !strings.HasSuffix(name, ".mp4") {
		t.Fatalf("mp4 ext: %s", name)
	}
}
