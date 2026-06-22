package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
	"strings"
)

// CreateImageEditRequest 使用官方 seedream 4.6/4.0 的图生图能力进行图片编辑，
// 输入图片支持 base64（Photo 本地文件）与 http(s) URL 两种形式。
func (c *ImageGenerator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok {
		return fmt.Errorf("jimeng-api unsupported model: %s", props.Model)
	}
	if spec.Capability != CapabilityGenerate {
		return fmt.Errorf("jimeng-api model %s does not support image editing", props.Model)
	}

	prompt := strings.TrimSpace(props.Prompt)
	if prompt == "" {
		return fmt.Errorf("jimeng-api image edit requires a prompt")
	}
	if spec.MaxPromptRunes > 0 && len([]rune(prompt)) > spec.MaxPromptRunes {
		return fmt.Errorf("jimeng-api prompt for %s exceeds %d characters", spec.Model, spec.MaxPromptRunes)
	}

	urls, base64s := classifyImageInputs(props.Images)
	total := len(urls) + len(base64s)
	if total == 0 {
		return fmt.Errorf("jimeng-api image edit requires at least one input image")
	}
	if spec.MaxImages > 0 && total > spec.MaxImages {
		return fmt.Errorf("jimeng-api model %s accepts at most %d input images, got %d", spec.Model, spec.MaxImages, total)
	}

	forceSingle := true
	req := SubmitTaskRequest{
		ReqKey:           spec.ReqKey,
		Prompt:           prompt,
		ImageURLs:        urls,
		BinaryDataBase64: base64s,
		ForceSingle:      &forceSingle,
	}

	// Photo 的 Strength 可选映射到官方 scale（按各模型量纲归一化）。
	if props.Strength != nil {
		strength := float64(*props.Strength)
		scale, err := normalizeScale(&strength, spec)
		if err != nil {
			return err
		}
		req.Scale = scale
	}

	return c.runTaskAndEmit(spec.ReqKey, req, hook)
}
