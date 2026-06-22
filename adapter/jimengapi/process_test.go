package jimengapi

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"math"
	"testing"
)

func TestClassifyImageInputs(t *testing.T) {
	urls, base64s := classifyImageInputs([]string{
		"https://example.com/a.png",
		"http://example.com/b.jpg",
		"data:image/png;base64,QUJD",
		"  ZGVm  ",
		"",
	})

	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d (%v)", len(urls), urls)
	}
	if len(base64s) != 2 {
		t.Fatalf("expected 2 base64 inputs, got %d (%v)", len(base64s), base64s)
	}
	if base64s[0] != "QUJD" {
		t.Errorf("data URI prefix not stripped: %q", base64s[0])
	}
	if base64s[1] != "ZGVm" {
		t.Errorf("base64 not trimmed: %q", base64s[1])
	}
}

func TestResolveUpscaleResolution(t *testing.T) {
	cases := map[string]string{
		"2k": "4k",
		"4k": "4k",
		"":   "4k",
		"8k": "8k",
		"8K": "8k",
	}
	for in, want := range cases {
		if got := resolveUpscaleResolution(in); got != want {
			t.Errorf("resolveUpscaleResolution(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRatio(t *testing.T) {
	for _, s := range []string{"16:9", "16x9", "16/9"} {
		w, h, err := parseRatio(s)
		if err != nil || w != 16 || h != 9 {
			t.Errorf("parseRatio(%q) = %d,%d,%v", s, w, h, err)
		}
	}
	if _, _, err := parseRatio("abc"); err == nil {
		t.Errorf("expected error for invalid ratio")
	}
	if _, _, err := parseRatio("16:0"); err == nil {
		t.Errorf("expected error for zero ratio component")
	}
}

func TestComputeOutpaintEdges(t *testing.T) {
	// 1000x1000 → 16:9 需要左右补宽，上下为 0
	top, bottom, left, right := computeOutpaintEdges(1000, 1000, 16, 9)
	if top != 0 || bottom != 0 {
		t.Errorf("expected no vertical expansion, got top=%g bottom=%g", top, bottom)
	}
	// newW = 1000 * (16/9) ≈ 1777.78, extra/2 ≈ 388.89, frac ≈ 0.3889
	wantFrac := (1000.0*16/9 - 1000) / 2 / 1000
	if math.Abs(left-wantFrac) > 1e-6 || math.Abs(right-wantFrac) > 1e-6 {
		t.Errorf("horizontal frac = %g/%g, want %g", left, right, wantFrac)
	}

	// 1000x1000 → 9:16 需要上下补高，左右为 0
	top, bottom, left, right = computeOutpaintEdges(1000, 1000, 9, 16)
	if left != 0 || right != 0 {
		t.Errorf("expected no horizontal expansion, got left=%g right=%g", left, right)
	}
	if top <= 0 || bottom <= 0 {
		t.Errorf("expected vertical expansion, got top=%g bottom=%g", top, bottom)
	}

	// 相同比例 → 全 0
	top, bottom, left, right = computeOutpaintEdges(1000, 1000, 1, 1)
	if top != 0 || bottom != 0 || left != 0 || right != 0 {
		t.Errorf("expected no expansion for matching ratio, got %g %g %g %g", top, bottom, left, right)
	}
}

func TestDecodeImageSizeBase64(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	w, h, err := decodeImageSize(nil, []string{b64})
	if err != nil {
		t.Fatalf("decodeImageSize: %v", err)
	}
	if w != 64 || h != 48 {
		t.Errorf("decoded size = %dx%d, want 64x48", w, h)
	}
}

func TestEditAndUpscaleSpecsRegistered(t *testing.T) {
	if spec, ok := GetModelSpec("jimeng-superres"); !ok || spec.Capability != CapabilityUpscale || spec.ReqKey != "jimeng_i2i_seed3_tilesr_cvtob" {
		t.Errorf("superres spec wrong: %+v ok=%v", spec, ok)
	}
	if spec, ok := GetModelSpec("jimeng-outpaint"); !ok || spec.Capability != CapabilityOutpaint || spec.ReqKey != "jimeng_img2img_seed3_painting_edit" {
		t.Errorf("outpaint spec wrong: %+v ok=%v", spec, ok)
	}
	// 图片编辑复用 seedream 生成模型
	if spec, ok := GetModelSpec("jimeng-seedream-4.6"); !ok || spec.Capability != CapabilityGenerate {
		t.Errorf("seedream 4.6 should be generate-capable for edit reuse: %+v ok=%v", spec, ok)
	}
}
