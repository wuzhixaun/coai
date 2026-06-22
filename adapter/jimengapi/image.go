package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const resultDir = "storage/results"

func (c *ImageGenerator) CreateImageGenerationRequest(props *adaptercommon.ImageGenerationProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok {
		return fmt.Errorf("jimeng-api unsupported model: %s", props.Model)
	}
	if spec.Capability != CapabilityGenerate {
		return fmt.Errorf("jimeng-api model %s is not an image generation model", props.Model)
	}

	submitReq, count, err := BuildSubmitTaskRequest(props, spec)
	if err != nil {
		return err
	}

	for i := 0; i < count; i++ {
		ctx := context.Background()
		submitResp, err := c.Submit(ctx, submitReq)
		if err != nil {
			return err
		}

		resultResp, err := c.Poll(ctx, spec.ReqKey, submitResp.Data.TaskID, 10*time.Minute, 10*time.Second)
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
	}
	return nil
}

func (c *ImageGenerator) storeResults(data *TaskPayload) ([]string, error) {
	results := make([]string, 0, len(data.ImageURLs)+len(data.BinaryDataBase64))

	for _, imageURL := range data.ImageURLs {
		stored, err := c.storeImageURL(imageURL)
		if err != nil {
			return nil, err
		}
		results = append(results, stored)
	}

	for _, b64 := range data.BinaryDataBase64 {
		stored, err := storeBase64Image(b64)
		if err != nil {
			return nil, err
		}
		results = append(results, stored)
	}

	return results, nil
}

func resultFilename(source string, ext string) string {
	if ext == "" || ext == "." {
		ext = ".png"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	hash := utils.Md5Encrypt(source)
	if len(hash) > 10 {
		hash = hash[:10]
	}
	return fmt.Sprintf("jimeng_%d_%s%s", time.Now().UnixNano(), hash, ext)
}

func publicResultURL(filename string) string {
	return "/storage/results/" + filename
}

func (c *ImageGenerator) storeImageURL(imageURL string) (string, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return "", fmt.Errorf("empty image url")
	}

	parsed, err := url.Parse(imageURL)
	if err != nil {
		return "", err
	}

	// 火山 TOS 图片 URL 常以 ~tplv 模板或 `.image` 结尾，path.Ext 会得到非图片扩展名，
	// 这里只保留已知图片扩展名，其余统一回退为 .png，避免落地文件无法按图片渲染。
	ext := strings.ToLower(path.Ext(parsed.Path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
	default:
		ext = ".png"
	}

	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return "", err
	}

	filename := resultFilename(imageURL, ext)
	savePath := filepath.Join(resultDir, filename)

	req, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download jimeng result failed: http %d", resp.StatusCode)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(savePath)
		return "", err
	}

	return publicResultURL(filename), nil
}

func storeBase64Image(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty base64 image")
	}

	ext := ".png"
	if strings.HasPrefix(raw, "data:image/") {
		head := strings.SplitN(strings.TrimPrefix(raw, "data:image/"), ";", 2)
		if len(head) > 0 && head[0] != "" {
			ext = "." + head[0]
		}
		if idx := strings.Index(raw, ","); idx >= 0 {
			raw = raw[idx+1:]
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("decode jimeng base64 image failed: %w", err)
	}

	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return "", err
	}

	filename := resultFilename(raw, ext)
	savePath := filepath.Join(resultDir, filename)
	if err := os.WriteFile(savePath, decoded, 0644); err != nil {
		return "", err
	}
	return publicResultURL(filename), nil
}
