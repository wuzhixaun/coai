package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"context"
	"fmt"
	"strings"
	"time"
)

// videoPollMaxWait 视频生成较慢，给足轮询时长。
const videoPollMaxWait = 20 * time.Minute

// CreateImageToVideoRequest 图/文生视频。
// 放开本地能力校验：所选模型一律提交到 grsai 视频端点 /v1/video/veo，由 grsai API
// 决定是否支持（不支持时原样透传 API 错误，如 "model not found"）。
func (c *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	// 默认强制走视频端点；若注册表里该模型本就是视频模型，则沿用其配置。
	spec := ModelSpec{Model: props.Model, Path: "/v1/video/veo", Surface: SurfaceB, Capability: CapabilityVideo, MaxImages: 1}
	if s, ok := GetModelSpec(props.Model); ok && s.Capability == CapabilityVideo {
		spec = s
	}
	images := classifyImages(props.Images)
	if spec.MaxImages > 0 && len(images) > spec.MaxImages {
		images = images[:spec.MaxImages]
	}
	body := GenerateRequest{
		Model:     spec.Model,
		Prompt:    strings.TrimSpace(props.Prompt),
		Images:    images,
		ReplyType: "async",
	}

	urls, err := c.runTask(context.Background(), spec, body, videoPollMaxWait, 10*time.Second)
	if err != nil {
		return err
	}
	if len(urls) == 0 || strings.TrimSpace(urls[0]) == "" {
		return fmt.Errorf("grsai 未返回视频结果")
	}

	stored, err := c.storeRemoteURL(urls[0], false)
	if err != nil {
		globals.Warn(fmt.Sprintf("[grsai] 视频结果落地失败，回退源地址: %s", err))
		stored = urls[0]
	}
	return hook(&globals.Chunk{Content: stored})
}
