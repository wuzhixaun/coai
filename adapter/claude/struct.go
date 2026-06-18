package claude

import (
	factory "chat/adapter/common"
	"chat/globals"
)

type ChatInstance struct {
	Endpoint        string
	ApiKey          string
	isFirstThinking bool
	isThinkingOver  bool
}

func NewChatInstance(endpoint, apiKey string) *ChatInstance {
	return &ChatInstance{
		Endpoint:        endpoint,
		ApiKey:          apiKey,
		isFirstThinking: true,
	}
}

func NewChatInstanceFromConfig(conf globals.ChannelConfig) factory.Factory {
	return NewChatInstance(
		conf.GetEndpoint(),
		conf.GetRandomSecret(),
	)
}

func (c *ChatInstance) GetEndpoint() string {
	return c.Endpoint
}

func (c *ChatInstance) GetApiKey() string {
	return c.ApiKey
}
