package jimeng

import (
	"fmt"
	"os"
	"time"

	adaptercommon "chat/adapter/common"
	"chat/globals"
)

// CreateImageToVideoRequest 图生视频 (multimodal2video)
func (c *CLIAdapter) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	if len(props.Images) == 0 {
		return fmt.Errorf("视频生成需要至少 1 张参考图")
	}
	if len(props.Images) > 9 {
		return fmt.Errorf("视频生成最多支持 9 张参考图")
	}

	// 1. 保存图片为临时文件
	var imageFiles []string
	for _, b64 := range props.Images {
		path, err := saveBase64Image(b64, "jimeng_video")
		if err != nil {
			return fmt.Errorf("保存参考图失败: %w", err)
		}
		defer os.Remove(path)
		imageFiles = append(imageFiles, path)
	}

	// 2. 构建 CLI 命令
	args := []string{
		"multimodal2video",
	}
	for _, f := range imageFiles {
		args = append(args, "--image", f)
	}
	if props.Prompt != "" {
		args = append(args, "--prompt", props.Prompt)
	}
	if props.Duration >= 4 {
		d := props.Duration
		if d > 15 { d = 15 }
		args = append(args, "--duration", fmt.Sprintf("%d", d))
	}
	args = append(args, "--poll")

	// 3. 进度报告
	hook(&globals.Chunk{Content: "Video generating..."})

	// 4. 提交并轮询（视频最长 20 分钟）
	result, err := c.submitAndPoll(args, 20*time.Minute)
	if err != nil {
		return err
	}

	// 5. 返回视频 URL
	videoURL := result.VideoUrl
	if videoURL == "" {
		videoURL = result.ResultUrl
	}
	if videoURL == "" {
		videoURL = result.Result
	}
	if videoURL == "" {
		return fmt.Errorf("jimeng 未返回视频结果")
	}

	return hook(&globals.Chunk{Content: videoURL})
}

var _ adaptercommon.ImageToVideoFactory = (*CLIAdapter)(nil)
