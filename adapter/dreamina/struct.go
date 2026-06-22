package dreamina

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
)

// ImageProcessor 即梦图片处理适配器
// 实现 adaptercommon.ImageEditFactory, ImageUpscaleFactory, ImageOutpaintFactory, ImageToVideoFactory
type ImageProcessor struct {
	instance globals.ChannelConfig
}

func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory {
	inst := &ImageProcessor{instance: conf}
	return inst
}

// ── 渠道配置方法 ──────────────────────────────────────────────

func (c *ImageProcessor) GetEndpoint() string {
	return c.instance.GetEndpoint()
}

func (c *ImageProcessor) GetSecret() string {
	return c.instance.GetRandomSecret()
}

func (c *ImageProcessor) GetType() string {
	return c.instance.GetType()
}

func (c *ImageProcessor) GetProxy() globals.ProxyConfig {
	return c.instance.GetProxy()
}

func (c *ImageProcessor) GetRetry() int {
	return c.instance.GetRetry()
}

// ── 请求头 ────────────────────────────────────────────────────

func (c *ImageProcessor) GetHeader() map[string]string {
	secret := c.GetSecret()
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": fmt.Sprintf("Bearer %s", secret),
	}
}
