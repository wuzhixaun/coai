package ark

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"net/http"
	"strings"
	"time"
)

// defaultEndpoint 火山方舟 Ark 北京区基础地址（既是 SDK base url，也是图片接口前缀）。
const defaultEndpoint = "https://ark.cn-beijing.volces.com/api/v3"

// Generator 火山方舟图片/视频生成器。
// 对话(/chat/completions) 复用 skylark（arkruntime SDK），不在此实现；
// 这里负责图片生成/编辑/超清/扩图(/images/generations) 与 图生视频(/contents/generations/tasks)。
type Generator struct {
	instance globals.ChannelConfig
	endpoint string
	apiKey   string
}

func newGenerator(conf globals.ChannelConfig) *Generator {
	endpoint := strings.TrimRight(strings.TrimSpace(conf.GetEndpoint()), "/")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Generator{
		instance: conf,
		endpoint: endpoint,
		apiKey:   strings.TrimSpace(conf.GetRandomSecret()),
	}
}

// NewImageGeneratorFromConfig 供生图工厂表使用（文生图/图生图）。
func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory {
	return newGenerator(conf)
}

// NewImageProcessorFromConfig 供图片处理工厂表使用；同一实例实现编辑/超清/扩图/图生视频接口。
func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory {
	return newGenerator(conf)
}

func (g *Generator) GetProxy() globals.ProxyConfig {
	return g.instance.GetProxy()
}

// header 方舟接口统一用 Bearer API Key 鉴权。
func (g *Generator) header() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + g.apiKey,
	}
}

func (g *Generator) httpClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute}
}

var (
	_ adaptercommon.ImageGenerationFactory = (*Generator)(nil)
	_ adaptercommon.ImageEditFactory       = (*Generator)(nil)
	_ adaptercommon.ImageUpscaleFactory    = (*Generator)(nil)
	_ adaptercommon.ImageOutpaintFactory   = (*Generator)(nil)
	_ adaptercommon.ImageToVideoFactory    = (*Generator)(nil)
)
