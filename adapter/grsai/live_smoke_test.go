package grsai

import (
	"os"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"
)

// 仅当设置 GRSAI_API_KEY 时运行，真实打一次 nano-banana 生图。
// 运行：GRSAI_API_KEY=sk-xxx go test ./adapter/grsai/ -run TestLiveGenerate -v
func TestLiveGenerate(t *testing.T) {
	key := os.Getenv("GRSAI_API_KEY")
	if key == "" {
		t.Skip("set GRSAI_API_KEY to run live smoke test")
	}
	endpoint := os.Getenv("GRSAI_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://grsaiapi.com"
	}
	globals.StorageResultDir = t.TempDir()

	c := newGenerator(fakeConfig{endpoint: endpoint, secret: key})
	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
		Model:  "nano-banana",
		Prompt: "a cute corgi poster, vivid colors",
	}, hook)
	if err != nil {
		t.Fatalf("live generate: %v", err)
	}
	if len(emitted) == 0 {
		t.Fatal("no image emitted")
	}
	t.Logf("emitted: %v", emitted)
}
