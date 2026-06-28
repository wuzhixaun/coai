package ark

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"fmt"
	"strings"
)

// normalizeInputImage 把输入图规整为方舟 image 可接受的形式：
// http(s) 与 data: 原样返回；裸 base64 补 data:image/png;base64, 前缀；空白返回 ""。
func normalizeInputImage(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "data:") {
		return s
	}
	return "data:image/png;base64," + s
}

// normalizeImages 规整并丢弃空白项。
func normalizeImages(images []string) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		if n := normalizeInputImage(img); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// imageRequest 对应 POST /images/generations（Seedream）。
type imageRequest struct {
	Model                     string   `json:"model"`
	Prompt                    string   `json:"prompt"`
	Image                     []string `json:"image,omitempty"` // 图生图参考图（url 或 data: base64）
	SequentialImageGeneration string   `json:"sequential_image_generation,omitempty"`
	ResponseFormat            string   `json:"response_format,omitempty"`
	Size                      string   `json:"size,omitempty"`
	Watermark                 *bool    `json:"watermark,omitempty"`
}

type imageResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// runImage 发一次 /images/generations，把返回图片回推。
// 失败或无图片时在任何 hook 调用前返回错误，保证不空跑扣费。
func (g *Generator) runImage(modelID, prompt string, images []string, size string, hook globals.Hook) error {
	watermark := false
	body := imageRequest{
		Model:                     modelID,
		Prompt:                    prompt,
		SequentialImageGeneration: "disabled",
		ResponseFormat:            "url",
		Watermark:                 &watermark,
	}
	if size != "" {
		body.Size = size
	}
	if imgs := normalizeImages(images); len(imgs) > 0 {
		body.Image = imgs
	}

	res, err := utils.Post(g.endpoint+"/images/generations", g.header(), body, g.GetProxy())
	if err != nil || res == nil {
		if err != nil {
			return fmt.Errorf("ark image request failed: %s", err.Error())
		}
		return fmt.Errorf("ark image request failed: empty response")
	}

	data := utils.MapToStruct[imageResponse](res)
	if data == nil {
		return fmt.Errorf("ark image error: cannot parse response")
	}
	if data.Error != nil && strings.TrimSpace(data.Error.Message) != "" {
		return fmt.Errorf("%s", data.Error.Message)
	}
	if len(data.Data) == 0 {
		return fmt.Errorf("ark 未返回图片结果")
	}

	pushed := 0
	for _, d := range data.Data {
		var markdown string
		switch {
		case strings.TrimSpace(d.URL) != "":
			markdown = utils.GetImageMarkdown(utils.StoreImage(d.URL)) // http 图落地后回推
		case strings.TrimSpace(d.B64JSON) != "":
			markdown = utils.GetImageMarkdown("data:image/png;base64," + d.B64JSON) // base64 直接内联
		default:
			continue
		}
		if err := hook(&globals.Chunk{Content: markdown}); err != nil {
			return err
		}
		pushed++
	}
	if pushed == 0 {
		return fmt.Errorf("ark 未返回有效图片结果")
	}
	return nil
}

// CreateImageGenerationRequest 文生图（参考图可空）。
func (g *Generator) CreateImageGenerationRequest(props *adaptercommon.ImageGenerationProps, hook globals.Hook) error {
	return g.runImage(props.Model, strings.TrimSpace(props.Prompt), props.Images, "", hook)
}

// CreateImageEditRequest 图生图 / 换色 / 场景 / 擦除（需输入图）。
func (g *Generator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	if len(normalizeImages(props.Images)) == 0 {
		return fmt.Errorf("图片编辑需要至少 1 张输入图")
	}
	return g.runImage(props.Model, strings.TrimSpace(props.Prompt), props.Images, "", hook)
}

// CreateImageUpscaleRequest 高清放大（Seedream 以更大 size + 提示词尽力而为）。
func (g *Generator) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	prompt := fmt.Sprintf("将这张图片高清放大到 %s 分辨率，保持画面内容与构图不变，仅提升清晰度与细节。", props.ResolutionType)
	size := ""
	switch strings.ToLower(strings.TrimSpace(props.ResolutionType)) {
	case "2k":
		size = "2K"
	case "4k":
		size = "4K"
	}
	return g.runImage(props.Model, prompt, []string{props.Image}, size, hook)
}

// CreateImageOutpaintRequest 画布扩展/改尺寸（提示词尽力而为）。
func (g *Generator) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	prompt := strings.TrimSpace(fmt.Sprintf("将这张图片的画布扩展为 %s 比例，自然延展周围背景，保持主体不变。%s", props.TargetRatio, props.Prompt))
	return g.runImage(props.Model, prompt, []string{props.Image}, "", hook)
}
