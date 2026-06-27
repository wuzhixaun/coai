package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"context"
	"fmt"
	"strings"
)

// classifyImages 规整图片输入：去空白、剥离 data: 前缀，http(s) URL 与纯 base64 都进 images[]。
func classifyImages(images []string) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		s := strings.TrimSpace(img)
		if s == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(s), "data:") {
			if idx := strings.Index(s, ","); idx >= 0 {
				s = strings.TrimSpace(s[idx+1:])
			}
		}
		out = append(out, s)
	}
	return out
}

// ratioFromSize 由目标宽高推导 grsai 的 aspectRatio，无法推导时返回空串。
func ratioFromSize(w, h *int) string {
	if w == nil || h == nil || *w <= 0 || *h <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", *w, *h)
}

// emitResults 把结果下载落地后经 hook 回推（图片 Markdown）。
func (c *Generator) emitResults(res *TaskResponse, hook globals.Hook) error {
	if len(res.Results) == 0 {
		return fmt.Errorf("grsai task finished without result")
	}
	for _, item := range res.Results {
		stored, err := c.storeRemoteURL(item.URL, true)
		if err != nil {
			return err
		}
		if err := hook(&globals.Chunk{Content: utils.GetImageMarkdown(stored)}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Generator) runGenerate(spec ModelSpec, body GenerateRequest, hook globals.Hook) error {
	ctx := context.Background()
	submit, err := c.Submit(ctx, spec, body)
	if err != nil {
		return err
	}
	// async 提交若直接返回成功结果，则无需轮询。
	if submit.IsSucceeded() && len(submit.Results) > 0 {
		return c.emitResults(submit, hook)
	}
	res, err := c.PollResult(ctx, submit.ID, globals.ImageTaskTimeout(), globals.ImagePollInterval())
	if err != nil {
		return err
	}
	return c.emitResults(res, hook)
}

// CreateImageGenerationRequest 文生图。
func (c *Generator) CreateImageGenerationRequest(props *adaptercommon.ImageGenerationProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的生图模型: %s", props.Model)
	}
	body := GenerateRequest{
		Model:       spec.Model,
		Prompt:      strings.TrimSpace(props.Prompt),
		Images:      classifyImages(props.Images),
		AspectRatio: ratioFromSize(props.Width, props.Height),
		ReplyType:   "async",
	}
	if spec.MaxImages > 0 && len(body.Images) > spec.MaxImages {
		body.Images = body.Images[:spec.MaxImages]
	}
	return c.runGenerate(spec, body, hook)
}

// CreateImageEditRequest 图生图 / 换色 / 场景 / 擦除等（带参考图）。
func (c *Generator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的图片编辑模型: %s", props.Model)
	}
	images := classifyImages(props.Images)
	if len(images) == 0 {
		return fmt.Errorf("grsai 图片编辑需要至少 1 张输入图")
	}
	if spec.MaxImages > 0 && len(images) > spec.MaxImages {
		images = images[:spec.MaxImages]
	}
	body := GenerateRequest{
		Model:     spec.Model,
		Prompt:    strings.TrimSpace(props.Prompt),
		Images:    images,
		ReplyType: "async",
	}
	return c.runGenerate(spec, body, hook)
}
