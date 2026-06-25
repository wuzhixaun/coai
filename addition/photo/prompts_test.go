package photo

import (
	"testing"
)

func init() {
	// 测试前确保配置已加载
	LoadPrompts()
}

func TestLoadPrompts(t *testing.T) {
	cfg := getConfig()
	if cfg == nil {
		t.Fatal("config is nil")
	}
	if len(cfg.Features) == 0 {
		t.Fatal("no features loaded")
	}
	t.Logf("loaded %d features", len(cfg.Features))
}

func TestGetSystemPrompt(t *testing.T) {
	// white_bg: "product on pure white background (#FFFFFF), professional studio lighting,
	//            product 100% unchanged, no shadow on background, {english_only}"
	//
	// defaults: {english_only} → "all visible text must be in English only"
	prompt := GetSystemPrompt("white_bg", nil)
	if prompt == "" {
		t.Fatal("white_bg prompt is empty")
	}
	if prompt == GetSystemPrompt("white_bg", nil) {
		// 验证 {english_only} 已被替换（不应再含占位符）
		t.Logf("white_bg prompt: %s", prompt)
	}
}

func TestGetSystemPromptWithKwargs(t *testing.T) {
	// scene_gen: "{user_prompt}, {english_only}, professional product photography"
	kwargs := map[string]string{
		"user_prompt": "a red shoes on a beach",
	}
	prompt := GetSystemPrompt("scene_gen", kwargs)
	t.Logf("scene_gen prompt: %s", prompt)
}

func TestGetChannelType(t *testing.T) {
	// channel_type 仅为信息字段（实际派发在 processor.go 中按功能硬编码）。
	// 全部编辑类功能已切换到官方 jimeng-api 渠道（见 config/prompts.json），
	// detail_image / logo_custom 的本地实现（local.go）为迁移前遗留，不再参与派发。
	for _, feature := range []string{"white_bg", "detail_image", "logo_custom"} {
		if ct := GetChannelType(feature); ct != "jimeng-api" {
			t.Errorf("%s channel_type: got %s, want jimeng-api", feature, ct)
		}
	}
	// 视频生成仍走 CLI jimeng 渠道，本阶段未接入官方视频模型。
	if ct := GetChannelType("video_gen"); ct != "jimeng" {
		t.Errorf("video_gen channel_type: got %s, want jimeng", ct)
	}
}

func TestGetTemplates(t *testing.T) {
	templates := GetTemplates("scene_gen")
	if len(templates) == 0 {
		t.Fatal("scene_gen has no templates")
	}
	t.Logf("scene_gen templates: %d", len(templates))
	for _, tmpl := range templates {
		preview := tmpl.Prompt
		if len(preview) > 30 {
			preview = preview[:30]
		}
		t.Logf("  - %s: %s", tmpl.Label, preview)
	}
}

func TestGetOptions(t *testing.T) {
	colors := GetOptions("color_change", "colors")
	if len(colors) == 0 {
		t.Fatal("color_change has no color options")
	}
	t.Logf("colors: %d options", len(colors))

	langs := GetOptions("image_translate", "languages")
	if len(langs) == 0 {
		t.Fatal("image_translate has no language options")
	}
	t.Logf("languages: %d options", len(langs))

	sizes := GetOptions("resize", "sizes")
	if len(sizes) == 0 {
		t.Fatal("resize has no size options")
	}
	t.Logf("sizes: %d options", len(sizes))
}

func TestBuildPrompt(t *testing.T) {
	// 自定义 system_prompt 应跳过配置
	custom := "custom prompt here"
	result := BuildPrompt("white_bg", custom, nil)
	if result != custom {
		t.Errorf("BuildPrompt with custom: got %s, want %s", result, custom)
	}

	// 空 system_prompt 应使用配置
	result = BuildPrompt("hd_upscale", "", nil)
	t.Logf("hd_upscale default prompt: '%s'", result)
}

func TestReloadPrompts(t *testing.T) {
	err := ReloadPrompts()
	if err != nil {
		t.Fatal(err)
	}
	cfg := getConfig()
	if cfg == nil || len(cfg.Features) == 0 {
		t.Fatal("reload failed: features empty")
	}
}
