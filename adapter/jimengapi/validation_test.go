package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"strings"
	"testing"
)

func testIntPtr(v int) *int {
	return &v
}

func testFloatPtr(v float64) *float64 {
	return &v
}

func testProps(model string) *adaptercommon.ImageGenerationProps {
	return &adaptercommon.ImageGenerationProps{
		Model:  model,
		Prompt: "一只戴着蓝色围巾的橘猫，电影感光线",
	}
}

func mustSpec(t *testing.T, model string) ModelSpec {
	t.Helper()
	spec, ok := GetModelSpec(model)
	if !ok {
		t.Fatalf("%s spec missing", model)
	}
	return spec
}

func TestSeedream40ModelSpec(t *testing.T) {
	spec := mustSpec(t, "jimeng-seedream-4.0")
	if spec.ReqKey != "jimeng_t2i_v40" {
		t.Fatalf("unexpected req_key: %s", spec.ReqKey)
	}
	if spec.ScaleKind != ScaleFloat0To1 {
		t.Fatalf("unexpected scale kind: %s", spec.ScaleKind)
	}
	if spec.DefaultScale != 0.5 {
		t.Fatalf("unexpected default scale: %g", spec.DefaultScale)
	}
}

func TestBuildSubmitTaskRequestScaleNormalization(t *testing.T) {
	spec46 := mustSpec(t, "jimeng-seedream-4.6")
	props46 := testProps(spec46.Model)
	props46.Scale = testFloatPtr(75)
	req46, _, err := BuildSubmitTaskRequest(props46, spec46)
	if err != nil {
		t.Fatal(err)
	}
	scale46, ok := req46.Scale.(int)
	if !ok || scale46 != 75 {
		t.Fatalf("expected int scale 75, got %#v", req46.Scale)
	}

	spec40 := mustSpec(t, "jimeng-seedream-4.0")
	props40 := testProps(spec40.Model)
	props40.Scale = testFloatPtr(0.75)
	req40, _, err := BuildSubmitTaskRequest(props40, spec40)
	if err != nil {
		t.Fatal(err)
	}
	scale40, ok := req40.Scale.(float64)
	if !ok || scale40 != 0.75 {
		t.Fatalf("expected float64 scale 0.75, got %#v", req40.Scale)
	}

	props46.Scale = testFloatPtr(0.5)
	if _, _, err := BuildSubmitTaskRequest(props46, spec46); err == nil {
		t.Fatal("expected non-integer 4.6 scale to fail")
	}
}

func TestBuildSubmitTaskRequestValidationFailures(t *testing.T) {
	spec := mustSpec(t, "jimeng-seedream-4.6")

	cases := []struct {
		name  string
		props *adaptercommon.ImageGenerationProps
	}{
		{
			name: "prompt too long",
			props: &adaptercommon.ImageGenerationProps{
				Model:  spec.Model,
				Prompt: strings.Repeat("猫", spec.MaxPromptRunes+1),
			},
		},
		{
			name: "width without height",
			props: &adaptercommon.ImageGenerationProps{
				Model:  spec.Model,
				Prompt: "猫",
				Width:  testIntPtr(1024),
			},
		},
		{
			name: "area too small",
			props: &adaptercommon.ImageGenerationProps{
				Model:  spec.Model,
				Prompt: "猫",
				Width:  testIntPtr(512),
				Height: testIntPtr(512),
			},
		},
		{
			name: "data url image",
			props: &adaptercommon.ImageGenerationProps{
				Model:  spec.Model,
				Prompt: "猫",
				Images: []string{"data:image/png;base64,AAAA"},
			},
		},
		{
			name: "unsupported image extension",
			props: &adaptercommon.ImageGenerationProps{
				Model:  spec.Model,
				Prompt: "猫",
				Images: []string{"https://example.com/cat.gif"},
			},
		},
		{
			name: "too many outputs",
			props: &adaptercommon.ImageGenerationProps{
				Model:  spec.Model,
				Prompt: "猫",
				N:      spec.MaxOutputCount + 1,
			},
		},
		{
			name: "mask unsupported",
			props: &adaptercommon.ImageGenerationProps{
				Model:  spec.Model,
				Prompt: "猫",
				Masks:  []string{"https://example.com/mask.png"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := BuildSubmitTaskRequest(tc.props, spec); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBuildSubmitTaskRequestValidDimensionsAndSignedURL(t *testing.T) {
	spec := mustSpec(t, "jimeng-seedream-4.6")
	props := testProps(spec.Model)
	props.Width = testIntPtr(1024)
	props.Height = testIntPtr(1024)
	props.Images = []string{"https://example.com/signed-image?x=1"}

	req, count, err := BuildSubmitTaskRequest(props, spec)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unexpected count: %d", count)
	}
	if req.Width == nil || *req.Width != 1024 || req.Height == nil || *req.Height != 1024 {
		t.Fatalf("unexpected dimensions: %#v %#v", req.Width, req.Height)
	}
	if len(req.ImageURLs) != 1 {
		t.Fatalf("unexpected image urls: %#v", req.ImageURLs)
	}
}
