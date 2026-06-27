package openai

import "testing"

func TestNormalizeInputImage(t *testing.T) {
	cases := map[string]string{
		"  https://x.com/a.png ":    "https://x.com/a.png",
		"data:image/png;base64,AAA": "data:image/png;base64,AAA",
		"AAABBB":                    "data:image/png;base64,AAABBB",
		"   ":                       "",
	}
	for in, want := range cases {
		if got := normalizeInputImage(in); got != want {
			t.Fatalf("normalizeInputImage(%q)=%q want %q", in, got, want)
		}
	}
}

func TestBuildUserMessage(t *testing.T) {
	msg := buildUserMessage("画只猫", []string{"https://x.com/a.png", "  "})
	if msg.Role != "user" {
		t.Fatalf("role=%q", msg.Role)
	}
	if len(msg.Content) != 2 { // 1 text + 1 image（空白图被丢弃）
		t.Fatalf("content len=%d want 2", len(msg.Content))
	}
	if msg.Content[0].Type != "text" || msg.Content[0].Text == nil || *msg.Content[0].Text != "画只猫" {
		t.Fatalf("text block wrong: %+v", msg.Content[0])
	}
	if msg.Content[1].Type != "image_url" || msg.Content[1].ImageUrl == nil || msg.Content[1].ImageUrl.Url != "https://x.com/a.png" {
		t.Fatalf("image block wrong: %+v", msg.Content[1])
	}
}

func TestExtractImages(t *testing.T) {
	// base64 data URI（nano-banana 风格）
	b64 := "![image](data:image/jpeg;base64,/9j/AAAB+/=)"
	got := extractImages(b64)
	if len(got) != 1 || got[0] != "data:image/jpeg;base64,/9j/AAAB+/=" {
		t.Fatalf("base64 extract=%v", got)
	}
	// http 图片 URL
	httpc := "done ![image](https://file.x.com/a.png)"
	got = extractImages(httpc)
	if len(got) != 1 || got[0] != "https://file.x.com/a.png" {
		t.Fatalf("http extract=%v", got)
	}
	// 无图
	if got = extractImages("纯文本没有图"); len(got) != 0 {
		t.Fatalf("expected no images, got %v", got)
	}
}

func TestExtractVideoURLs(t *testing.T) {
	content := "生成完成 ![video](https://file.x.com/v.mp4) 另有 https://file.x.com/b.webm?sig=1"
	got := extractVideoURLs(content)
	if len(got) != 2 {
		t.Fatalf("video extract=%v want 2", got)
	}
	// 图片 URL 不应被当成视频
	if v := extractVideoURLs("![image](https://x.com/a.png)"); len(v) != 0 {
		t.Fatalf("png should not be video: %v", v)
	}
}
