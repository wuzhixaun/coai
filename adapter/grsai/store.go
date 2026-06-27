package grsai

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

func resultFilename(source, ext string) string {
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
	return fmt.Sprintf("grsai_%d_%s_%s%s", time.Now().UnixNano(), hash, utils.GenerateChar(6), ext)
}

// storeRemoteURL 下载 remote 并落地到 storage/results，返回本地公开 URL。
// imageExt=true 时按图片扩展名回退 .png；否则按视频回退 .mp4。
func (c *Generator) storeRemoteURL(remote string, imageExt bool) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", fmt.Errorf("empty result url")
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if imageExt {
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		default:
			ext = ".png"
		}
	} else {
		switch ext {
		case ".mp4", ".webm", ".mov":
		default:
			ext = ".mp4"
		}
	}

	if err := os.MkdirAll(globals.StorageResultDir, 0755); err != nil {
		return "", err
	}
	filename := resultFilename(remote, ext)
	savePath := filepath.Join(globals.StorageResultDir, filename)

	req, err := http.NewRequest(http.MethodGet, remote, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download grsai result failed: http %d", resp.StatusCode)
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
	return globals.ResultPublicURL(filename), nil
}
