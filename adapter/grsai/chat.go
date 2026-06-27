package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"fmt"
	"regexp"
	"strings"
)

// markdownImagePattern 匹配整段 markdown 图片语法 ![...](...)，用于从提示词中剔除参考图。
var markdownImagePattern = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

// NewChatInstanceFromConfig 让 grsai 渠道可作为「聊天模型」在对话框里直接出图/出视频：
// 取最后一条用户消息作为 prompt（消息里的图片 URL 作为参考图），按模型能力路由到生成方法。
func NewChatInstanceFromConfig(conf globals.ChannelConfig) adaptercommon.Factory {
	return newGenerator(conf)
}

// latestPromptAndImages 从最后一条用户消息提取提示词与参考图（markdown 图片 / base64）。
func (c *Generator) latestPromptAndImages(props *adaptercommon.ChatProps) (string, []string) {
	if len(props.Message) == 0 {
		return "", nil
	}
	msg := props.Message[len(props.Message)-1]
	for i := len(props.Message) - 1; i >= 0; i-- {
		if props.Message[i].Role == globals.User {
			msg = props.Message[i]
			break
		}
	}
	_, urls := utils.ExtractImages(msg.Content, true)
	// 从提示词中剔除整段 markdown 图片语法，避免把 ![image](...) 残留传给模型。
	text := markdownImagePattern.ReplaceAllString(msg.Content, "")
	return strings.TrimSpace(text), urls
}

// CreateStreamChatRequest 把聊天请求当作一次生成任务：视频模型出视频，带参考图出图生图，否则文生图。
// 结果（图片 Markdown / 视频 URL）经 hook 流式回推。
func (c *Generator) CreateStreamChatRequest(props *adaptercommon.ChatProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok {
		return fmt.Errorf("grsai 不支持的模型: %s", props.Model)
	}
	prompt, images := c.latestPromptAndImages(props)
	if prompt == "" && len(images) == 0 {
		return fmt.Errorf("请提供生成提示词")
	}

	if spec.Capability == CapabilityVideo {
		return c.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
			Model:  props.Model,
			Images: images,
			Prompt: prompt,
		}, hook)
	}

	if len(images) > 0 {
		return c.CreateImageEditRequest(&adaptercommon.ImageEditProps{
			Model:  props.Model,
			Images: images,
			Prompt: prompt,
		}, hook)
	}

	return c.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
		Model:  props.Model,
		Prompt: prompt,
	}, hook)
}
