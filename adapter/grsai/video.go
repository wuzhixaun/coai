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

// CreateImageToVideoRequest 图/文生视频（veo）。
func (c *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityVideo {
		return fmt.Errorf("grsai 不支持的视频模型: %s", props.Model)
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

	ctx := context.Background()
	submit, err := c.Submit(ctx, spec, body)
	if err != nil {
		return err
	}
	res := submit
	if !(submit.IsSucceeded() && len(submit.Results) > 0) {
		res, err = c.PollResult(ctx, submit.ID, videoPollMaxWait, 10*time.Second)
		if err != nil {
			return err
		}
	}
	if len(res.Results) == 0 || strings.TrimSpace(res.Results[0].URL) == "" {
		return fmt.Errorf("grsai 未返回视频结果 (id=%s)", submit.ID)
	}

	stored, err := c.storeRemoteURL(res.Results[0].URL, false)
	if err != nil {
		globals.Warn(fmt.Sprintf("[grsai] 视频结果落地失败，回退源地址 (id=%s): %s", submit.ID, err))
		stored = res.Results[0].URL
	}
	return hook(&globals.Chunk{Content: stored})
}
