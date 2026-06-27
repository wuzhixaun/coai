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
