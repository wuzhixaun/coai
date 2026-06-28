package adapter

import (
	"chat/globals"
	"testing"
)

func TestOpenAIRegisteredForImagePipelines(t *testing.T) {
	if imageGenerationFactories[globals.OpenAIChannelType] == nil {
		t.Fatal("openai 未注册到 imageGenerationFactories")
	}
	if imageProcessorFactories[globals.OpenAIChannelType] == nil {
		t.Fatal("openai 未注册到 imageProcessorFactories")
	}
}

func TestArkRegisteredForAllPipelines(t *testing.T) {
	if channelFactories[globals.ArkChannelType] == nil {
		t.Fatal("ark 未注册到 channelFactories（对话）")
	}
	if imageGenerationFactories[globals.ArkChannelType] == nil {
		t.Fatal("ark 未注册到 imageGenerationFactories（生图）")
	}
	if imageProcessorFactories[globals.ArkChannelType] == nil {
		t.Fatal("ark 未注册到 imageProcessorFactories（图片处理/图生视频）")
	}
}
