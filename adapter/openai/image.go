package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"fmt"
	"strings"
)

type ImageProps struct {
	Model  string
	Prompt string
	Size   ImageSize
	Proxy  globals.ProxyConfig
}

func (c *ChatInstance) GetImageEndpoint() string {
	return fmt.Sprintf("%s/v1/images/generations", c.GetEndpoint())
}

// CreateImageRequest will create a dalle image from prompt, return url of image, base64 data and error
func (c *ChatInstance) CreateImageRequest(props ImageProps) (string, string, error) {
	res, err := utils.Post(
		c.GetImageEndpoint(),
		c.GetHeader(), ImageRequest{
			Model:  props.Model,
			Prompt: props.Prompt,
			Size: utils.Multi[ImageSize](
				props.Model == globals.Dalle3 || props.Model == globals.GPTImage1,
				ImageSize1024,
				ImageSize512,
			),
			N: 1,
		}, props.Proxy)
	if err != nil || res == nil {
		if err != nil {
			return "", "", fmt.Errorf("openai image request failed: %s", err.Error())
		}
		return "", "", fmt.Errorf("openai image request failed: empty response")
	}

	data := utils.MapToStruct[ImageResponse](res)
	if data == nil {
		return "", "", fmt.Errorf("openai error: cannot parse response")
	} else if data.Error.Message != "" {
		return "", "", fmt.Errorf(data.Error.Message)
	}

	// for gpt-image-1, return base64 data if available
	if props.Model == globals.GPTImage1 && data.Data[0].B64Json != "" {
		return "", data.Data[0].B64Json, nil
	}

	return data.Data[0].Url, "", nil
}

// CreateImage will create a dalle image from prompt, return markdown of image
func (c *ChatInstance) CreateImage(props *adaptercommon.ChatProps) (string, error) {
	url, b64Json, err := c.CreateImageRequest(ImageProps{
		Model:  props.Model,
		Prompt: c.GetLatestPrompt(props),
		Proxy:  props.Proxy,
	})
	if err != nil {
		// 内容被安全系统拒绝时，返回明确错误而非把错误文本当作图片内容返回，
		// 避免被计为成功生成并产生扣费（CollectQuota 在 err != nil 时不扣费）。
		if strings.Contains(err.Error(), "safety") {
			return "", fmt.Errorf("content rejected by safety system: %s", err.Error())
		}
		return "", err
	}

	if b64Json != "" {
		return utils.GetBase64ImageMarkdown(b64Json), nil
	}

	storedUrl := utils.StoreImage(url)
	return utils.GetImageMarkdown(storedUrl), nil
}
