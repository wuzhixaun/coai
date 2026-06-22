package dreamina

// ── 提交请求 ──────────────────────────────────────────────────

type ImageSubmitRequest struct {
	Model  string   `json:"model,omitempty"`
	Images []string `json:"images"` // base64 编码
	Prompt string   `json:"prompt"`
	Ratio  string   `json:"ratio,omitempty"`
}

type ImageSubmitResponse struct {
	SubmitId  string `json:"submit_id"`
	GenStatus string `json:"gen_status"`
}

type UpscaleSubmitRequest struct {
	Model          string `json:"model,omitempty"`
	Image          string `json:"image"` // base64
	ResolutionType string `json:"resolution_type"`
}

type VideoSubmitRequest struct {
	Model    string   `json:"model,omitempty"`
	Images   []string `json:"images"`
	Prompt   string   `json:"prompt,omitempty"`
	Duration int      `json:"duration,omitempty"`
}

// ── 轮询/结果 ──────────────────────────────────────────────────

type QueryResultResponse struct {
	SubmitId  string `json:"submit_id"`
	GenStatus string `json:"gen_status"` // success, fail, querying, Generating
	Result    string `json:"result,omitempty"`
	ResultUrl string `json:"result_url,omitempty"` // 图片或视频结果 URL
	VideoUrl  string `json:"video_url,omitempty"`
	FailReason string `json:"fail_reason,omitempty"`
}

type VideoJobResponse struct {
	Id          string `json:"id"`
	Status      string `json:"status"` // completed, failed, processing
	Progress    *int   `json:"progress,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt *int64 `json:"completed_at,omitempty"`
	Error       *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
