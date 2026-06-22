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

// isAI 判断功能是否需要 AI（本地功能：detail_image, logo_custom）
var localFeatures = map[string]bool{
	FeatureDetailImage: true,
	FeatureLogoCustom:  true,
}

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
}
