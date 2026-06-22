package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
	"strings"
)

// CreateImageOutpaintRequest 使用官方智能扩图（jimeng-outpaint）按目标比例扩展画布。
// Photo 仅传入目标比例，适配器据原图尺寸计算只扩不裁的四向扩展比例。
func (c *ImageGenerator) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	model := props.Model
	spec, ok := GetModelSpec(model)
	if !ok || spec.Capability != CapabilityOutpaint {
		spec, ok = GetModelSpec(globals.JimengOutpaint)
		if !ok {
			return fmt.Errorf("jimeng-api outpaint model is not registered")
		}
	}

	urls, base64s := classifyImageInputs([]string{props.Image})
	if len(urls)+len(base64s) == 0 {
		return fmt.Errorf("jimeng-api outpaint requires an input image")
	}

	targetW, targetH, err := parseRatio(props.TargetRatio)
	if err != nil {
		return err
	}

	width, height, err := decodeImageSize(urls, base64s)
	if err != nil {
		return err
	}

	top, bottom, left, right := computeOutpaintEdges(width, height, targetW, targetH)
	if top == 0 && bottom == 0 && left == 0 && right == 0 {
		return fmt.Errorf("jimeng-api outpaint: target ratio %s already matches source %dx%d, nothing to expand", props.TargetRatio, width, height)
	}

	req := SubmitTaskRequest{
		ReqKey:           spec.ReqKey,
		ImageURLs:        urls,
		BinaryDataBase64: base64s,
		Prompt:           strings.TrimSpace(props.Prompt),
	}
	if top > 0 {
		req.Top = &top
	}
	if bottom > 0 {
		req.Bottom = &bottom
	}
	if left > 0 {
		req.Left = &left
	}
	if right > 0 {
		req.Right = &right
	}

	return c.runTaskAndEmit(spec.ReqKey, req, hook)
}
