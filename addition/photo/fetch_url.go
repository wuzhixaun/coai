package photo

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ── URL / 商品链接抓图驱动（P5.3）─────────────────────────────
//
// 贴商品链接 → 抓取主图 → 落库为可用图片，进而进入成套工作流。
// 支持：① 直链图片 URL；② 页面 og:image 元标签。
// 局限：需要登录态或纯前端 JS 渲染的页面无法抓到（电商详情页常见），属预期内，给出明确报错。

const fetchMaxHTMLBytes = 2 << 20  // 解析 HTML 最多读 2MB
const fetchMaxImageBytes = MaxUploadSize

var ogImageRe = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`)
var ogImageReAlt = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image["']`)

type fetchURLRequest struct {
	Url string `json:"url" binding:"required"`
}

// isBlockedHost 基础 SSRF 防护：拒绝本机/内网地址，避免服务端抓取被用于探测内网。
func isBlockedHost(host string) bool {
	host = strings.ToLower(host)
	if host == "localhost" {
		return true
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return true // 解析失败直接拒绝
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}

func httpGetBrowser(rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,image/*,*/*")
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

// extFromContentType 由 content-type 推断扩展名（仅允许的图片类型）。
func extFromContentType(ct string) string {
	switch {
	case strings.HasPrefix(ct, "image/png"):
		return ".png"
	case strings.HasPrefix(ct, "image/jpeg"), strings.HasPrefix(ct, "image/jpg"):
		return ".jpg"
	case strings.HasPrefix(ct, "image/webp"):
		return ".webp"
	case strings.HasPrefix(ct, "image/bmp"):
		return ".bmp"
	case strings.HasPrefix(ct, "image/tiff"):
		return ".tiff"
	default:
		return ""
	}
}

// resolveImageURL 取直链图片本身或页面的 og:image。
func resolveImageURL(pageURL string) (string, error) {
	resp, err := httpGetBrowser(pageURL)
	if err != nil {
		return "", fmt.Errorf("抓取链接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("链接返回 %d，可能需要登录或已失效", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "image/") {
		return pageURL, nil // 直链图片
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, fetchMaxHTMLBytes))
	for _, re := range []*regexp.Regexp{ogImageRe, ogImageReAlt} {
		if m := re.FindSubmatch(body); m != nil {
			return absoluteURL(pageURL, string(m[1]))
		}
	}
	return "", fmt.Errorf("未能从链接解析到主图（页面可能需要登录或由 JS 渲染，请改用直链图片地址）")
}

func absoluteURL(base, ref string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	r, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(r).String(), nil
}

func FetchURLAPI(c *gin.Context) {
	var req fetchURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请提供有效的 url"})
		return
	}

	u, err := url.Parse(strings.TrimSpace(req.Url))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "仅支持 http/https 链接"})
		return
	}
	if isBlockedHost(u.Hostname()) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "不允许抓取该地址"})
		return
	}

	imgURL, err := resolveImageURL(req.Url)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "message": err.Error()})
		return
	}
	// og:image 可能也指向受限主机，二次校验
	if iu, e := url.Parse(imgURL); e == nil && isBlockedHost(iu.Hostname()) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "图片地址不被允许"})
		return
	}

	resp, err := httpGetBrowser(imgURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "message": "下载图片失败"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "message": fmt.Sprintf("下载图片返回 %d", resp.StatusCode)})
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, fetchMaxImageBytes+1))
	if err != nil || len(data) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "message": "图片内容为空或读取失败"})
		return
	}
	if int64(len(data)) > fetchMaxImageBytes {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "图片超过 50MB"})
		return
	}

	// 扩展名：优先 content-type，其次 URL 路径
	ext := extFromContentType(resp.Header.Get("Content-Type"))
	if ext == "" {
		if e := strings.ToLower(path.Ext(imgURL)); AllowedExtensions[e] {
			ext = e
		}
	}
	if ext == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "无法识别的图片格式"})
		return
	}

	filename := "url_" + generateImageID() + ext
	info, err := SaveImageBytes(getDBFromCtx(c), getUserID(c), filename, data, "url")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}
