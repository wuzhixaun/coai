package grsai

import "strings"

// Capability 标识模型能力类别。
type Capability int

const (
	CapabilityGenerate Capability = iota // 文生图 / 图生图
	CapabilityVideo                      // 文/图生视频
)

// Surface 标识 grsai 的两套接口面。
type Surface int

const (
	// SurfaceA：nano-banana 系。提交 POST /v1/api/generate，结果 GET /v1/api/result，
	// 响应为扁平 JSON {id,status,progress,results[].url,error}。
	SurfaceA Surface = iota
	// SurfaceB：gpt-image-2 / veo3-* 系。提交 POST /v1/draw/completions 或 /v1/video/veo，
	// 响应为 SSE（data: 帧）；结果 POST /v1/draw/result，响应为 {code,data,msg} 包装。
	SurfaceB
)

// ModelSpec 描述一个 grsai 模型的调用方式。
type ModelSpec struct {
	Model      string
	Path       string // 提交接口相对路径
	Surface    Surface
	Capability Capability
	MaxImages  int // 允许的最大参考图数量，0 表示不限制
}

// modelSpecs 是 grsai 模型注册表。新增模型只需加一行。
// 模型名与接口面经实测确认（2026-06-27）：nano-banana 系走 SurfaceA；
// gpt-image-2、veo3.1-fast、veo3.1-pro 走 SurfaceB。
var modelSpecs = map[string]ModelSpec{
	"nano-banana":   {Model: "nano-banana", Path: "/v1/api/generate", Surface: SurfaceA, Capability: CapabilityGenerate, MaxImages: 6},
	"nano-banana-2": {Model: "nano-banana-2", Path: "/v1/api/generate", Surface: SurfaceA, Capability: CapabilityGenerate, MaxImages: 6},
	"gpt-image-2":   {Model: "gpt-image-2", Path: "/v1/draw/completions", Surface: SurfaceB, Capability: CapabilityGenerate, MaxImages: 6},
	"veo3.1-fast":   {Model: "veo3.1-fast", Path: "/v1/video/veo", Surface: SurfaceB, Capability: CapabilityVideo, MaxImages: 1},
	"veo3.1-pro":    {Model: "veo3.1-pro", Path: "/v1/video/veo", Surface: SurfaceB, Capability: CapabilityVideo, MaxImages: 1},
}

// GetModelSpec 按模型名查注册表。
func GetModelSpec(model string) (ModelSpec, bool) {
	spec, ok := modelSpecs[model]
	return spec, ok
}

// GenerateRequest 是两套接口面通用的提交体（model 字段决定路由）。
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

// ── SurfaceA 响应 ────────────────────────────────────────

// TaskResponse 是 SurfaceA 提交与结果查询的扁平响应体。
type TaskResponse struct {
	ID       string       `json:"id"`
	Status   string       `json:"status"`
	Progress int          `json:"progress"`
	Results  []TaskResult `json:"results"`
	Error    string       `json:"error"`
}

// IsTerminal 终态：succeeded / failed / violation。
func (r *TaskResponse) IsTerminal() bool { return isTerminalStatus(r.Status) }

// IsSucceeded 是否成功。
func (r *TaskResponse) IsSucceeded() bool { return r.Status == "succeeded" }

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

// ── SurfaceB 响应 ────────────────────────────────────────

// TaskResponseB 是 SurfaceB（gpt-image-2 / veo3-*）SSE 流中每个 data: 帧的对象，
// 进度帧与终态帧同构；终态帧带 status=succeeded 与 results[].url。
type TaskResponseB struct {
	ID            string       `json:"id"`
	Status        string       `json:"status"`
	Progress      int          `json:"progress"`
	URL           string       `json:"url"`
	Results       []TaskResult `json:"results"`
	Error         string       `json:"error"`
	FailureReason string       `json:"failure_reason"`
}

// IsTerminal 终态：succeeded / failed / violation。
func (r *TaskResponseB) IsTerminal() bool { return isTerminalStatus(r.Status) }

// IsSucceeded 是否成功。
func (r *TaskResponseB) IsSucceeded() bool { return r.Status == "succeeded" }

// ErrorMessage 返回错误信息，缺失时回退到 def。
func (r *TaskResponseB) ErrorMessage(def string) string {
	if r.Error != "" {
		return r.Error
	}
	if r.FailureReason != "" {
		return r.FailureReason
	}
	if r.Status == "violation" {
		return "content violation"
	}
	return def
}

// URLs 提取结果地址：优先 results[].url，回退顶层 url。
func (r *TaskResponseB) URLs() []string {
	urls := make([]string, 0, len(r.Results)+1)
	for _, it := range r.Results {
		if u := strings.TrimSpace(it.URL); u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		if u := strings.TrimSpace(r.URL); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

// isTerminalStatus 判定状态是否为终态。
func isTerminalStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "violation":
		return true
	default:
		return false
	}
}
