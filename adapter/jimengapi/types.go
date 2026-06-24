package jimengapi

import "strconv"

const (
	defaultEndpoint = "https://visual.volcengineapi.com"
	region          = "cn-north-1"
	service         = "cv"
	apiVersion      = "2022-08-31"
	submitAction    = "CVSync2AsyncSubmitTask"
	getAction       = "CVSync2AsyncGetResult"
	successCode     = 10000
)

type Capability string

const (
	CapabilityGenerate Capability = "generate"
	CapabilityUpscale  Capability = "upscale"
	CapabilityOutpaint Capability = "outpaint"
	CapabilityInpaint  Capability = "inpaint"
	CapabilityExtract  Capability = "extract"
	CapabilityVideo    Capability = "video" // 图生视频，与图片共用同一渠道凭证
)

type ScaleKind string

const (
	ScaleInt1To100 ScaleKind = "int_1_100"
	ScaleFloat0To1 ScaleKind = "float_0_1"
)

type ModelSpec struct {
	Model           string
	ReqKey          string
	Capability      Capability
	MaxImages       int
	MaxOutputCount  int
	MaxPromptRunes  int
	ScaleKind       ScaleKind
	DefaultScale    float64
	MinSizeArea     int
	MaxSizeArea     int
	DefaultMinRatio float64
	DefaultMaxRatio float64
	OutputFormat    string
	// PromptField 指定 prompt 写入哪个请求字段。提取类接口使用
	// image_edit_prompt / edit_prompt，其余默认 "prompt"。
	PromptField string
	// DefaultSeed 提供给需要 seed 的能力（如 inpaint），<0 表示不设置。
	DefaultSeed int
}

var modelSpecs = map[string]ModelSpec{
	"jimeng-seedream-4.6": {
		Model:           "jimeng-seedream-4.6",
		ReqKey:          "jimeng_seedream46_cvtob",
		Capability:      CapabilityGenerate,
		MaxImages:       14,
		MaxOutputCount:  6,
		MaxPromptRunes:  800,
		ScaleKind:       ScaleInt1To100,
		DefaultScale:    50,
		MinSizeArea:     1024 * 1024,
		MaxSizeArea:     4096 * 4096,
		DefaultMinRatio: 1.0 / 3.0,
		DefaultMaxRatio: 3,
		OutputFormat:    "png",
	},
	"jimeng-seedream-4.0": {
		Model:           "jimeng-seedream-4.0",
		ReqKey:          "jimeng_t2i_v40",
		Capability:      CapabilityGenerate,
		MaxImages:       10,
		MaxOutputCount:  6,
		MaxPromptRunes:  800,
		ScaleKind:       ScaleFloat0To1,
		DefaultScale:    0.5,
		MinSizeArea:     1024 * 1024,
		MaxSizeArea:     4096 * 4096,
		DefaultMinRatio: 1.0 / 3.0,
		DefaultMaxRatio: 3,
		OutputFormat:    "png",
	},
	"jimeng-superres": {
		Model:          "jimeng-superres",
		ReqKey:         "jimeng_i2i_seed3_tilesr_cvtob",
		Capability:     CapabilityUpscale,
		MaxImages:      1,
		MaxOutputCount: 1,
		ScaleKind:      ScaleInt1To100,
		DefaultScale:   50,
		OutputFormat:   "png",
	},
	"jimeng-outpaint": {
		Model:          "jimeng-outpaint",
		ReqKey:         "jimeng_img2img_seed3_painting_edit",
		Capability:     CapabilityOutpaint,
		MaxImages:      1,
		MaxOutputCount: 1,
		MaxPromptRunes: 800,
		OutputFormat:   "png",
	},
	"jimeng-inpaint": {
		Model:          "jimeng-inpaint",
		ReqKey:         "jimeng_image2image_dream_inpaint",
		Capability:     CapabilityInpaint,
		MaxImages:      2, // 源图 + 单通道灰度 mask
		MaxOutputCount: 1,
		MaxPromptRunes: 800,
		PromptField:    "prompt",
		DefaultSeed:    101,
		OutputFormat:   "jpeg",
	},
	"jimeng-material-extract": {
		Model:          "jimeng-material-extract",
		ReqKey:         "i2i_material_extraction",
		Capability:     CapabilityExtract,
		MaxImages:      1,
		MaxOutputCount: 1,
		MaxPromptRunes: 800,
		PromptField:    "image_edit_prompt",
		DefaultSeed:    -1,
		OutputFormat:   "jpeg",
	},
	"jimeng-product-extract": {
		Model:          "jimeng-product-extract",
		ReqKey:         "jimeng_i2i_extract_tiled_images",
		Capability:     CapabilityExtract,
		MaxImages:      1,
		MaxOutputCount: 1,
		MaxPromptRunes: 800,
		PromptField:    "edit_prompt", // 接口表字段；示例中曾出现 image_edit_prompt
		DefaultSeed:    -1,
		OutputFormat:   "jpeg",
	},
	// 图生视频：官方即梦视频生成，与图片共用同一火山视觉 API 凭证与
	// CVSync2AsyncSubmitTask / CVSync2AsyncGetResult 流程，仅 req_key 不同。
	"jimeng-video": {
		Model:          "jimeng-video",
		ReqKey:         "jimeng_ti2v_v30_pro", // 即梦视频 3.0 Pro（文/图生视频统一 req_key，frames 121/241=5s/10s）
		Capability:     CapabilityVideo,
		MaxImages:      9,
		MaxOutputCount: 1,
		MaxPromptRunes: 800,
		DefaultSeed:    -1,
	},
}

