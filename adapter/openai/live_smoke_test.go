package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"os"
	"strings"
	"testing"
)

// 全场景真实链路自测（环境门控）。运行：
//
//	OPENAI_BRIDGE_ENDPOINT=https://api.nycatai.com/image \
//	OPENAI_BRIDGE_KEY=sk-xxx \
//	OPENAI_BRIDGE_MODEL=nano-banana-2 \
//	OPENAI_BRIDGE_VIDEO_MODEL=grok-video-15s \
//	go test ./adapter/openai/ -run TestLiveBridge -v
//
// 未设置 endpoint/key 时全部 SKIP。
func liveGen(t *testing.T) (*Generator, string, string) {
	endpoint, key := os.Getenv("OPENAI_BRIDGE_ENDPOINT"), os.Getenv("OPENAI_BRIDGE_KEY")
	if endpoint == "" || key == "" {
		t.Skip("set OPENAI_BRIDGE_ENDPOINT and OPENAI_BRIDGE_KEY to run")
	}
	model := os.Getenv("OPENAI_BRIDGE_MODEL")
	if model == "" {
		model = "nano-banana-2"
	}
	return &Generator{ChatInstance: NewChatInstance(endpoint, key)}, model, os.Getenv("OPENAI_BRIDGE_VIDEO_MODEL")
}

// 公网可访问的输入图，供编辑/超分/扩图用。
const sampleInputImage = "https://picsum.photos/seed/coai/512"

func collect() (*string, func(*globals.Chunk) error) {
	var got string
	return &got, func(c *globals.Chunk) error { got += c.Content; return nil }
}

func TestLiveBridgeImageGeneration(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	err := g.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
		Model: model, Prompt: "a red ceramic cup on a clean white background, product photo",
	}, hook)
	if err != nil {
		t.Fatalf("文生图 失败: %v", err)
	}
	if !strings.Contains(*got, "![image](") {
		t.Fatalf("文生图 未返回图片 markdown: %.100s", *got)
	}
	t.Logf("文生图 OK: %.80s...", *got)
}

func TestLiveBridgeImageEdit(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	err := g.CreateImageEditRequest(&adaptercommon.ImageEditProps{
		Model: model, Prompt: "把背景换成纯白色，保持主体不变", Images: []string{sampleInputImage},
	}, hook)
	if err != nil {
		t.Fatalf("图生图/编辑 失败: %v", err)
	}
	if !strings.Contains(*got, "![image](") {
		t.Fatalf("图生图/编辑 未返回图片 markdown: %.100s", *got)
	}
	t.Logf("图生图/编辑 OK: %.80s...", *got)
}

func TestLiveBridgeUpscale(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	err := g.CreateImageUpscaleRequest(&adaptercommon.ImageUpscaleProps{
		Model: model, Image: sampleInputImage, ResolutionType: "2k",
	}, hook)
	if err != nil {
		t.Logf("超分（尽力而为）返回错误，可接受: %v", err)
		return
	}
	t.Logf("超分 返回: %.80s...", *got)
}

func TestLiveBridgeOutpaint(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	err := g.CreateImageOutpaintRequest(&adaptercommon.ImageOutpaintProps{
		Model: model, Image: sampleInputImage, TargetRatio: "16:9", Prompt: "",
	}, hook)
	if err != nil {
		t.Logf("扩图（尽力而为）返回错误，可接受: %v", err)
		return
	}
	t.Logf("扩图 返回: %.80s...", *got)
}

func TestLiveBridgeImageToVideo(t *testing.T) {
	g, _, videoModel := liveGen(t)
	if videoModel == "" {
		t.Skip("set OPENAI_BRIDGE_VIDEO_MODEL to run video scenario")
	}
	got, hook := collect()
	err := g.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
		Model: videoModel, Prompt: "缓慢旋转展示这个杯子", Images: []string{sampleInputImage},
	}, hook)
	if err != nil {
		t.Logf("图生视频 返回错误（站点未开通视频模型时预期如此）: %v", err)
		return
	}
	t.Logf("图生视频 OK: %.80s...", *got)
}
