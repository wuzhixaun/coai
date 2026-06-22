package photo

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// createTestImage 创建一个简单的测试图片
func createTestImage(path string, w, h int, c color.Color) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func TestEnsureStorageDir(t *testing.T) {
	if err := ensureStorageDir(ResultDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ResultDir); os.IsNotExist(err) {
		t.Fatal("ResultDir was not created")
	}
}

func TestSmartCropCenter(t *testing.T) {
	// 创建 1200x800 测试图片
	inputPath := filepath.Join(UploadDir, "test_crop_input.png")
	if err := ensureStorageDir(UploadDir); err != nil {
		t.Fatal(err)
	}
	if err := createTestImage(inputPath, 1200, 800, color.RGBA{100, 150, 200, 255}); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(inputPath)

	outputPath := filepath.Join(ResultDir, "test_crop_output.png")
	defer os.Remove(outputPath)

	if err := SmartCropCenter(inputPath, outputPath, 800, 800); err != nil {
		t.Fatal(err)
	}

	// 验证输出
	img, err := openImage(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 800 || bounds.Dy() != 800 {
		t.Errorf("expected 800x800, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	t.Logf("Crop result: %dx%d", bounds.Dx(), bounds.Dy())
}

func TestCompositeLogo(t *testing.T) {
	// 基准图 800x600
	basePath := filepath.Join(UploadDir, "test_base.png")
	createTestImage(basePath, 800, 600, color.RGBA{255, 255, 255, 255})
	defer os.Remove(basePath)

	// Logo 200x100 (红色)
	logoPath := filepath.Join(UploadDir, "test_logo.png")
	createTestImage(logoPath, 200, 100, color.RGBA{255, 0, 0, 200})
	defer os.Remove(logoPath)

	outputPath := filepath.Join(ResultDir, "test_composite.png")
	defer os.Remove(outputPath)

	// 测试右下角叠加
	if err := CompositeLogo(basePath, logoPath, outputPath, "bottom-right"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("output file not created")
	}

	t.Logf("Composite result: %s", outputPath)
}

func TestAllPositions(t *testing.T) {
	positions := []string{"top-left", "top-right", "bottom-left", "bottom-right", "center"}

	basePath := filepath.Join(UploadDir, "test_base2.png")
	createTestImage(basePath, 600, 400, color.RGBA{200, 200, 200, 255})
	defer os.Remove(basePath)

	logoPath := filepath.Join(UploadDir, "test_logo2.png")
	createTestImage(logoPath, 100, 50, color.RGBA{0, 255, 0, 200})
	defer os.Remove(logoPath)

	for _, pos := range positions {
		outputPath := filepath.Join(ResultDir, "test_"+pos+".png")
		defer os.Remove(outputPath)

		err := CompositeLogo(basePath, logoPath, outputPath, pos)
		if err != nil {
			t.Errorf("CompositeLogo %s failed: %v", pos, err)
			continue
		}

		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Errorf("output not created for position: %s", pos)
		} else {
			t.Logf("Position %s: OK", pos)
		}
	}
}

func TestProcessDetailImage(t *testing.T) {
	inputPath := filepath.Join(UploadDir, "test_detail.png")
	createTestImage(inputPath, 1200, 900, color.RGBA{50, 100, 150, 255})
	defer os.Remove(inputPath)

	url, err := ProcessDetailImage(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Detail image URL: %s", url)

	// 验证文件存在
	path := filepath.Join(ResultDir, filepath.Base(url))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("detail image file not created")
	}
	defer os.Remove(path)
}
