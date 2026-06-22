package jimengapi

import (
	"bytes"
	"chat/globals"
	"chat/utils"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg" // 注册 jpeg 解码器，供 image.DecodeConfig 使用
	_ "image/png"  // 注册 png 解码器
	"strconv"
	"strings"
	"time"
)

// classifyImageInputs 把 Photo / OpenAI 传入的图片列表区分为 http(s) URL 与原始 base64。
// 官方 submit 支持 image_urls 与 binary_data_base64 两种输入，本地文件走 base64。
func classifyImageInputs(images []string) (urls []string, base64s []string) {
	for _, img := range images {
		s := strings.TrimSpace(img)
		if s == "" {
			continue
		}
		lower := strings.ToLower(s)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			urls = append(urls, s)
			continue
		}
		base64s = append(base64s, rawBase64(s))
	}
	return urls, base64s
}

// rawBase64 去掉 data URI 前缀，返回纯 base64 字符串（火山 binary_data_base64 不接受 data: 前缀）。
func rawBase64(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "data:") {
		if idx := strings.Index(s, ","); idx >= 0 {
			return strings.TrimSpace(s[idx+1:])
		}
	}
	return s
}

// runTaskAndEmit 提交任务、轮询、转存结果，并以 Markdown 图片形式推送到 hook，
// 与 dreamina / 生成路径保持一致的输出约定。
func (c *ImageGenerator) runTaskAndEmit(reqKey string, req SubmitTaskRequest, hook globals.Hook) error {
	ctx := context.Background()

	submitResp, err := c.Submit(ctx, req)
	if err != nil {
		return err
	}

	resultResp, err := c.Poll(ctx, reqKey, submitResp.Data.TaskID, 10*time.Minute, 10*time.Second)
	if err != nil {
		return err
	}
	if resultResp.Data == nil {
		return fmt.Errorf("jimeng task finished without data (task_id=%s)", submitResp.Data.TaskID)
	}

	stored, err := c.storeResults(resultResp.Data)
	if err != nil {
		return err
	}
	if len(stored) == 0 {
		return fmt.Errorf("jimeng task finished without image result (task_id=%s)", submitResp.Data.TaskID)
	}

	for _, imageURL := range stored {
		if err := hook(&globals.Chunk{Content: utils.GetImageMarkdown(imageURL)}); err != nil {
			return err
		}
	}
	return nil
}

// decodeImageSize 解析输入图片的宽高，优先使用 base64，其次远程 URL。
func decodeImageSize(urls []string, base64s []string) (int, int, error) {
	if len(base64s) > 0 {
		data, err := base64.StdEncoding.DecodeString(base64s[0])
		if err != nil {
			return 0, 0, fmt.Errorf("decode input image base64 failed: %w", err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return 0, 0, fmt.Errorf("decode input image config failed: %w", err)
		}
		return cfg.Width, cfg.Height, nil
	}
	if len(urls) > 0 {
		img, err := utils.NewImage(urls[0])
		if err != nil {
			return 0, 0, err
		}
		if img == nil || img.Object == nil {
			return 0, 0, fmt.Errorf("cannot decode input image from url")
		}
		return img.GetWidth(), img.GetHeight(), nil
	}
	return 0, 0, fmt.Errorf("no input image to decode")
}

// parseRatio 解析 "16:9" / "16x9" / "16/9" 形式的目标比例。
func parseRatio(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	for _, sep := range []string{":", "x", "X", "/", "*"} {
		if strings.Contains(s, sep) {
			parts := strings.SplitN(s, sep, 2)
			w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
				return 0, 0, fmt.Errorf("invalid target ratio: %s", s)
			}
			return w, h, nil
		}
	}
	return 0, 0, fmt.Errorf("invalid target ratio: %s", s)
}

// computeOutpaintEdges 根据原图宽高与目标比例计算只扩不裁的四向扩展比例 [0,1]。
func computeOutpaintEdges(w, h, targetW, targetH int) (top, bottom, left, right float64) {
	if w <= 0 || h <= 0 || targetW <= 0 || targetH <= 0 {
		return 0, 0, 0, 0
	}
	target := float64(targetW) / float64(targetH)
	current := float64(w) / float64(h)
	const eps = 1e-6

	if target > current+eps {
		// 目标更宽：左右补宽
		newW := float64(h) * target
		frac := (newW - float64(w)) / 2 / float64(w)
		left, right = frac, frac
	} else if target < current-eps {
		// 目标更高：上下补高
		newH := float64(w) / target
		frac := (newH - float64(h)) / 2 / float64(h)
		top, bottom = frac, frac
	}

	clamp01 := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	return clamp01(top), clamp01(bottom), clamp01(left), clamp01(right)
}
