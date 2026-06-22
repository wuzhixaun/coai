package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
	"strings"
)

// resolveUpscaleResolution 把 Photo 的 2k/4k/8k 归一化到官方支持的 4k/8k。
func resolveUpscaleResolution(resolutionType string) string {
	switch strings.ToLower(strings.TrimSpace(resolutionType)) {
	case "8k":
		return "8k"
	default:
		// 官方仅支持 4k（默认）/ 8k；2k 等其它取值统一映射到 4k。
		return "4k"
	}
}

// CreateImageUpscaleRequest 使用官方智能超清（jimeng-superres）放大图片。
func (c *ImageGenerator) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	model := props.Model
	spec, ok := GetModelSpec(model)
	if !ok || spec.Capability != CapabilityUpscale {
		// 兼容 Photo 未显式配置官方超清模型的情况。
		spec, ok = GetModelSpec(globals.JimengSuperres)
		if !ok {
			return fmt.Errorf("jimeng-api super-resolution model is not registered")
		}
	}

	urls, base64s := classifyImageInputs([]string{props.Image})
	if len(urls)+len(base64s) == 0 {
		return fmt.Errorf("jimeng-api super-resolution requires an input image")
	}

	resolution := resolveUpscaleResolution(props.ResolutionType)
	req := SubmitTaskRequest{
		ReqKey:           spec.ReqKey,
		ImageURLs:        urls,
		BinaryDataBase64: base64s,
		Resolution:       &resolution,
	}

	// 细节生成强度默认 50（int [0,100]）。
	if spec.DefaultScale > 0 {
		req.Scale = int(spec.DefaultScale)
	}

	return c.runTaskAndEmit(spec.ReqKey, req, hook)
}
