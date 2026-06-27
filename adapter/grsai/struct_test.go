package grsai

import (
	adaptercommon "chat/adapter/common"
	"testing"
)

func TestGeneratorImplementsInterfaces(t *testing.T) {
	var _ adaptercommon.ImageGenerationFactory = (*Generator)(nil)
	var _ adaptercommon.ImageEditFactory = (*Generator)(nil)
	var _ adaptercommon.ImageUpscaleFactory = (*Generator)(nil)
	var _ adaptercommon.ImageOutpaintFactory = (*Generator)(nil)
	var _ adaptercommon.ImageToVideoFactory = (*Generator)(nil)

	c := NewImageGeneratorFromConfig(fakeConfig{endpoint: "https://grsaiapi.com", secret: "k"})
	if c == nil {
		t.Fatal("nil generator factory")
	}
	if NewImageProcessorFromConfig(fakeConfig{secret: "k"}) == nil {
		t.Fatal("nil processor factory")
	}
}
