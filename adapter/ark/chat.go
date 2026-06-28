package ark

import (
	adaptercommon "chat/adapter/common"
	"chat/adapter/skylark"
	"chat/globals"
	"chat/utils"
	"fmt"
	"regexp"
	"strings"
)

// markdownImagePattern 匹配整段 markdown 图片语法 ![...](...)，用于从提示词中剔除参考图。
var markdownImagePattern = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

// ChatInstance 火山方舟对话实例：按所选模型能力分流，让对话框也能「直接出图/出视频」。
//   - seedance（视频）→ 图/文生视频
//   - seedream（图片）→ 有参考图出图生图，否则文生图
//   - 其它（doubao-seed* 等对话模型）→ 真对话（skylark / arkruntime）
type ChatInstance struct {
	*Generator
	chat adaptercommon.Factory
}

func NewChatInstanceFromConfig(conf globals.ChannelConfig) adaptercommon.Factory {
	return &ChatInstance{
		Generator: newGenerator(conf),
		chat:      skylark.NewChatInstanceFromConfig(conf),
	}
}

// isVideoModel / isImageModel 按方舟模型命名判定能力。
func isVideoModel(model string) bool { return strings.Contains(model, "seedance") }
func isImageModel(model string) bool { return strings.Contains(model, "seedream") }

// latestPromptAndImages 从最后一条用户消息提取提示词与参考图（markdown 图片 / base64）。
func latestPromptAndImages(props *adaptercommon.ChatProps) (string, []string) {
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
	text := markdownImagePattern.ReplaceAllString(msg.Content, "")
	return strings.TrimSpace(text), urls
}

// CreateStreamChatRequest 按模型分流：视频/图片模型当作一次生成任务，其余走真对话。
func (c *ChatInstance) CreateStreamChatRequest(props *adaptercommon.ChatProps, hook globals.Hook) error {
	model := props.Model

	if isVideoModel(model) {
		prompt, images := latestPromptAndImages(props)
		if prompt == "" && len(images) == 0 {
			return fmt.Errorf("请提供提示词或参考图")
		}
		return c.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
			Model: model, Prompt: prompt, Images: images,
		}, hook)
	}

	if isImageModel(model) {
		prompt, images := latestPromptAndImages(props)
		if prompt == "" && len(images) == 0 {
			return fmt.Errorf("请提供生成提示词")
		}
		if len(images) > 0 {
			return c.CreateImageEditRequest(&adaptercommon.ImageEditProps{
				Model: model, Prompt: prompt, Images: images,
			}, hook)
		}
		return c.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
			Model: model, Prompt: prompt,
		}, hook)
	}

	// 普通对话模型：交给 skylark（arkruntime SDK）做真对话。
	return c.chat.CreateStreamChatRequest(props, hook)
}