func GetModelSpec(model string) (ModelSpec, bool) {
	spec, ok := modelSpecs[model]
	return spec, ok
}

type SubmitTaskRequest struct {
	ReqKey           string   `json:"req_key"`
	ImageURLs        []string `json:"image_urls,omitempty"`
	BinaryDataBase64 []string `json:"binary_data_base64,omitempty"`
	Prompt           string   `json:"prompt"`
	Size             *int     `json:"size,omitempty"`
	Width            *int     `json:"width,omitempty"`
	Height           *int     `json:"height,omitempty"`
	Scale            any      `json:"scale,omitempty"`
	ForceSingle      *bool    `json:"force_single,omitempty"`
	MinRatio         *float64 `json:"min_ratio,omitempty"`
	MaxRatio         *float64 `json:"max_ratio,omitempty"`
	Seed             *int     `json:"seed,omitempty"`

	// 智能超清 (jimeng-superres)
	Resolution *string `json:"resolution,omitempty"` // 4k / 8k

	// 智能扩图 (jimeng-outpaint)，方向扩展比例 [0,1]
	Top    *float64 `json:"top,omitempty"`
	Bottom *float64 `json:"bottom,omitempty"`
	Left   *float64 `json:"left,omitempty"`
	Right  *float64 `json:"right,omitempty"`

	// 提取类接口的 prompt 字段（按模型择一写入）
	ImageEditPrompt *string  `json:"image_edit_prompt,omitempty"` // 素材提取 POD
	EditPrompt      *string  `json:"edit_prompt,omitempty"`       // 商品提取
	LoraWeight      *float64 `json:"lora_weight,omitempty"`       // 素材提取可选

	// 图生视频 (jimeng-video)
	AspectRatio *string `json:"aspect_ratio,omitempty"` // 如 "16:9"，留空则按参考图自适应
	Frames      *int    `json:"frames,omitempty"`       // 视频帧数，留空使用模型默认
}

type GetResultRequest struct {
	ReqKey  string `json:"req_key"`
	TaskID  string `json:"task_id"`
	ReqJSON string `json:"req_json,omitempty"`
}

type GetResultOptions struct {
	ReturnURL bool `json:"return_url,omitempty"`
}

type APIResponse struct {
	Code        int          `json:"code"`
	Message     string       `json:"message"`
	Status      int          `json:"status,omitempty"`
	RequestID   string       `json:"request_id"`
	TimeElapsed string       `json:"time_elapsed,omitempty"`
	TaskID      string       `json:"task_id,omitempty"`
	Data        *TaskPayload `json:"data"`
}

type TaskPayload struct {
	TaskID           string   `json:"task_id,omitempty"`
	Status           string   `json:"status,omitempty"`
	ImageURLs        []string `json:"image_urls,omitempty"`
	BinaryDataBase64 []string `json:"binary_data_base64,omitempty"`
	RespData         string   `json:"resp_data,omitempty"`
	VideoURL         string   `json:"video_url,omitempty"` // 图生视频结果地址
}

func (r *APIResponse) IsSuccess() bool {
	return r != nil && r.Code == successCode
}

func (r *APIResponse) ErrorMessage(prefix string) string {
	if r == nil {
		return prefix + ": empty response"
	}
	msg := r.Message
	if msg == "" {
		msg = "unknown"
	}
	if r.RequestID != "" {
		return prefix + ": code=" + strconv.Itoa(r.Code) + " message=" + msg + " request_id=" + r.RequestID
	}
	return prefix + ": code=" + strconv.Itoa(r.Code) + " message=" + msg
}
