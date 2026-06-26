package photo

// ── 任务状态 ────────────────────────────────────────────────

const (
	TaskStatusPending    = "pending"
	TaskStatusProcessing = "processing"
	TaskStatusSuccess    = "success"
	TaskStatusFailed     = "failed"
)

// ── 15 功能类型 ──────────────────────────────────────────────

const (
	FeatureWhiteBg        = "white_bg"
	FeatureSceneGen       = "scene_gen"
	FeatureImageErase     = "image_erase"
	FeatureColorChange    = "color_change"
	FeatureMarketing      = "marketing"
	FeatureImageTranslate = "image_translate"
	FeatureHdUpscale      = "hd_upscale"
	FeatureModelImage     = "model_image"
	FeatureMaterialChange = "material_change"
	FeatureInstructionGen = "instruction_gen"
	FeatureDetailImage    = "detail_image"
	FeatureLogoCustom     = "logo_custom"
	FeatureProductionFlow = "production_flow"
	FeatureResize         = "resize"
	FeatureVideoGen       = "video_gen"
	FeatureMaterialExtract = "material_extract"
	FeatureProductExtract  = "product_extract"
)

var AllFeatures = []string{
	FeatureWhiteBg, FeatureSceneGen, FeatureImageErase, FeatureColorChange,
	FeatureMarketing, FeatureImageTranslate, FeatureHdUpscale, FeatureModelImage,
	FeatureMaterialChange, FeatureInstructionGen, FeatureDetailImage, FeatureLogoCustom,
	FeatureProductionFlow, FeatureResize, FeatureVideoGen,
	FeatureMaterialExtract, FeatureProductExtract,
}

// localFeatures 现已为空：detail_image / logo_custom 均改由官方即梦 AI 处理。
// 保留该映射以便将来如有纯本地功能可继续登记。
var localFeatures = map[string]bool{}

func IsAIFeature(feature string) bool {
	return !localFeatures[feature]
}

// ── 图片信息 ─────────────────────────────────────────────────

type ImageInfo struct {
	Id         string `json:"id"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Url        string `json:"url"`
	FolderName string `json:"folder_name"`
	CreatedAt  string `json:"created_at"`
}

// ── 任务信息 ─────────────────────────────────────────────────

// ItemStatus 逐图状态：批量任务中每张源图的处理结果，支持精确的部分成功与只重试失败项。
type ItemStatus struct {
	Index    int      `json:"index"`
	Filename string   `json:"filename"`
	Status   string   `json:"status"` // success | failed
	Urls     []string `json:"urls"`
	Error    string   `json:"error"`
}

type TaskInfo struct {
	TaskId          string   `json:"task_id"`
	Feature         string   `json:"feature"`
	Status          string   `json:"status"`
	ImageIds        []string `json:"image_ids"`
	ResultUrls      []string `json:"result_urls"`
	ErrorMessage    string   `json:"error_message"`
	Progress        int      `json:"progress"`
	CreatedAt       string   `json:"created_at"`
	FolderName      string   `json:"folder_name"`
	TotalImages     int      `json:"total_images"`
	ProcessedImages int      `json:"processed_images"`
	TotalVideos     int      `json:"total_videos"`
	ProcessedVideos int      `json:"processed_videos"`
	CompletedAt     string   `json:"completed_at"`
	SourceFilenames []string `json:"source_filenames"`
	SubmitIds       []string `json:"submit_ids"`
	ItemStatus      []ItemStatus `json:"item_status"`
}

// ── 一致性身份 ───────────────────────────────────────────────

const (
	IdentityTypeProduct  = "product"
	IdentityTypeModel    = "model"
	IdentityTypeBrandKit = "brandkit"
)

// IdentityInfo 一致性身份：
//   - product/model：一组参考图 + 锁定 seed + 主体描述，保持主体一致
//   - brandkit：Logo(存于 ref_image_ids[0]) + 主色(color)，产出时叠加品牌元素
type IdentityInfo struct {
	Id            string   `json:"id"`
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	RefImageIds   []string `json:"ref_image_ids"`
	RefImageUrls  []string `json:"ref_image_urls"` // 派生：供前端展示，不入库
	Seed          int      `json:"seed"`
	SubjectPrompt string   `json:"subject_prompt"`
	Color         string   `json:"color"` // 品牌主色(brandkit)，存于 meta
	CreatedAt     string   `json:"created_at"`
}

// CreateIdentityRequest 新建身份请求
type CreateIdentityRequest struct {
	Type          string   `json:"type"`
	Name          string   `json:"name" binding:"required"`
	RefImageIds   []string `json:"ref_image_ids" binding:"required,min=1"`
	SubjectPrompt string   `json:"subject_prompt"`
	Color         string   `json:"color"` // 品牌主色(brandkit)
}

// ── 请求体 ───────────────────────────────────────────────────

// ProcessRequest 统一处理请求（多功能批量）
type ProcessRequest struct {
	ImageIds        []string               `json:"image_ids" binding:"required,min=1,max=50"`
	Features        []string               `json:"features" binding:"required,min=1"`
	Params          map[string]interface{}  `json:"params"`
	SystemPrompt    string                 `json:"system_prompt"`
	ChannelOverride string                 `json:"channel_override"` // 可选，覆盖默认渠道
	FeatureParams   map[string]interface{} `json:"feature_params"`   // 每个功能的独立参数
	IdentityId      string                 `json:"identity_id"`      // 可选，应用一致性身份（参考图+seed+主体）
	BrandKitId      string                 `json:"brand_kit_id"`     // 可选，叠加品牌资产（Logo+主色），与 identity 可组合
}
