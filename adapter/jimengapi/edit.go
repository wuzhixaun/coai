package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
	"strings"
)

// CreateImageEditRequest 是 Photo 页面图片编辑类能力的统一入口，按模型能力分流：
//   - generate: seedream 4.6/4.0 图生图编辑
//   - inpaint:  局部重绘/消除笔（源图 + 单通道灰度 mask 两张图）
//   - extract:  素材提取 / 商品提取（单图 + 类别 prompt，写入专用字段）
//
// 输入图片支持 base64（Photo 本地文件）与 http(s) URL。
func (c *ImageGenerator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok {
		return fmt.Errorf("jimeng-api unsupported model: %s", props.Model)
	}

	prompt := strings.TrimSpace(props.Prompt)
	if spec.MaxPromptRunes > 0 && len([]rune(prompt)) > spec.MaxPromptRunes {
		return fmt.Errorf("jimeng-api prompt for %s exceeds %d characters", spec.Model, spec.MaxPromptRunes)
	}

	urls, base64s := classifyImageInputs(props.Images)
	total := len(urls) + len(base64s)

	switch spec.Capability {
	case CapabilityGenerate:
		if prompt == "" {
			return fmt.Errorf("jimeng-api image edit requires a prompt")
		}
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
		if props.Strength != nil {
			strength := float64(*props.Strength)
			scale, err := normalizeScale(&strength, spec)
			if err != nil {
				return err
			}
			req.Scale = scale
		}
		return c.runTaskAndEmit(spec.ReqKey, req, hook)

	case CapabilityInpaint:
		// 需源图 + 单通道灰度 mask 两张图，且要保持「先源图后 mask」的顺序。
		// classifyImageInputs 在各自分桶内保留输入顺序；Photo 两张图均为 base64，
		// 因此 base64s = [源图, mask] 顺序正确。
		if len(props.Images) != 2 {
			return fmt.Errorf("jimeng-api inpaint requires exactly 2 images (source + mask), got %d", len(props.Images))
		}
		if len(base64s) > 0 && len(urls) > 0 {
			return fmt.Errorf("jimeng-api inpaint requires both source and mask in the same form (both base64 or both URLs)")
		}
		req := SubmitTaskRequest{
			ReqKey:           spec.ReqKey,
			Prompt:           prompt, // 传 "删除" 执行消除
			BinaryDataBase64: base64s,
			ImageURLs:        urls,
		}
		if spec.DefaultSeed >= 0 {
			seed := spec.DefaultSeed
			req.Seed = &seed
		}
		return c.runTaskAndEmit(spec.ReqKey, req, hook)

	case CapabilityExtract:
		if prompt == "" {
			return fmt.Errorf("jimeng-api %s requires a category prompt", spec.Model)
		}
		if total != 1 {
			return fmt.Errorf("jimeng-api %s requires exactly 1 input image, got %d", spec.Model, total)
		}
		req := SubmitTaskRequest{
			ReqKey:           spec.ReqKey,
			ImageURLs:        urls,
			BinaryDataBase64: base64s,
		}
		setPromptField(&req, spec, prompt)
		if spec.DefaultSeed >= 0 {
			seed := spec.DefaultSeed
			req.Seed = &seed
		}
		return c.runTaskAndEmit(spec.ReqKey, req, hook)

	default:
		return fmt.Errorf("jimeng-api model %s does not support image editing (capability=%s)", spec.Model, spec.Capability)
	}
}

// setPromptField 根据模型把 prompt 写入正确的请求字段。
func setPromptField(req *SubmitTaskRequest, spec ModelSpec, prompt string) {
	switch spec.PromptField {
	case "image_edit_prompt":
		req.ImageEditPrompt = &prompt
	case "edit_prompt":
		req.EditPrompt = &prompt
	default:
		req.Prompt = prompt
	}
}
