package photo

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw" // for high-quality resize (CatmullRom)
)

// ReadImageBase64EnsureMinSide 读取图片并保证最短边落在 [minSide, maxSide] 内，
// 返回纯 base64 PNG。即梦素材/商品提取（jimeng_i2i_extract_tiled_images /
// i2i_material_extraction）要求输入边长 1024–4096，小图（如 800×800）会被服务端
// 直接判 50500/50207。此处对过小图按比例高质量放大，过大图按比例缩小。
func ReadImageBase64EnsureMinSide(path string, minSide, maxSide int) (string, error) {
	src, err := openImage(path)
	if err != nil {
		return "", fmt.Errorf("打开图片失败: %w", err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return "", fmt.Errorf("图片尺寸非法: %dx%d", w, h)
	}

	img := src
	short := w
	if h < short {
		short = h
	}
	long := w
	if h > long {
		long = h
	}

	// 计算缩放系数：最短边补到 minSide，同时最长边不超过 maxSide。
	scale := 1.0
	if short < minSide {
		scale = float64(minSide) / float64(short)
	}
	if float64(long)*scale > float64(maxSide) {
		scale = float64(maxSide) / float64(long)
	}
	if scale != 1.0 {
		nw := int(float64(w)*scale + 0.5)
		nh := int(float64(h)*scale + 0.5)
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}
		img = resizeImage(src, nw, nh)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("编码图片失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ── 中心裁剪 ────────────────────────────────────────────────

// SmartCropCenter 智能中心裁剪：从图片中心裁剪指定大小的区域
func SmartCropCenter(inputPath, outputPath string, targetW, targetH int) error {
	src, err := openImage(inputPath)
	if err != nil {
		return fmt.Errorf("打开图片失败: %w", err)
	}

	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()

	// 计算中心裁剪区域
	left := (srcW - targetW) / 2
	top := (srcH - targetH) / 2

	if left < 0 {
		targetW = srcW
		left = 0
	}
	if top < 0 {
		targetH = srcH
		top = 0
	}
	if left+targetW > srcW {
		targetW = srcW - left
	}
	if top+targetH > srcH {
		targetH = srcH - top
	}

	// 裁剪
	cropRect := image.Rect(left, top, left+targetW, top+targetH)
	cropped := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.Draw(cropped, cropped.Bounds(), src, cropRect.Min, draw.Src)

	return savePNG(outputPath, cropped)
}

// ProcessDetailImage 细节图：中心裁剪 800x800
func ProcessDetailImage(inputPath string) (string, error) {
	outputPath := filepath.Join(ResultDir, fmt.Sprintf("detail_%s", filepath.Base(inputPath)))

	if err := ensureStorageDir(ResultDir); err != nil {
		return "", err
	}

	if err := SmartCropCenter(inputPath, outputPath, 800, 800); err != nil {
		return "", err
	}

	return "/storage/results/" + filepath.Base(outputPath), nil
}

// ── Logo 叠加 ────────────────────────────────────────────────

// CompositeLogo 在基准图上叠加 Logo
// position: top-left, top-right, bottom-left, bottom-right, center
func CompositeLogo(basePath, logoPath, outputPath, position string) error {
	base, err := openImage(basePath)
	if err != nil {
		return fmt.Errorf("打开基准图失败: %w", err)
	}
	logo, err := openImage(logoPath)
	if err != nil {
		return fmt.Errorf("打开Logo失败: %w", err)
	}

	baseW := base.Bounds().Dx()
	baseH := base.Bounds().Dy()

	// Logo 缩放：宽度 = 基准图宽度的 1/4
	logoW := baseW / 4
	if logoW < 50 {
		logoW = 50
	}
	if logoW > 500 {
		logoW = 500
	}

	logoBounds := logo.Bounds()
	origLogoW := logoBounds.Dx()
	origLogoH := logoBounds.Dy()
	logoH := int(float64(origLogoH) * float64(logoW) / float64(origLogoW))

	resizedLogo := resizeImage(logo, logoW, logoH)

	// 计算位置 (留 20px 边距)
	margin := 20
	var logoX, logoY int

	switch position {
	case "top-left":
		logoX = margin
		logoY = margin
	case "top-right":
		logoX = baseW - logoW - margin
		logoY = margin
	case "bottom-left":
		logoX = margin
		logoY = baseH - logoH - margin
	case "bottom-right":
		logoX = baseW - logoW - margin
		logoY = baseH - logoH - margin
	case "center":
		logoX = (baseW - logoW) / 2
		logoY = (baseH - logoH) / 2
	default:
		logoX = baseW - logoW - margin
		logoY = baseH - logoH - margin
	}

	// 合成：先画基准图，再画 Logo
	result := image.NewRGBA(base.Bounds())
	draw.Draw(result, result.Bounds(), base, image.Point{}, draw.Src)
	draw.Draw(result,
		image.Rect(logoX, logoY, logoX+logoW, logoY+logoH),
		resizedLogo, image.Point{},
		draw.Over,
	)

	return savePNG(outputPath, result)
}

// ProcessLogoCustom Logo定制入口
func ProcessLogoCustom(basePath, logoPath, position string) (string, error) {
	if err := ensureStorageDir(ResultDir); err != nil {
		return "", err
	}

	outputName := fmt.Sprintf("logo_%s", filepath.Base(basePath))
	outputPath := filepath.Join(ResultDir, outputName)

	if err := CompositeLogo(basePath, logoPath, outputPath, position); err != nil {
		return "", err
	}

	return "/storage/results/" + filepath.Base(outputPath), nil
}

// ── 图片工具 ────────────────────────────────────────────────

// openImage 打开并解码图片文件
func openImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, err
}

// savePNG 保存为 PNG 文件
func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// resizeImage 使用高质量算法缩放图片到指定尺寸
func resizeImage(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
