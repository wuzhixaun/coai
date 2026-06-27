package grsai

// Capability 标识模型能力类别。
type Capability int

const (
	CapabilityGenerate Capability = iota // 文生图 / 图生图
	CapabilityVideo                      // 文/图生视频
)

// ModelSpec 描述一个 grsai 模型的调用方式。
type ModelSpec struct {
	Model      string
	Path       string // 提交接口相对路径，如 /v1/api/generate
	Capability Capability
	MaxImages  int // 允许的最大参考图数量，0 表示不限制
}

// modelSpecs 是 grsai 模型注册表。新增模型只需加一行。
var modelSpecs = map[string]ModelSpec{
	"nano-banana":   {Model: "nano-banana", Path: "/v1/api/generate", Capability: CapabilityGenerate, MaxImages: 6},
	"nano-banana-2": {Model: "nano-banana-2", Path: "/v1/api/generate", Capability: CapabilityGenerate, MaxImages: 6},
	"gpt-image":     {Model: "gpt-image", Path: "/v1/api/generate", Capability: CapabilityGenerate, MaxImages: 6},
	"veo":           {Model: "veo", Path: "/v1/api/generate", Capability: CapabilityVideo, MaxImages: 1},
}

// GetModelSpec 按模型名查注册表。
func GetModelSpec(model string) (ModelSpec, bool) {
	spec, ok := modelSpecs[model]
	return spec, ok
}

// GenerateRequest 是 nano-banana / gpt-image / veo 的统一提交体。
// veo 不使用 ImageSize，其余字段通用。
type GenerateRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Images      []string `json:"images,omitempty"`
	AspectRatio string   `json:"aspectRatio,omitempty"`
	ImageSize   string   `json:"imageSize,omitempty"`
	ReplyType   string   `json:"replyType"`
}

// TaskResult 单条结果。
type TaskResult struct {
	URL string `json:"url"`
}

// TaskResponse 是提交与结果查询的统一响应体。
type TaskResponse struct {
	ID       string       `json:"id"`
	Status   string       `json:"status"`
	Progress int          `json:"progress"`
	Results  []TaskResult `json:"results"`
	Error    string       `json:"error"`
}

// IsTerminal 终态：succeeded / failed / violation。
func (r *TaskResponse) IsTerminal() bool {
	switch r.Status {
	case "succeeded", "failed", "violation":
		return true
	default:
		return false
	}
}

// IsSucceeded 是否成功。
func (r *TaskResponse) IsSucceeded() bool {
	return r.Status == "succeeded"
}

// ErrorMessage 返回错误信息，缺失时回退到 def。
func (r *TaskResponse) ErrorMessage(def string) string {
	if r.Error != "" {
		return r.Error
	}
	if r.Status == "violation" {
		return "content violation"
	}
	return def
}
