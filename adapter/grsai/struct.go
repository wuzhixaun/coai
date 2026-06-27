package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
)

// NewImageGeneratorFromConfig 供生图工厂表使用。
func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory {
	return newGenerator(conf)
}

// NewImageProcessorFromConfig 供图片处理工厂表使用；返回实例同时实现编辑/超分/扩图/视频接口，
// adapter 层按需断言。
func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory {
	return newGenerator(conf)
}

var (
	_ adaptercommon.Factory                = (*Generator)(nil)
	_ adaptercommon.ImageGenerationFactory = (*Generator)(nil)
	_ adaptercommon.ImageEditFactory       = (*Generator)(nil)
	_ adaptercommon.ImageUpscaleFactory    = (*Generator)(nil)
	_ adaptercommon.ImageOutpaintFactory   = (*Generator)(nil)
	_ adaptercommon.ImageToVideoFactory    = (*Generator)(nil)
)
