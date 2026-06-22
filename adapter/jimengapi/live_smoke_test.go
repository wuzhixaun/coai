package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// liveMockConfig 是一个最小的 ChannelConfig，用于联调冒烟测试。
type liveMockConfig struct {
	ak string
	sk string
}

func (c liveMockConfig) GetType() string                  { return "jimeng-api" }
func (c liveMockConfig) GetModelReflect(model string) string { return model }
func (c liveMockConfig) GetRetry() int                     { return 0 }
func (c liveMockConfig) GetRandomSecret() string           { return c.ak + "|" + c.sk }
func (c liveMockConfig) SplitRandomSecret(num int) []string {
	parts := []string{c.ak, c.sk}
	for len(parts) < num {
		parts = append(parts, "")
	}
	return parts
}
func (c liveMockConfig) GetEndpoint() string            { return "https://visual.volcengineapi.com" }
func (c liveMockConfig) ProcessError(err error) error   { return err }
func (c liveMockConfig) GetId() int                     { return 0 }
func (c liveMockConfig) GetProxy() globals.ProxyConfig  { return globals.ProxyConfig{ProxyType: globals.NoneProxyType} }

// TestLiveSmoke 仅在显式设置 JIMENG_LIVE=1 时运行，会发起真实火山 API 调用并消耗付费额度。
func TestLiveSmoke(t *testing.T) {
	if os.Getenv("JIMENG_LIVE") != "1" {
		t.Skip("set JIMENG_LIVE=1 to run live smoke test (consumes paid quota)")
	}
	ak := os.Getenv("JIMENG_AK")
	sk := os.Getenv("JIMENG_SK")
	if ak == "" || sk == "" {
		t.Fatal("JIMENG_AK / JIMENG_SK required")
	}

	gen := NewImageGeneratorFromConfig(liveMockConfig{ak: ak, sk: sk}).(*ImageGenerator)

	// 1) 文生图：验证签名 + submit + poll + 结果解析 + 转存
	t.Run("generate", func(t *testing.T) {
		size := 1024 * 1024
		single := true
		var results []string
		err := gen.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
			Model:       "jimeng-seedream-4.6",
			Prompt:      "一只戴着蓝色围巾的橘猫，柔和影棚光线，电商主图",
			N:           1,
			Size:        &size,
			ForceSingle: &single,
		}, func(chunk *globals.Chunk) error {
			if chunk != nil && chunk.Content != "" {
				results = append(results, chunk.Content)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("generate failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("generate returned no image")
		}
		t.Logf("generate ok: %s", strings.Join(results, " | "))
	})

	// 2) 图生图编辑：验证 binary_data_base64 输入字段是否被官方接口接受
	t.Run("edit_binary_base64", func(t *testing.T) {
		// 生成一张 512x512 纯色 PNG 作为编辑输入（官方对最小尺寸有要求）。
		var buf bytes.Buffer
		img := image.NewRGBA(image.Rect(0, 0, 512, 512))
		for y := 0; y < 512; y++ {
			for x := 0; x < 512; x++ {
				img.Set(x, y, color.RGBA{R: 200, G: 80, B: 60, A: 255})
			}
		}
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("encode png: %v", err)
		}
		inputB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

		var results []string
		err := gen.CreateImageEditRequest(&adaptercommon.ImageEditProps{
			Model:  "jimeng-seedream-4.6",
			Images: []string{inputB64},
			Prompt: "把背景换成纯白色影棚，保持主体不变",
		}, func(chunk *globals.Chunk) error {
			if chunk != nil && chunk.Content != "" {
				results = append(results, chunk.Content)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("edit failed (binary_data_base64 may be wrong field): %v", err)
		}
		if len(results) == 0 {
			t.Fatal("edit returned no image")
		}
		t.Logf("edit ok: %s", strings.Join(results, " | "))
	})

	// 3) 智能超清：验证 jimeng-superres / resolution 字段
	t.Run("upscale", func(t *testing.T) {
		var results []string
		err := gen.CreateImageUpscaleRequest(&adaptercommon.ImageUpscaleProps{
			Model:          "jimeng-superres",
			Image:          makeSolidPNGBase64(t, 512, 512),
			ResolutionType: "4k",
		}, func(chunk *globals.Chunk) error {
			if chunk != nil && chunk.Content != "" {
				results = append(results, chunk.Content)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("upscale failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("upscale returned no image")
		}
		t.Logf("upscale ok: %s", strings.Join(results, " | "))
	})

	// 4) 智能扩图：验证 jimeng-outpaint / 目标比例 → 方向扩展比例
	t.Run("outpaint", func(t *testing.T) {
		var results []string
		err := gen.CreateImageOutpaintRequest(&adaptercommon.ImageOutpaintProps{
			Model:       "jimeng-outpaint",
			Image:       makeSolidPNGBase64(t, 512, 512),
			TargetRatio: "16:9",
			Prompt:      "自然延展背景，保持主体居中不变",
		}, func(chunk *globals.Chunk) error {
			if chunk != nil && chunk.Content != "" {
				results = append(results, chunk.Content)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("outpaint failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("outpaint returned no image")
		}
		t.Logf("outpaint ok: %s", strings.Join(results, " | "))
	})
}

// makeSolidPNGBase64 生成指定尺寸纯色 PNG 的原始 base64（无 data: 前缀）。
func makeSolidPNGBase64(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 80, B: 60, A: 255})
		}
	}
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
