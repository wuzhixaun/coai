package ark

import (
	"chat/globals"
	"chat/utils"
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
