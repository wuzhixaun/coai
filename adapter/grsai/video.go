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
