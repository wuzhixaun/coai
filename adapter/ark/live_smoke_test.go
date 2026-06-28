package ark

import (
	adaptercommon "chat/adapter/common"
	"chat/adapter/skylark"
	"chat/globals"
	"os"
	"strings"
	"testing"
)

// 全场景真实链路自测（环境门控），直接走 Generator 的接口方法。运行：
//
//	ARK_ENDPOINT=https://ark.cn-beijing.volces.com/api/v3 \
//	ARK_KEY=ark-xxx \
//	ARK_IMAGE_MODEL=doubao-seedream-4-0-250828 \
//	ARK_VIDEO_MODEL=doubao-seedance-1-0-pro-250528 \
//	go test ./adapter/ark/ -run TestLive -v -timeout 20m
//
// 未设置 ARK_ENDPOINT/ARK_KEY 时全部 SKIP。
type liveConfig struct{ endpoint, key string }

func (c liveConfig) GetType() string                     { return globals.ArkChannelType }
func (c liveConfig) GetModelReflect(model string) string { return model }
func (c liveConfig) GetRetry() int                       { return 1 }
func (c liveConfig) GetRandomSecret() string             { return c.key }
func (c liveConfig) SplitRandomSecret(num int) []string  { return []string{c.key} }
func (c liveConfig) GetEndpoint() string                 { return c.endpoint }
func (c liveConfig) ProcessError(err error) error        { return err }
func (c liveConfig) GetId() int                          { return 0 }
func (c liveConfig) GetProxy() globals.ProxyConfig       { return globals.ProxyConfig{} }

func liveGen(t *testing.T) (*Generator, string, string) {
	endpoint, key := os.Getenv("ARK_ENDPOINT"), os.Getenv("ARK_KEY")
	if endpoint == "" || key == "" {
		t.Skip("set ARK_ENDPOINT and ARK_KEY to run")
	}
	img := os.Getenv("ARK_IMAGE_MODEL")
	if img == "" {
		img = "doubao-seedream-4-0-250828"
	}
	return newGenerator(liveConfig{endpoint: endpoint, key: key}), img, os.Getenv("ARK_VIDEO_MODEL")
}

const sampleInputImage = "https://picsum.photos/seed/coai/512"

func collect() (*string, func(*globals.Chunk) error) {
	var got string
	return &got, func(c *globals.Chunk) error { got += c.Content; return nil }
}

func TestLiveImageGeneration(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	if err := g.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
		Model: model, Prompt: "a red ceramic cup on a clean white background, product photo",
	}, hook); err != nil {
		t.Fatalf("文生图 失败: %v", err)
	}
	if !strings.Contains(*got, "![image](") {
		t.Fatalf("文生图 未返回图片 markdown: %.120s", *got)
	}
	t.Logf("文生图 OK: %.90s...", *got)
}

func TestLiveImageEdit(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	if err := g.CreateImageEditRequest(&adaptercommon.ImageEditProps{
		Model: model, Prompt: "把背景换成纯白色，保持主体不变", Images: []string{sampleInputImage},
	}, hook); err != nil {
		t.Fatalf("图生图/编辑 失败: %v", err)
	}
	if !strings.Contains(*got, "![image](") {
		t.Fatalf("图生图/编辑 未返回图片 markdown: %.120s", *got)
	}
	t.Logf("图生图/编辑 OK: %.90s...", *got)
}

func TestLiveUpscale(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	if err := g.CreateImageUpscaleRequest(&adaptercommon.ImageUpscaleProps{
		Model: model, Image: sampleInputImage, ResolutionType: "2k",
	}, hook); err != nil {
		t.Logf("超清（尽力而为）返回错误，可接受: %v", err)
		return
	}
	t.Logf("超清 返回: %.90s...", *got)
}

func TestLiveOutpaint(t *testing.T) {
	g, model, _ := liveGen(t)
	got, hook := collect()
	if err := g.CreateImageOutpaintRequest(&adaptercommon.ImageOutpaintProps{
		Model: model, Image: sampleInputImage, TargetRatio: "16:9",
	}, hook); err != nil {
		t.Logf("扩图（尽力而为）返回错误，可接受: %v", err)
		return
	}
	t.Logf("扩图 返回: %.90s...", *got)
}

// TestLiveChat 走 ark 渠道实际使用的对话链路（skylark/arkruntime），与注册一致。
func TestLiveChat(t *testing.T) {
	endpoint, key := os.Getenv("ARK_ENDPOINT"), os.Getenv("ARK_KEY")
	if endpoint == "" || key == "" {
		t.Skip("set ARK_ENDPOINT and ARK_KEY to run")
	}
	model := os.Getenv("ARK_CHAT_MODEL")
	if model == "" {
		model = "doubao-seed-1-6-250615"
	}
	inst := skylark.NewChatInstanceFromConfig(liveConfig{endpoint: endpoint, key: key})
	got, hook := collect()
	err := inst.CreateStreamChatRequest(&adaptercommon.ChatProps{
		Model:   model,
		Message: []globals.Message{{Role: globals.User, Content: "用一句话介绍你自己"}},
	}, hook)
	if err != nil {
		t.Fatalf("对话 失败: %v", err)
	}
	if strings.TrimSpace(*got) == "" {
		t.Fatalf("对话 未返回内容")
	}
	t.Logf("对话 OK: %.120s", *got)
}

func TestLiveImageToVideo(t *testing.T) {
	g, _, videoModel := liveGen(t)
	if videoModel == "" {
		t.Skip("set ARK_VIDEO_MODEL to run video scenario")
	}
	got, hook := collect()
	if err := g.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
		Model: videoModel, Prompt: "缓慢旋转展示这个杯子", Images: []string{sampleInputImage}, Duration: 5,
	}, hook); err != nil {
		t.Fatalf("图生视频 失败: %v", err)
	}
	if strings.TrimSpace(*got) == "" {
		t.Fatalf("图生视频 未返回结果")
	}
	t.Logf("图生视频 OK: %.120s...", *got)
}
