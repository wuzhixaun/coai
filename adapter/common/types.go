package adaptercommon

import (
	"chat/globals"
	"chat/utils"
)

type RequestProps struct {
	MaxRetries *int                `json:"-"`
	Current    int                 `json:"-"`
	Group      string              `json:"-"`
	Proxy      globals.ProxyConfig `json:"-"`
}

type VideoProps struct {
	RequestProps

	Model         string `json:"model,omitempty"`
	OriginalModel string `json:"-"`

	Prompt         string  `json:"prompt"`
	Seconds        *string `json:"seconds,omitempty"`
	Size           *string `json:"size,omitempty"`
	InputReference *string `json:"input_reference,omitempty"`

	User string `json:"-"`
}

type ChatProps struct {
	RequestProps

	Model         string `json:"model,omitempty"`
	OriginalModel string `json:"-"`

	Message           []globals.Message      `json:"messages,omitempty"`
	MaxTokens         *int                   `json:"max_tokens,omitempty"`
	PresencePenalty   *float32               `json:"presence_penalty,omitempty"`
	FrequencyPenalty  *float32               `json:"frequency_penalty,omitempty"`
	RepetitionPenalty *float32               `json:"repetition_penalty,omitempty"`
	Temperature       *float32               `json:"temperature,omitempty"`
	TopP              *float32               `json:"top_p,omitempty"`
	TopK              *int                   `json:"top_k,omitempty"`
	Tools             *globals.FunctionTools `json:"tools,omitempty"`
	ToolChoice        *interface{}           `json:"tool_choice,omitempty"`
	Buffer            *utils.Buffer          `json:"-"`
}

func (c *ChatProps) SetupBuffer(buf *utils.Buffer) {
	buf.SetPrompts(c)
	c.Buffer = buf
}

func CreateChatProps(props *ChatProps, buffer *utils.Buffer) *ChatProps {
	props.SetupBuffer(buffer)
	return props
}

func CreateVideoProps(props *VideoProps) *VideoProps {
	return props
}

// ── 图片编辑请求属性 ──────────────────────────────────────

type ImageEditProps struct {
	RequestProps

	Model         string   `json:"model"`
	OriginalModel string   `json:"-"`
	Images        []string `json:"images"` // base64 编码的图片列表
	Prompt        string   `json:"prompt"`
	Strength      *float32 `json:"strength,omitempty"`
	User          string   `json:"-"`
}

func CreateImageEditProps(props *ImageEditProps) *ImageEditProps {
	return props
}

type ImageGenerationProps struct {
	RequestProps

	Model         string   `json:"model"`
	OriginalModel string   `json:"-"`
	Prompt        string   `json:"prompt"`
	Images        []string `json:"images,omitempty"` // 官方 Jimeng 4.x 使用 URL 图片列表，文生图可为空
	Masks         []string `json:"masks,omitempty"`  // 预留给 inpaint；生成模型不接受 mask
	N             int      `json:"n,omitempty"`
	Width         *int     `json:"width,omitempty"`
	Height        *int     `json:"height,omitempty"`
	Size          *int     `json:"size,omitempty"`
	MinRatio      *float64 `json:"min_ratio,omitempty"`
	MaxRatio      *float64 `json:"max_ratio,omitempty"`
	ForceSingle   *bool    `json:"force_single,omitempty"`
	Seed          *int     `json:"seed,omitempty"`
	Scale         *float64 `json:"scale,omitempty"`
	ReturnURL     bool     `json:"return_url,omitempty"`
	User          string   `json:"-"`
}

func CreateImageGenerationProps(props *ImageGenerationProps) *ImageGenerationProps {
	return props
}

type ImageUpscaleProps struct {
	RequestProps

	Model          string `json:"model"`
	OriginalModel  string `json:"-"`
	Image          string `json:"image"`           // base64
	ResolutionType string `json:"resolution_type"` // 2k, 4k, 8k
	User           string `json:"-"`
}

func CreateImageUpscaleProps(props *ImageUpscaleProps) *ImageUpscaleProps {
	return props
}

type ImageOutpaintProps struct {
	RequestProps

	Model         string `json:"model"`
	OriginalModel string `json:"-"`
	Image         string `json:"image"`        // base64
	TargetRatio   string `json:"target_ratio"` // 1:1, 16:9, 4:3...
	Prompt        string `json:"prompt"`
	User          string `json:"-"`
}

func CreateImageOutpaintProps(props *ImageOutpaintProps) *ImageOutpaintProps {
	return props
}

type ImageToVideoProps struct {
	RequestProps

	Model         string   `json:"model"`
	OriginalModel string   `json:"-"`
	Images        []string `json:"images"` // 1-9 张参考图 base64
	Prompt        string   `json:"prompt"` // 可选
	Duration      int      `json:"duration,omitempty"`
	User          string   `json:"-"`
}

func CreateImageToVideoProps(props *ImageToVideoProps) *ImageToVideoProps {
	return props
}
