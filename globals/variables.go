package globals

import (
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const ChatMaxThread = 5
const AnonymousMaxThread = 1

var HttpMaxTimeout = 30 * time.Minute

var AllowedOrigins []string

var DebugMode bool
var NotifyUrl = ""
var ArticlePermissionGroup []string
var GenerationPermissionGroup []string
var CacheAcceptedModels []string
var CacheAcceptedExpire int64
var CacheAcceptedSize int64
var AcceptImageStore bool
var AcceptPromptStore bool
var CloseRegistration bool
var CloseRelay bool

var SearchEndpoint string
var SearchCrop bool
var SearchCropLength int
var SearchEngines string    // e.g. "google,bing"
var SearchImageProxy string // e.g. "True", "False"
var SearchSafeSearch int    // e.g. 0: None, 1: Moderation, 2: Strict

func OriginIsAllowed(uri string) bool {
	if len(AllowedOrigins) == 0 {
		// if allowed origins is empty, allow all origins
		return true
	}

	instance, _ := url.Parse(uri)
	if instance == nil {
		return false
	}

	if instance.Hostname() == "localhost" || instance.Scheme == "file" {
		return true
	}

	if strings.HasPrefix(instance.Host, "www.") {
		instance.Host = instance.Host[4:]
	}

	return in(instance.Host, AllowedOrigins)
}

func OriginIsOpen(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/v1") || strings.HasPrefix(c.Request.URL.Path, "/dashboard") || strings.HasPrefix(c.Request.URL.Path, "/mj")
}

const (
	GPT3Turbo                    = "gpt-3.5-turbo"
	GPT3TurboInstruct            = "gpt-3.5-turbo-instruct"
	GPT3Turbo0613                = "gpt-3.5-turbo-0613"
	GPT3Turbo0301                = "gpt-3.5-turbo-0301"
	GPT3Turbo1106                = "gpt-3.5-turbo-1106"
	GPT3Turbo0125                = "gpt-3.5-turbo-0125"
	GPT3Turbo16k                 = "gpt-3.5-turbo-16k"
	GPT3Turbo16k0613             = "gpt-3.5-turbo-16k-0613"
	GPT3Turbo16k0301             = "gpt-3.5-turbo-16k-0301"
	GPT4                         = "gpt-4"
	GPT4All                      = "gpt-4-all"
	GPT4Vision                   = "gpt-4-v"
	GPT4Dalle                    = "gpt-4-dalle"
	GPT40314                     = "gpt-4-0314"
	GPT40613                     = "gpt-4-0613"
	GPT41106Preview              = "gpt-4-1106-preview"
	GPT40125Preview              = "gpt-4-0125-preview"
	GPT4TurboPreview             = "gpt-4-turbo-preview"
	GPT4VisionPreview            = "gpt-4-vision-preview"
	GPT4Turbo                    = "gpt-4-turbo"
	GPT4Turbo20240409            = "gpt-4-turbo-2024-04-09"
	GPT41106VisionPreview        = "gpt-4-1106-vision-preview"
	GPT432k                      = "gpt-4-32k"
	GPT432k0314                  = "gpt-4-32k-0314"
	GPT432k0613                  = "gpt-4-32k-0613"
	GPT4O                        = "gpt-4o"
	GPT4O20240513                = "gpt-4o-2024-05-13"
	GPTImage1                    = "gpt-image-1"
	Sora2                        = "sora-2"
	Dalle                        = "dalle"
	Dalle2                       = "dall-e-2"
	Dalle3                       = "dall-e-3"
	Claude1                      = "claude-1"
	Claude1100k                  = "claude-1.3"
	Claude2                      = "claude-1-100k"
	Claude2100k                  = "claude-2"
	Claude2200k                  = "claude-2.1"
	Claude3                      = "claude-3"
	ClaudeSlack                  = "claude-slack"
	SparkDeskLite                = "spark-desk-lite"
	SparkDeskPro                 = "spark-desk-pro"
	SparkDeskPro128K             = "spark-desk-pro-128k"
	SparkDeskMax                 = "spark-desk-max"
	SparkDeskMax32K              = "spark-desk-max-32k"
	SparkDeskV4Ultra             = "spark-desk-4.0-ultra"
	ChatBison001                 = "chat-bison-001"
	GeminiPro                    = "gemini-pro"
	GeminiProVision              = "gemini-pro-vision"
	Gemini15ProLatest            = "gemini-1.5-pro-latest"
	Gemini15FlashLatest          = "gemini-1.5-flash-latest"
	Gemini20ProExp               = "gemini-2.0-pro-exp-02-05"
	Gemini20Flash                = "gemini-2.0-flash"
	Gemini20FlashExp             = "gemini-2.0-flash-exp"
	Gemini20Flash001             = "gemini-2.0-flash-001"
	Gemini20FlashThinkingExp     = "gemini-2.0-flash-thinking-exp-01-21"
	Gemini20FlashLitePreview     = "gemini-2.0-flash-lite-preview-02-05"
	Gemini20FlashThinkingExp1219 = "gemini-2.0-flash-thinking-exp-1219"
	GeminiExp1206                = "gemini-exp-1206"
	GoogleImagen002              = "imagen-3.0-generate-002"
	BingCreative                 = "bing-creative"
	BingBalanced                 = "bing-balanced"
	BingPrecise                  = "bing-precise"
	ZhiPuChatGLM4                = "glm-4"
	ZhiPuChatGLM4Vision          = "glm-4v"
	ZhiPuChatGLM3Turbo           = "glm-3-turbo"
	ZhiPuChatGLMTurbo            = "zhipu-chatglm-turbo"
	ZhiPuChatGLMPro              = "zhipu-chatglm-pro"
	ZhiPuChatGLMStd              = "zhipu-chatglm-std"
	ZhiPuChatGLMLite             = "zhipu-chatglm-lite"
	QwenTurbo                    = "qwen-turbo"
	QwenPlus                     = "qwen-plus"
	QwenTurboNet                 = "qwen-turbo-net"
	QwenPlusNet                  = "qwen-plus-net"
	Midjourney                   = "midjourney"
	MidjourneyFast               = "midjourney-fast"
	MidjourneyTurbo              = "midjourney-turbo"
	Hunyuan                      = "hunyuan"
	GPT360V9                     = "360-gpt-v9"
	Baichuan53B                  = "baichuan-53b"
	SkylarkLite                  = "skylark-lite-public"
	SkylarkPlus                  = "skylark-plus-public"
	SkylarkPro                   = "skylark-pro-public"
	SkylarkChat                  = "skylark-chat"
	DeepseekV3                   = "deepseek-chat"
	DeepseekR1                   = "deepseek-reasoner"
	JimengSeedream46             = "jimeng-seedream-4.6"
	JimengSeedream40             = "jimeng-seedream-4.0"
	JimengSuperres               = "jimeng-superres"
	JimengOutpaint               = "jimeng-outpaint"
	JimengInpaint                = "jimeng-inpaint"
	JimengMaterialExtract        = "jimeng-material-extract"
	JimengProductExtract         = "jimeng-product-extract"
)

var OpenAIDalleModels = []string{
	Dalle, Dalle2, Dalle3, GPTImage1,
}

var GoogleImagenModels = []string{
	GoogleImagen002,
}

var JimengImageGenerationModels = []string{
	JimengSeedream46,
	JimengSeedream40,
}

var VisionModels = []string{
	GPT4VisionPreview, GPT41106VisionPreview, GPT4Turbo, GPT4Turbo20240409, GPT4O, GPT4O20240513, // openai
	GeminiProVision, Gemini15ProLatest, Gemini15FlashLatest, // gemini
	Claude3,             // anthropic
	ZhiPuChatGLM4Vision, // chatglm
}

var VisionSkipModels = []string{
	GPT4TurboPreview,
}

var VideoModels = []string{
	Sora2,
}

func in(value string, slice []string) bool {
	for _, item := range slice {
		if item == value || strings.Contains(value, item) {
			return true
		}
	}
	return false
}

func IsOpenAIDalleModel(model string) bool {
	// using image generation api if model is in dalle models
	return in(model, OpenAIDalleModels) && !strings.Contains(model, "gpt-4-dalle")
}

func IsGoogleImagenModel(model string) bool {
	// using image generation api if model is in imagen models
	return in(model, GoogleImagenModels)
}

func IsJimengImageGenerationModel(model string) bool {
	return in(model, JimengImageGenerationModels)
}

// JimengImageModels 即梦全部图片相关模型（文生图/图生图/超清/扩图/inpaint/提取）。
var JimengImageModels = []string{
	JimengSeedream46, JimengSeedream40, JimengSuperres,
	JimengOutpaint, JimengInpaint, JimengMaterialExtract, JimengProductExtract,
}

// IsImageGenerationModel 统一判断模型是否「会产出图片」的生成类模型（即梦文生图/图生图、DALLE、Imagen），
// 用于计费默认与能力判定。注意：不同模型内部走不同适配器路径，路由判定仍各自使用专用函数
// （如即梦走图片工厂、DALLE 走聊天适配器），切勿用本函数直接决定路由。
func IsImageGenerationModel(model string) bool {
	return IsJimengImageGenerationModel(model) || IsOpenAIDalleModel(model) || IsGoogleImagenModel(model)
}

// ImageCapability 图片模型能力（粗粒度）。详细参数规格以各适配器为准（如 jimengapi.GetModelSpec）。
type ImageCapability string

const (
	ImageCapNone     ImageCapability = ""
	ImageCapGenerate ImageCapability = "generate" // 文生图 / 图生图
	ImageCapUpscale  ImageCapability = "upscale"  // 超清放大
	ImageCapOutpaint ImageCapability = "outpaint" // 画布扩展
	ImageCapInpaint  ImageCapability = "inpaint"  // 局部重绘
	ImageCapExtract  ImageCapability = "extract"  // 材料 / 商品提取
)

// GetImageModelCapability 返回图片模型的粗粒度能力，非图片模型返回 ImageCapNone。
func GetImageModelCapability(model string) ImageCapability {
	switch {
	case in(model, []string{JimengSuperres}):
		return ImageCapUpscale
	case in(model, []string{JimengOutpaint}):
		return ImageCapOutpaint
	case in(model, []string{JimengInpaint}):
		return ImageCapInpaint
	case in(model, []string{JimengMaterialExtract, JimengProductExtract}):
		return ImageCapExtract
	case IsImageGenerationModel(model):
		return ImageCapGenerate
	default:
		return ImageCapNone
	}
}

func IsVisionModel(model string) bool {
	return in(model, VisionModels) && !in(model, VisionSkipModels)
}

func IsVideoModel(model string) bool {
	return in(model, VideoModels)
}
