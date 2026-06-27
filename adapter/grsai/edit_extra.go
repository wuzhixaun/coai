package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
	"strings"
)

// normalizeImageSize 把 photo 的分辨率档位映射到 grsai imageSize（grsai 上限 4K）。
func normalizeImageSize(res string) string {
	switch strings.ToLower(strings.TrimSpace(res)) {
	case "4k", "8k":
		return "4K"
	case "2k":
		return "2K"
	default:
		return "2K"
	}
}

// CreateImageUpscaleRequest 用 generate + imageSize 近似超分（grsai 无独立超分端点）。
func (c *Generator) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的超分模型: %s", props.Model)
	}
	images := classifyImages([]string{props.Image})
	if len(images) == 0 {
		return fmt.Errorf("grsai 超分需要 1 张输入图")
	}
	body := GenerateRequest{
		Model:     spec.Model,
		Prompt:    "upscale this image to higher resolution, keep all original content and details unchanged",
		Images:    images,
		ImageSize: normalizeImageSize(props.ResolutionType),
		ReplyType: "async",
	}
	return c.runGenerate(spec, body, hook)
}

// CreateImageOutpaintRequest 用 generate + aspectRatio 近似扩图（grsai 无独立扩图端点）。
func (c *Generator) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的扩图模型: %s", props.Model)
	}
	images := classifyImages([]string{props.Image})
	if len(images) == 0 {
		return fmt.Errorf("grsai 扩图需要 1 张输入图")
	}
	prompt := strings.TrimSpace(props.Prompt)
	if prompt == "" {
		prompt = "expand the canvas naturally, extend the background to fill the new area, do not crop the subject"
	}
	body := GenerateRequest{
		Model:       spec.Model,
		Prompt:      prompt,
		Images:      images,
		AspectRatio: strings.TrimSpace(props.TargetRatio),
		ReplyType:   "async",
	}
	return c.runGenerate(spec, body, hook)
}
