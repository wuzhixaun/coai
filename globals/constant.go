package globals

const (
	System    = "system"
	User      = "user"
	Assistant = "assistant"
	Tool      = "tool"
	Function  = "function"
)

const (
	OpenAIChannelType      = "openai"
	AzureOpenAIChannelType = "azure"
	ClaudeChannelType      = "claude"
	SlackChannelType       = "slack"
	SparkdeskChannelType   = "sparkdesk"
	ChatGLMChannelType     = "chatglm"
	HunyuanChannelType     = "hunyuan"
	QwenChannelType        = "qwen"
	ZhinaoChannelType      = "zhinao"
	BaichuanChannelType    = "baichuan"
	SkylarkChannelType     = "skylark"
	BingChannelType        = "bing"
	PalmChannelType        = "palm"
	MidjourneyChannelType  = "midjourney"
	MoonshotChannelType    = "moonshot"
	GroqChannelType        = "groq"
	DeepseekChannelType    = "deepseek"
	DifyChannelType        = "dify"
	CozeChannelType        = "coze"
)

// 图片处理专用渠道类型
const (
	// dreamina（自定义 Bearer 代理）已退役，能力由官方 jimeng-api 取代。
	JimengChannelType    = "jimeng"     // 即梦 CLI (subprocess)，仅 video_gen 仍在使用，待官方视频模型接入后下线
	JimengAPIChannelType = "jimeng-api" // 火山引擎即梦官方 Visual API
	GrsaiChannelType     = "grsai"      // grsai 多模型平台（nano-banana / gpt-image / veo）
	// ArkChannelType 火山方舟（Ark）：一个 API Key 同时服务对话(/chat/completions)、
	// 图片生成(/images/generations)、视频生成(/contents/generations/tasks 异步任务)。
	ArkChannelType = "ark"
)

const (
	NonBilling   = "non-billing"
	TimesBilling = "times-billing"
	TokenBilling = "token-billing"
	// ImageBilling 按生成图片张数计费：每张消耗 GetOutput() 额度，区别于 TimesBilling 的「每请求固定费」。
	ImageBilling = "image-billing"
)

const (
	AnonymousType = "anonymous"
	NormalType    = "normal"
	BasicType     = "basic"    // basic subscription
	StandardType  = "standard" // standard subscription
	ProType       = "pro"      // pro subscription
	AdminType     = "admin"
)

const (
	NoneProxyType = iota
	HttpProxyType
	HttpsProxyType
	Socks5ProxyType
)

const (
	WebTokenType = "web"
	ApiTokenType = "api"
	SystemToken  = "system"
)
