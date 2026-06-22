package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/utils"
	"fmt"
	"math"
	"net/url"
	"path"
	"strings"
)

const (
	minSupportedRatio = 1.0 / 16.0
	maxSupportedRatio = 16.0
)

var allowedInputImageExts = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
}

func BuildSubmitTaskRequest(props *adaptercommon.ImageGenerationProps, spec ModelSpec) (SubmitTaskRequest, int, error) {
	if props == nil {
		return SubmitTaskRequest{}, 0, fmt.Errorf("jimeng-api image generation props is nil")
	}

	prompt := strings.TrimSpace(props.Prompt)
	if prompt == "" {
		return SubmitTaskRequest{}, 0, fmt.Errorf("prompt is required")
	}
	if spec.MaxPromptRunes > 0 && len([]rune(prompt)) > spec.MaxPromptRunes {
		return SubmitTaskRequest{}, 0, fmt.Errorf("prompt for %s exceeds %d characters", spec.Model, spec.MaxPromptRunes)
	}
	if len(props.Masks) > 0 {
		return SubmitTaskRequest{}, 0, fmt.Errorf("jimeng-api model %s does not support mask inputs in image generation", spec.Model)
	}

	count := props.N
	if count <= 0 {
		count = 1
	}
	if spec.MaxOutputCount > 0 && count > spec.MaxOutputCount {
		return SubmitTaskRequest{}, 0, fmt.Errorf("jimeng-api supports n up to %d for %s", spec.MaxOutputCount, spec.Model)
	}

	images, err := normalizeImageURLs(props.Images, spec)
	if err != nil {
		return SubmitTaskRequest{}, 0, err
	}

	minRatio, maxRatio, err := normalizeRatios(props.MinRatio, props.MaxRatio, spec)
	if err != nil {
		return SubmitTaskRequest{}, 0, err
	}
	if err := validateDimensions(props.Size, props.Width, props.Height, minRatio, maxRatio, spec); err != nil {
		return SubmitTaskRequest{}, 0, err
	}

	scale, err := normalizeScale(props.Scale, spec)
	if err != nil {
		return SubmitTaskRequest{}, 0, err
	}

	forceSingle := true
	if props.ForceSingle != nil {
		forceSingle = *props.ForceSingle
	}

	return SubmitTaskRequest{
		ReqKey:      spec.ReqKey,
		ImageURLs:   images,
		Prompt:      prompt,
		Size:        props.Size,
		Width:       props.Width,
		Height:      props.Height,
		Scale:       scale,
		ForceSingle: &forceSingle,
		MinRatio:    utils.ToPtr(minRatio),
		MaxRatio:    utils.ToPtr(maxRatio),
	}, count, nil
}

func normalizeImageURLs(images []string, spec ModelSpec) ([]string, error) {
	if len(images) > spec.MaxImages {
		return nil, fmt.Errorf("too many input images for %s: got %d, max %d", spec.Model, len(images), spec.MaxImages)
	}

	normalized := make([]string, 0, len(images))
	for i, imageURL := range images {
		raw := strings.TrimSpace(imageURL)
		if raw == "" {
			return nil, fmt.Errorf("image_urls[%d] is required", i)
		}
		if strings.HasPrefix(strings.ToLower(raw), "data:image/") {
			return nil, fmt.Errorf("image_urls[%d] must be an http(s) URL; data URLs are not supported by jimeng-api", i)
		}

		parsed, err := url.Parse(raw)
		if err != nil || parsed == nil || parsed.Host == "" {
			return nil, fmt.Errorf("image_urls[%d] is not a valid URL", i)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("image_urls[%d] must use http or https", i)
		}

		ext := strings.ToLower(path.Ext(parsed.Path))
		if ext != "" {
			if _, ok := allowedInputImageExts[ext]; !ok {
				return nil, fmt.Errorf("image_urls[%d] must be jpg, jpeg, or png when the URL has a file extension", i)
			}
		}
		normalized = append(normalized, raw)
	}
	return normalized, nil
}

func normalizeRatios(minRatio *float64, maxRatio *float64, spec ModelSpec) (float64, float64, error) {
	minValue := spec.DefaultMinRatio
	maxValue := spec.DefaultMaxRatio
	if minValue <= 0 {
		minValue = 1.0 / 3.0
	}
	if maxValue <= 0 {
		maxValue = 3
	}
	if minRatio != nil {
		minValue = *minRatio
	}
	if maxRatio != nil {
		maxValue = *maxRatio
	}
	if minValue < minSupportedRatio || minValue >= maxSupportedRatio {
		return 0, 0, fmt.Errorf("min_ratio for %s must be in [%g,%g)", spec.Model, minSupportedRatio, maxSupportedRatio)
	}
	if maxValue <= minSupportedRatio || maxValue > maxSupportedRatio {
		return 0, 0, fmt.Errorf("max_ratio for %s must be in (%g,%g]", spec.Model, minSupportedRatio, maxSupportedRatio)
	}
	if minValue > maxValue {
		return 0, 0, fmt.Errorf("min_ratio for %s must be less than or equal to max_ratio", spec.Model)
	}
	return minValue, maxValue, nil
}

func validateDimensions(size *int, width *int, height *int, minRatio float64, maxRatio float64, spec ModelSpec) error {
	if size != nil {
		if *size <= 0 {
			return fmt.Errorf("size for %s must be positive", spec.Model)
		}
		if err := validateArea(*size, spec); err != nil {
			return err
		}
	}

	if (width == nil) != (height == nil) {
		return fmt.Errorf("width and height for %s must be provided together", spec.Model)
	}
	if width == nil && height == nil {
		return nil
	}
	if *width <= 0 || *height <= 0 {
		return fmt.Errorf("width and height for %s must be positive", spec.Model)
	}

	area := *width * *height
	if err := validateArea(area, spec); err != nil {
		return err
	}

	ratio := float64(*width) / float64(*height)
	if ratio < minRatio || ratio > maxRatio {
		return fmt.Errorf("width/height ratio for %s must be in [%g,%g]", spec.Model, minRatio, maxRatio)
	}
	return nil
}

func validateArea(area int, spec ModelSpec) error {
	minArea := spec.MinSizeArea
	maxArea := spec.MaxSizeArea
	if minArea <= 0 {
		minArea = 1024 * 1024
	}
	if maxArea <= 0 {
		maxArea = 4096 * 4096
	}
	if area < minArea || area > maxArea {
		return fmt.Errorf("size area for %s must be in [%d,%d]", spec.Model, minArea, maxArea)
	}
	return nil
}

func normalizeScale(scale *float64, spec ModelSpec) (any, error) {
	value := spec.DefaultScale
	if scale != nil {
		value = *scale
	}

	switch spec.ScaleKind {
	case ScaleInt1To100:
		if value < 1 || value > 100 {
			return nil, fmt.Errorf("scale for %s must be in [1,100]", spec.Model)
		}
		if math.Trunc(value) != value {
			return nil, fmt.Errorf("scale for %s must be an integer in [1,100]", spec.Model)
		}
		return int(value), nil
	case ScaleFloat0To1:
		if value < 0 || value > 1 {
			return nil, fmt.Errorf("scale for %s must be in [0,1]", spec.Model)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported scale kind %s for %s", spec.ScaleKind, spec.Model)
	}
}
