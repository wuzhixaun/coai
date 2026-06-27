package openai

import (
	"chat/globals"
	"chat/utils"
	"strings"
)

// normalizeInputImage 把输入图规整为 OpenAI image_url 可接受的形式：
// http(s) 与 data: 原样返回；裸 base64 补 data:image/png;base64, 前缀；空白返回 ""。
func normalizeInputImage(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "data:") {
		return s
	}
	return "data:image/png;base64," + s
}

// normalizeImages 规整并丢弃空白项。
func normalizeImages(images []string) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		if n := normalizeInputImage(img); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// buildUserMessage 构造一条多模态 user 消息：text + 若干 image_url。
func buildUserMessage(prompt string, images []string) Message {
	p := prompt
	contents := MessageContents{{Type: "text", Text: &p}}
	for _, u := range normalizeImages(images) {
		url := u
		contents = append(contents, MessageContent{
			Type:     "image_url",
			ImageUrl: &ImageUrl{Url: url},
		})
	}
	return Message{Role: globals.User, Content: contents}
}

// extractImages 从响应文本抠出图片（http 图片 URL 与 base64 data URI）。
func extractImages(content string) []string {
	_, images := utils.ExtractImages(content, true)
	return images
}

// extractVideoURLs 从响应文本抠出视频链接（.mp4/.webm/.mov，允许带 query）。
func extractVideoURLs(content string) []string {
	var out []string
	for _, u := range utils.ExtractUrls(content) {
		clean := strings.TrimRight(u, ").,\"'")
		p := strings.ToLower(clean)
		if i := strings.IndexAny(p, "?#"); i >= 0 {
			p = p[:i]
		}
		if strings.HasSuffix(p, ".mp4") || strings.HasSuffix(p, ".webm") || strings.HasSuffix(p, ".mov") {
			out = append(out, clean)
		}
	}
	return out
}
