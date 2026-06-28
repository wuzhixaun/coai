package ark

import (
	"chat/globals"
	"chat/utils"
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

// storeImageURL 下载图片结果并存到 storage/results，返回本地公开路径。
// 方舟图片 URL 是 24h 过期的 TOS 签名地址，必须落地：既保证持久可访问，
// 也让「一键成套」链式步骤能把上一步结果当本地文件读取（否则带 ?X-Tos-... 的
// 远程 URL 会被当成本地文件名，触发 "file name too long"）。
func (g *Generator) storeImageURL(imageURL string) (string, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return "", fmt.Errorf("empty image url")
	}
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
	default:
		ext = ".png"
	}

	if err := os.MkdirAll(globals.StorageResultDir, 0755); err != nil {
		return "", err
	}
	filename := resultFilename(imageURL, ext)
	savePath := filepath.Join(globals.StorageResultDir, filename)

	req, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download ark image failed: http %d", resp.StatusCode)
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

// storeImageB64 把 base64 图片解码后落地，返回本地公开路径。
func (g *Generator) storeImageB64(b64 string) (string, error) {
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return "", fmt.Errorf("empty base64 image")
	}
	if i := strings.Index(b64, ","); strings.HasPrefix(b64, "data:") && i >= 0 {
		b64 = b64[i+1:]
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(globals.StorageResultDir, 0755); err != nil {
		return "", err
	}
	filename := resultFilename(b64, ".png")
	savePath := filepath.Join(globals.StorageResultDir, filename)
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return "", err
	}
	return publicResultURL(filename), nil
}

// storeVideoURL 下载视频结果并存到 storage/results，返回本地公开路径。
// 方舟返回的视频 URL 为 24h 有效的临时地址，必须落地后再回推，
// 否则下载接口会当作本地文件找不到、退化成 octet-stream。
func (g *Generator) storeVideoURL(videoURL string) (string, error) {
	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		return "", fmt.Errorf("empty video url")
	}
	parsed, err := url.Parse(videoURL)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	switch ext {
	case ".mp4", ".webm", ".mov":
	default:
		ext = ".mp4"
	}

	if err := os.MkdirAll(globals.StorageResultDir, 0755); err != nil {
		return "", err
	}
	filename := resultFilename(videoURL, ext)
	savePath := filepath.Join(globals.StorageResultDir, filename)

	req, err := http.NewRequest(http.MethodGet, videoURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download ark video failed: http %d", resp.StatusCode)
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

func resultFilename(source, ext string) string {
	if ext == "" || ext == "." {
		ext = ".mp4"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	hash := utils.Md5Encrypt(source)
	if len(hash) > 10 {
		hash = hash[:10]
	}
	// 追加随机串，消除「同源 + 同纳秒时间戳」高并发下的文件名碰撞风险。
	return fmt.Sprintf("ark_%d_%s_%s%s", time.Now().UnixNano(), hash, utils.GenerateChar(6), ext)
}

func publicResultURL(filename string) string {
	return globals.ResultPublicURL(filename)
}
