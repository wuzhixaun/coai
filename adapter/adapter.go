package adapter

import (
	"chat/adapter/azure"
	"chat/adapter/baichuan"
	"chat/adapter/bing"
	"chat/adapter/claude"
	adaptercommon "chat/adapter/common"
	"chat/adapter/coze"
	"chat/adapter/dashscope"
	"chat/adapter/deepseek"
	"chat/adapter/dify"
	"chat/adapter/grsai"
	"chat/adapter/hunyuan"
	"chat/adapter/jimeng"
	"chat/adapter/jimengapi"
	"chat/adapter/midjourney"
	"chat/adapter/openai"
	"chat/adapter/palm2"
	"chat/adapter/skylark"
	"chat/adapter/slack"
	"chat/adapter/sparkdesk"
	"chat/adapter/zhinao"
	"chat/adapter/zhipuai"
	"chat/globals"
	"fmt"
)

var channelFactories = map[string]adaptercommon.FactoryCreator{
	globals.OpenAIChannelType:      openai.NewChatInstanceFromConfig,
	globals.AzureOpenAIChannelType: azure.NewChatInstanceFromConfig,
	globals.ClaudeChannelType:      claude.NewChatInstanceFromConfig,
	globals.SlackChannelType:       slack.NewChatInstanceFromConfig,
	globals.BingChannelType:        bing.NewChatInstanceFromConfig,
	globals.PalmChannelType:        palm2.NewChatInstanceFromConfig,
	globals.SparkdeskChannelType:   sparkdesk.NewChatInstanceFromConfig,
	globals.ChatGLMChannelType:     zhipuai.NewChatInstanceFromConfig,
	globals.QwenChannelType:        dashscope.NewChatInstanceFromConfig,
	globals.HunyuanChannelType:     hunyuan.NewChatInstanceFromConfig,
	globals.BaichuanChannelType:    baichuan.NewChatInstanceFromConfig,
	globals.SkylarkChannelType:     skylark.NewChatInstanceFromConfig,
	globals.ZhinaoChannelType:      zhinao.NewChatInstanceFromConfig,
	globals.MidjourneyChannelType:  midjourney.NewChatInstanceFromConfig,
	globals.DeepseekChannelType:    deepseek.NewChatInstanceFromConfig,
	globals.DifyChannelType:        dify.NewChatInstanceFromConfig,
	globals.CozeChannelType:        coze.NewChatInstanceFromConfig,

	globals.MoonshotChannelType: openai.NewChatInstanceFromConfig, // openai format
	globals.GroqChannelType:     openai.NewChatInstanceFromConfig, // openai format

	globals.GrsaiChannelType: grsai.NewChatInstanceFromConfig, // 对话框内直接出图/出视频
}

// 图片处理适配器工厂映射
var imageProcessorFactories = map[string]adaptercommon.ImageEditFactoryCreator{
	globals.JimengChannelType:    jimeng.NewCLIAdapterFromConfig, // CLI，仅 video_gen 仍在使用
	globals.JimengAPIChannelType: jimengapi.NewImageProcessorFromConfig,
	globals.GrsaiChannelType:     grsai.NewImageProcessorFromConfig,
	globals.OpenAIChannelType:    openai.NewImageProcessorFromConfig,
}

var imageGenerationFactories = map[string]adaptercommon.ImageGenerationFactoryCreator{
	globals.JimengAPIChannelType: jimengapi.NewImageGeneratorFromConfig,
	globals.GrsaiChannelType:     grsai.NewImageGeneratorFromConfig,
	globals.OpenAIChannelType:    openai.NewImageGeneratorFromConfig,
}

func createImageGenerationRequest(conf globals.ChannelConfig, props *adaptercommon.ImageGenerationProps, hook globals.Hook) error {
	props.Model = conf.GetModelReflect(props.OriginalModel)
	props.Proxy = conf.GetProxy()

	factoryType := conf.GetType()
	if creator, ok := imageGenerationFactories[factoryType]; ok {
		return creator(conf).CreateImageGenerationRequest(props, hook)
	}

	return fmt.Errorf("该模型不支持图片生成 (channel type: %s)。请选择 jimeng-seedream-4.6 等图片生成模型", conf.GetType())
}

func createImageEditRequest(conf globals.ChannelConfig, props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	props.Model = conf.GetModelReflect(props.OriginalModel)
	props.Proxy = conf.GetProxy()

	factoryType := conf.GetType()
	if creator, ok := imageProcessorFactories[factoryType]; ok {
		return creator(conf).CreateImageEditRequest(props, hook)
	}

	return fmt.Errorf("该模型不支持图片编辑 (channel type: %s)。请选择 jimeng-v2 等图片专用模型", conf.GetType())
}

func createImageUpscaleRequest(conf globals.ChannelConfig, props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	props.Model = conf.GetModelReflect(props.OriginalModel)
	props.Proxy = conf.GetProxy()

	if creator, ok := imageProcessorFactories[conf.GetType()]; ok {
		if inst, ok := creator(conf).(adaptercommon.ImageUpscaleFactory); ok {
			return inst.CreateImageUpscaleRequest(props, hook)
		}
	}
	return fmt.Errorf("该模型不支持高清放大，请选择 jimeng-v2")
}

func createImageOutpaintRequest(conf globals.ChannelConfig, props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	props.Model = conf.GetModelReflect(props.OriginalModel)
	props.Proxy = conf.GetProxy()

	if creator, ok := imageProcessorFactories[conf.GetType()]; ok {
		if inst, ok := creator(conf).(adaptercommon.ImageOutpaintFactory); ok {
			return inst.CreateImageOutpaintRequest(props, hook)
		}
	}
	return fmt.Errorf("该模型不支持画布扩展，请选择 jimeng-v2")
}

func createImageToVideoRequest(conf globals.ChannelConfig, props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	props.Model = conf.GetModelReflect(props.OriginalModel)
	props.Proxy = conf.GetProxy()

	factoryType := conf.GetType()
	if creator, ok := imageProcessorFactories[factoryType]; ok {
		inst := creator(conf)
		if v, ok := inst.(adaptercommon.ImageToVideoFactory); ok {
			return v.CreateImageToVideoRequest(props, hook)
		}
		return fmt.Errorf("video not supported by channel type %s", conf.GetType())
	}
	return fmt.Errorf("unknown channel type %s", conf.GetType())
}

func createChatRequest(conf globals.ChannelConfig, props *adaptercommon.ChatProps, hook globals.Hook) error {
	props.Model = conf.GetModelReflect(props.OriginalModel)
	props.Proxy = conf.GetProxy()

	factoryType := conf.GetType()
	if factory, ok := channelFactories[factoryType]; ok {
		return factory(conf).CreateStreamChatRequest(props, hook)
	}

	return fmt.Errorf("unknown channel type %s (channel #%d)", conf.GetType(), conf.GetId())
}

func createVideoRequest(conf globals.ChannelConfig, props *adaptercommon.VideoProps, hook globals.Hook) error {
	props.Model = conf.GetModelReflect(props.OriginalModel)
	props.Proxy = conf.GetProxy()

	factoryType := conf.GetType()
	if creator, ok := channelFactories[factoryType]; ok {
		inst := creator(conf)
		if v, ok := inst.(adaptercommon.VideoFactory); ok {
			return v.CreateVideoRequest(props, hook)
		}
		return fmt.Errorf("video request not supported by channel type %s (channel #%d)", conf.GetType(), conf.GetId())
	}

	return fmt.Errorf("unknown channel type %s (channel #%d)", conf.GetType(), conf.GetId())
}
