package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"fmt"
	"strings"
)

// normalizeInputImage 把输入图规整为 OpenAI image_url 可接受的形式：
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

// buildUserMessage 构造一条多模态 user 消息：text + 若干 image_url。
func buildUserMessage(prompt string, images []string) Message {
	p := prompt
	contents := MessageContents{{Type: "text", Text: &p}}
	for _, u := range normalizeImages(images) {
		url := u
		contents = append(contents, MessageContent{
			Type:     "image_url",
			ImageUrl: &ImageUrl{Url: url},
		})
	}
	return Message{Role: globals.User, Content: contents}
}

// extractImages 从响应文本抠出图片（http 图片 URL 与 base64 data URI）。
func extractImages(content string) []string {
	_, images := utils.ExtractImages(content, true)
	return images
}

// extractVideoURLs 从响应文本抠出视频链接（.mp4/.webm/.mov，允许带 query）。
func extractVideoURLs(content string) []string {
	var out []string
	for _, u := range utils.ExtractUrls(content) {
		clean := strings.TrimRight(u, ").,\"'")
		p := strings.ToLower(clean)
		if i := strings.IndexAny(p, "?#"); i >= 0 {
			p = p[:i]
		}
		if strings.HasSuffix(p, ".mp4") || strings.HasSuffix(p, ".webm") || strings.HasSuffix(p, ".mov") {
			out = append(out, clean)
		}
	}
	return out
}

// Generator 通用 OpenAI 兼容媒体桥：把图片/视频操作统一走多模态 chat。
type Generator struct {
	*ChatInstance
}

func newGenerator(conf globals.ChannelConfig) *Generator {
	return &Generator{ChatInstance: NewChatInstance(conf.GetEndpoint(), conf.GetRandomSecret())}
}

// NewImageGeneratorFromConfig 供生图工厂表使用。
func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory {
	return newGenerator(conf)
}

// NewImageProcessorFromConfig 供图片处理工厂表使用；同一实例实现编辑/超分/扩图/视频接口。
func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory {
	return newGenerator(conf)
}

var (
	_ adaptercommon.ImageGenerationFactory = (*Generator)(nil)
	_ adaptercommon.ImageEditFactory       = (*Generator)(nil)
	_ adaptercommon.ImageUpscaleFactory    = (*Generator)(nil)
	_ adaptercommon.ImageOutpaintFactory   = (*Generator)(nil)
	_ adaptercommon.ImageToVideoFactory    = (*Generator)(nil)
)

// runChatMedia 发一次多模态 /v1/chat/completions，从响应里抠出图/视频并回推。
func (g *Generator) runChatMedia(model, prompt string, images []string, wantVideo bool, proxy globals.ProxyConfig, hook globals.Hook) error {
	body := ChatRequest{
		Model:    model,
		Messages: []Message{buildUserMessage(prompt, images)},
		Stream:   false,
	}
	res, err := utils.Post(
		fmt.Sprintf("%s/v1/chat/completions", g.GetEndpoint()),
		g.GetHeader(), body, proxy,
	)
	if err != nil || res == nil {
		if err != nil {
			return fmt.Errorf("openai media request failed: %s", err.Error())
		}
		return fmt.Errorf("openai media request failed: empty response")
	}

	data := utils.MapToStruct[ChatResponse](res)
	if data == nil {
		return fmt.Errorf("openai media error: cannot parse response")
	}
	if data.Error.Message != "" {
		return fmt.Errorf("%s", hideRequestId(data.Error.Message))
	}
	if len(data.Choices) == 0 {
		return fmt.Errorf("openai media error: empty choices")
	}
	content := data.Choices[0].Message.Content

	if wantVideo {
		urls := extractVideoURLs(content)
		if len(urls) == 0 {
			return fmt.Errorf("未从响应中解析到视频结果")
		}
		for _, u := range urls {
			if err := hook(&globals.Chunk{Content: utils.StoreImage(u)}); err != nil {
				return err
			}
		}
		return nil
	}

	imgs := extractImages(content)
	if len(imgs) == 0 {
		return fmt.Errorf("未从响应中解析到图片结果")
	}
	for _, img := range imgs {
		var markdown string
		if strings.HasPrefix(img, "data:") {
			markdown = utils.GetImageMarkdown(img) // data: URI 直接内联
		} else {
			markdown = utils.GetImageMarkdown(utils.StoreImage(img)) // http 图落地后再回推
		}
		if err := hook(&globals.Chunk{Content: markdown}); err != nil {
			return err
		}
	}
	return nil
}

// CreateImageGenerationRequest 文生图（参考图可空）。
func (g *Generator) CreateImageGenerationRequest(props *adaptercommon.ImageGenerationProps, hook globals.Hook) error {
	return g.runChatMedia(props.Model, strings.TrimSpace(props.Prompt), props.Images, false, props.Proxy, hook)
}

// CreateImageEditRequest 图生图 / 换色 / 场景 / 擦除（需输入图）。
func (g *Generator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	if len(normalizeImages(props.Images)) == 0 {
		return fmt.Errorf("图片编辑需要至少 1 张输入图")
	}
	return g.runChatMedia(props.Model, strings.TrimSpace(props.Prompt), props.Images, false, props.Proxy, hook)
}

// CreateImageUpscaleRequest 超分（chat 尽力而为）。
func (g *Generator) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	prompt := fmt.Sprintf("将这张图片高清放大到 %s 分辨率，保持画面内容与构图不变，仅提升清晰度与细节。", props.ResolutionType)
	return g.runChatMedia(props.Model, prompt, []string{props.Image}, false, props.Proxy, hook)
}

// CreateImageOutpaintRequest 扩图（chat 尽力而为）。
func (g *Generator) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	prompt := strings.TrimSpace(fmt.Sprintf("将这张图片的画布扩展为 %s 比例，自然延展周围背景，保持主体不变。%s", props.TargetRatio, props.Prompt))
	return g.runChatMedia(props.Model, prompt, []string{props.Image}, false, props.Proxy, hook)
}

// CreateImageToVideoRequest 图/文生视频。
func (g *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	return g.runChatMedia(props.Model, strings.TrimSpace(props.Prompt), props.Images, true, props.Proxy, hook)
}
