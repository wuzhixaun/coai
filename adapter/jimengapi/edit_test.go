package jimengapi

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"strings"
	"testing"
)

func TestSetPromptField(t *testing.T) {
	cases := []struct {
		field string
		check func(SubmitTaskRequest) bool
	}{
		{"image_edit_prompt", func(r SubmitTaskRequest) bool { return r.ImageEditPrompt != nil && r.EditPrompt == nil && r.Prompt == "" }},
		{"edit_prompt", func(r SubmitTaskRequest) bool { return r.EditPrompt != nil && r.ImageEditPrompt == nil && r.Prompt == "" }},
		{"prompt", func(r SubmitTaskRequest) bool { return r.Prompt == "x" && r.ImageEditPrompt == nil && r.EditPrompt == nil }},
		{"", func(r SubmitTaskRequest) bool { return r.Prompt == "x" }},
	}
	for _, c := range cases {
		var req SubmitTaskRequest
		setPromptField(&req, ModelSpec{PromptField: c.field}, "x")
		if !c.check(req) {
			t.Errorf("setPromptField(%q) wrong: %+v", c.field, req)
		}
	}
}

func TestExtractAndInpaintSpecsRegistered(t *testing.T) {
	for model, wantReqKey := range map[string]string{
		"jimeng-inpaint":          "jimeng_image2image_dream_inpaint",
		"jimeng-material-extract": "i2i_material_extraction",
		"jimeng-product-extract":  "jimeng_i2i_extract_tiled_images",
	} {
		spec, ok := GetModelSpec(model)
		if !ok || spec.ReqKey != wantReqKey {
			t.Errorf("%s spec wrong: %+v ok=%v", model, spec, ok)
		}
	}
	if s, _ := GetModelSpec("jimeng-material-extract"); s.PromptField != "image_edit_prompt" {
		t.Errorf("material-extract PromptField = %q", s.PromptField)
	}
	if s, _ := GetModelSpec("jimeng-product-extract"); s.PromptField != "edit_prompt" {
		t.Errorf("product-extract PromptField = %q", s.PromptField)
	}
}

// CreateImageEditRequest 的校验分支在发起网络请求前返回，可离线测试。
func TestEditRequestValidation(t *testing.T) {
	gen := &ImageGenerator{} // 仅触发校验错误，不会走到网络

	mustErr := func(name string, props *adaptercommon.ImageEditProps, want string) {
		err := gen.CreateImageEditRequest(props, func(*globals.Chunk) error { return nil })
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got err=%v, want contains %q", name, err, want)
		}
	}

	mustErr("inpaint needs 2 images",
		&adaptercommon.ImageEditProps{Model: "jimeng-inpaint", Images: []string{"aaa"}, Prompt: "删除"},
		"exactly 2 images")
	mustErr("extract needs prompt",
		&adaptercommon.ImageEditProps{Model: "jimeng-material-extract", Images: []string{"aaa"}},
		"category prompt")
	mustErr("extract needs 1 image",
		&adaptercommon.ImageEditProps{Model: "jimeng-product-extract", Images: []string{"aaa", "bbb"}, Prompt: "服装"},
		"exactly 1 input image")
	mustErr("generate needs prompt",
		&adaptercommon.ImageEditProps{Model: "jimeng-seedream-4.6", Images: []string{"aaa"}},
		"requires a prompt")
	mustErr("unknown model",
		&adaptercommon.ImageEditProps{Model: "nope", Images: []string{"aaa"}, Prompt: "x"},
		"unsupported model")
}
