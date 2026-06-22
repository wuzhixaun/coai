package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"strings"
)

type ImageGenerator struct {
	instance  globals.ChannelConfig
	endpoint  string
	accessKey string
	secretKey string
}

func newImageGenerator(conf globals.ChannelConfig) *ImageGenerator {
	secret := conf.SplitRandomSecret(2)
	endpoint := strings.TrimSpace(conf.GetEndpoint())
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	return &ImageGenerator{
		instance:  conf,
		endpoint:  strings.TrimRight(endpoint, "/"),
		accessKey: strings.TrimSpace(secret[0]),
		secretKey: strings.TrimSpace(secret[1]),
	}
}

func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory {
	return newImageGenerator(conf)
}

// NewImageProcessorFromConfig 让官方 jimeng-api 渠道同时服务 Photo 页面的
// 图片编辑 / 超清 / 扩图能力。返回的实例同时实现了多个图片处理接口，
// adapter 层会按需断言。
func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory {
	return newImageGenerator(conf)
}

func (c *ImageGenerator) GetProxy() globals.ProxyConfig {
	return c.instance.GetProxy()
}

var (
	_ adaptercommon.ImageGenerationFactory = (*ImageGenerator)(nil)
	_ adaptercommon.ImageEditFactory       = (*ImageGenerator)(nil)
	_ adaptercommon.ImageUpscaleFactory    = (*ImageGenerator)(nil)
	_ adaptercommon.ImageOutpaintFactory   = (*ImageGenerator)(nil)
)
