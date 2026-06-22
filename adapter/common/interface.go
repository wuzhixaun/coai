package adaptercommon

import (
	"chat/globals"
)

type Factory interface {
	CreateStreamChatRequest(props *ChatProps, hook globals.Hook) error
}

type VideoFactory interface {
	CreateVideoRequest(props *VideoProps, hook globals.Hook) error
}

type FactoryCreator func(globals.ChannelConfig) Factory

// ── 图片处理接口（电商图片处理专用） ────────────────────────

// ImageEditFactory 图片编辑（图生图/换色/擦除/场景生成等）
type ImageEditFactory interface {
	CreateImageEditRequest(props *ImageEditProps, hook globals.Hook) error
}

// ImageGenerationFactory 通用图片生成（文生图/多图组合生成等）
type ImageGenerationFactory interface {
	CreateImageGenerationRequest(props *ImageGenerationProps, hook globals.Hook) error
}

// ImageUpscaleFactory 图片超清放大
type ImageUpscaleFactory interface {
	CreateImageUpscaleRequest(props *ImageUpscaleProps, hook globals.Hook) error
}

// ImageOutpaintFactory 画布扩展（改尺寸）
type ImageOutpaintFactory interface {
	CreateImageOutpaintRequest(props *ImageOutpaintProps, hook globals.Hook) error
}

// ImageToVideoFactory 图生视频
type ImageToVideoFactory interface {
	CreateImageToVideoRequest(props *ImageToVideoProps, hook globals.Hook) error
}

type ImageEditFactoryCreator func(globals.ChannelConfig) ImageEditFactory

type ImageGenerationFactoryCreator func(globals.ChannelConfig) ImageGenerationFactory
