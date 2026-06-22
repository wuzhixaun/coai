package jimeng

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	adaptercommon "chat/adapter/common"
	"chat/globals"
)

// ── CLI 调用核心 ──────────────────────────────────────────────

// jimengResult CLI 输出的 JSON 结构
type jimengResult struct {
	SubmitId  string `json:"submit_id"`
	GenStatus string `json:"gen_status"`
	Result    string `json:"result,omitempty"`
	ResultUrl string `json:"result_url,omitempty"`
	VideoUrl  string `json:"video_url,omitempty"`
	FailReason string `json:"fail_reason,omitempty"`
}

// runCLI 执行 jimeng CLI 命令并解析 JSON 输出
func (c *CLIAdapter) runCLI(args ...string) (*jimengResult, error) {
	cmd := exec.Command(c.cliPath, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("jimeng CLI 执行失败 (%s %s): %w", c.cliPath, strings.Join(args, " "), err)
	}

	var result jimengResult
	if err := json.Unmarshal(output, &result); err != nil {
		// 可能是纯文本输出，尝试当作 result_url
		text := strings.TrimSpace(string(output))
		if text != "" {
			return &jimengResult{GenStatus: "success", ResultUrl: text}, nil
		}
		return nil, fmt.Errorf("jimeng CLI 输出解析失败: %s", string(output))
	}
	return &result, nil
}

// saveBase64Image 将 base64 图片保存到临时文件，返回文件路径
func saveBase64Image(b64data string, prefix string) (string, error) {
	// 解码 base64（去掉可能的 data:image/xxx;base64, 前缀）
	data := b64data
	if idx := strings.Index(b64data, ","); idx > 0 {
		data = b64data[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}

	tmpDir := os.TempDir()
	filename := filepath.Join(tmpDir, fmt.Sprintf("%s_%d.png", prefix, time.Now().UnixNano()))
	if err := os.WriteFile(filename, decoded, 0644); err != nil {
		return "", err
	}
	return filename, nil
}

// submitAndPoll 提交任务并轮询等待完成
func (c *CLIAdapter) submitAndPoll(submitArgs []string, maxWait time.Duration) (*jimengResult, error) {
	// 1. 提交
	submitResult, err := c.runCLI(submitArgs...)
	if err != nil {
		return nil, err
	}

	if submitResult.GenStatus == "success" {
		return submitResult, nil
	}

	if submitResult.SubmitId == "" {
		return nil, fmt.Errorf("jimeng 提交失败：未返回 submit_id")
	}

	// 2. 轮询等待
	pollInterval := 10 * time.Second
	if maxWait == 0 {
		maxWait = 15 * time.Minute
	}

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)

		result, err := c.runCLI("query_result", "--submit_id", submitResult.SubmitId)
		if err != nil {
			continue
		}

		switch result.GenStatus {
		case "success":
			return result, nil
		case "fail":
			reason := result.FailReason
			if reason == "" {
				reason = "unknown"
			}
			return nil, fmt.Errorf("jimeng 生成失败 (submit_id=%s): %s", submitResult.SubmitId, reason)
		}
	}

	return nil, fmt.Errorf("jimeng 生成超时 (> %v)", maxWait)
}

// ── 图片编辑 (image2image) ────────────────────────────────────

func (c *CLIAdapter) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	// 1. 将 base64 图片保存为临时文件
	var imageFiles []string
	for _, b64 := range props.Images {
		path, err := saveBase64Image(b64, "jimeng_input")
		if err != nil {
			return fmt.Errorf("保存输入图片失败: %w", err)
		}
		defer os.Remove(path)
		imageFiles = append(imageFiles, path)
	}

	// 2. 构建 CLI 命令
	args := []string{
		"image2image",
		"--images", strings.Join(imageFiles, ","),
		"--prompt", props.Prompt,
		"--poll",
	}

	// 3. 提交并轮询
	result, err := c.submitAndPoll(args, 10*time.Minute)
	if err != nil {
		return err
	}

	// 4. 返回结果
	resultURL := result.ResultUrl
	if resultURL == "" {
		resultURL = result.Result
	}
	if resultURL == "" {
		return fmt.Errorf("jimeng 未返回结果")
	}

	return hook(&globals.Chunk{Content: resultURL})
}

// ── 超清放大 ──────────────────────────────────────────────────

func (c *CLIAdapter) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	path, err := saveBase64Image(props.Image, "jimeng_upscale")
	if err != nil {
		return err
	}
	defer os.Remove(path)

	resolution := props.ResolutionType
	if resolution == "" {
		resolution = "2k"
	}

	args := []string{
		"image_upscale",
		"--image", path,
		"--resolution_type", resolution,
		"--poll",
	}

	result, err := c.submitAndPoll(args, 5*time.Minute)
	if err != nil {
		return err
	}

	resultURL := result.ResultUrl
	if resultURL == "" {
		resultURL = result.Result
	}
	if resultURL == "" {
		return fmt.Errorf("jimeng 未返回放大结果")
	}

	return hook(&globals.Chunk{Content: resultURL})
}

// ── 画布扩展 (outpaint) ───────────────────────────────────────

func (c *CLIAdapter) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	// outpaint 通过 image2image + ratio 实现
	path, err := saveBase64Image(props.Image, "jimeng_outpaint")
	if err != nil {
		return err
	}
	defer os.Remove(path)

	args := []string{
		"image2image",
		"--images", path,
		"--prompt", props.Prompt,
		"--ratio", props.TargetRatio,
		"--poll",
	}

	result, err := c.submitAndPoll(args, 10*time.Minute)
	if err != nil {
		return err
	}

	resultURL := result.ResultUrl
	if resultURL == "" {
		resultURL = result.Result
	}
	if resultURL == "" {
		return fmt.Errorf("jimeng 未返回扩展结果")
	}

	return hook(&globals.Chunk{Content: resultURL})
}

// 确保编译时检查接口
var _ adaptercommon.ImageUpscaleFactory = (*CLIAdapter)(nil)
var _ adaptercommon.ImageOutpaintFactory = (*CLIAdapter)(nil)
