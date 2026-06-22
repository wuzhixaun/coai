package manager

import (
	adaptercommon "chat/adapter/common"
	"chat/admin"
	"chat/auth"
	"chat/channel"
	"chat/globals"
	"chat/utils"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func ImagesRelayAPI(c *gin.Context) {
	if globals.CloseRelay {
		abortWithErrorResponse(c, fmt.Errorf("relay api is denied of access"), "access_denied_error")
		return
	}

	username := utils.GetUserFromContext(c)
	if username == "" {
		abortWithErrorResponse(c, fmt.Errorf("access denied for invalid api key"), "authentication_error")
		return
	}

	if utils.GetAgentFromContext(c) != "api" {
		abortWithErrorResponse(c, fmt.Errorf("access denied for invalid agent"), "authentication_error")
		return
	}

	var form RelayImageForm
	if err := c.ShouldBindJSON(&form); err != nil {
		abortWithErrorResponse(c, fmt.Errorf("invalid request body: %s", err.Error()), "invalid_request_error")
		return
	}

	prompt := strings.TrimSpace(form.Prompt)
	if prompt == "" {
		sendErrorResponse(c, fmt.Errorf("prompt is required"), "invalid_request_error")
		return
	}

	db := utils.GetDBFromContext(c)
	user := &auth.User{
		Username: username,
	}

	created := time.Now().Unix()

	if strings.HasSuffix(form.Model, "-official") {
		form.Model = strings.TrimSuffix(form.Model, "-official")
	}

	check := auth.CanEnableModel(db, user, form.Model, []globals.Message{})
	if check != nil {
		sendErrorResponse(c, check, "quota_exceeded_error")
		return
	}

	createRelayImageObject(c, form, prompt, created, user, supportRelayPlan())
}

func getImageProps(form RelayImageForm, messages []globals.Message, buffer *utils.Buffer) *adaptercommon.ChatProps {
	return adaptercommon.CreateChatProps(&adaptercommon.ChatProps{
		Model:     form.Model,
		Message:   messages,
		MaxTokens: utils.ToPtr(-1),
	}, buffer)
}

func getImageGenerationProps(form RelayImageForm, prompt string, user string) (*adaptercommon.ImageGenerationProps, error) {
	n := 1
	if form.N != nil {
		n = *form.N
	}

	size, width, height, err := parseRelayImageSize(form.Size, form.Width, form.Height)
	if err != nil {
		return nil, err
	}

	forceSingle := true
	if form.ForceSingle != nil {
		forceSingle = *form.ForceSingle
	}

	images := make([]string, 0, len(form.ImageURLs)+len(form.Images))
	images = append(images, form.ImageURLs...)
	images = append(images, form.Images...)

	return adaptercommon.CreateImageGenerationProps(&adaptercommon.ImageGenerationProps{
		Model:       form.Model,
		Prompt:      prompt,
		Images:      images,
		Masks:       form.Masks,
		N:           n,
		Size:        size,
		Width:       width,
		Height:      height,
		Scale:       form.Scale,
		MinRatio:    form.MinRatio,
		MaxRatio:    form.MaxRatio,
		ForceSingle: utils.ToPtr(forceSingle),
		ReturnURL:   true,
		User:        user,
	}), nil
}

func parseRelayImageSize(raw any, width *int, height *int) (*int, *int, *int, error) {
	if raw == nil {
		return nil, width, height, nil
	}
	if value, ok := raw.(string); ok && strings.TrimSpace(value) == "" {
		return nil, width, height, nil
	}

	if width != nil || height != nil {
		return nil, width, height, fmt.Errorf("size cannot be combined with explicit width or height")
	}

	switch value := raw.(type) {
	case string:
		return parseRelayImageSizeString(value)
	case float64:
		size, err := parsePositiveIntegerSize(value)
		if err != nil {
			return nil, nil, nil, err
		}
		return utils.ToPtr(size), nil, nil, nil
	case int:
		if value <= 0 {
			return nil, nil, nil, fmt.Errorf("size must be positive")
		}
		return utils.ToPtr(value), nil, nil, nil
	case json.Number:
		f, err := value.Float64()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("size must be a positive integer area or WIDTHxHEIGHT string")
		}
		size, err := parsePositiveIntegerSize(f)
		if err != nil {
			return nil, nil, nil, err
		}
		return utils.ToPtr(size), nil, nil, nil
	default:
		return nil, nil, nil, fmt.Errorf("size must be a positive integer area or WIDTHxHEIGHT string")
	}
}

func parseRelayImageSizeString(raw string) (*int, *int, *int, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	value = strings.ReplaceAll(value, "×", "x")
	if value == "" {
		return nil, nil, nil, nil
	}

	if strings.Contains(value, "x") {
		parts := strings.Split(value, "x")
		if len(parts) != 2 {
			return nil, nil, nil, fmt.Errorf("size must be WIDTHxHEIGHT")
		}
		width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || width <= 0 {
			return nil, nil, nil, fmt.Errorf("size width must be positive")
		}
		height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || height <= 0 {
			return nil, nil, nil, fmt.Errorf("size height must be positive")
		}
		return nil, utils.ToPtr(width), utils.ToPtr(height), nil
	}

	size, err := strconv.Atoi(value)
	if err != nil || size <= 0 {
		return nil, nil, nil, fmt.Errorf("size must be a positive integer area or WIDTHxHEIGHT string")
	}
	return utils.ToPtr(size), nil, nil, nil
}

func parsePositiveIntegerSize(value float64) (int, error) {
	if value <= 0 || math.Trunc(value) != value {
		return 0, fmt.Errorf("size must be a positive integer area")
	}
	return int(value), nil
}

func getImageDataListFromBuffer(buffer *utils.Buffer) []RelayImageData {
	content := buffer.Read()
	items := make([]RelayImageData, 0)

	for _, imageURL := range utils.ExtractImagesFromMarkdown(content) {
		items = append(items, RelayImageData{Url: imageURL})
	}

	for _, b64Json := range utils.ExtractBase64FromMarkdown(content) {
		items = append(items, RelayImageData{B64Json: b64Json})
	}

	return items
}

func createRelayImageObject(c *gin.Context, form RelayImageForm, prompt string, created int64, user *auth.User, plan bool) {
	if globals.IsJimengImageGenerationModel(form.Model) {
		createRelayJimengImageObject(c, form, prompt, created, user, plan)
		return
	}

	db := utils.GetDBFromContext(c)
	cache := utils.GetCacheFromContext(c)

	messages := []globals.Message{
		{
			Role:    globals.User,
			Content: prompt,
		},
	}

	buffer := utils.NewBuffer(form.Model, messages, channel.ChargeInstance.GetCharge(form.Model))
	hit, err := channel.NewChatRequestWithCache(cache, buffer, auth.GetGroup(db, user), getImageProps(form, messages, buffer), func(data *globals.Chunk) error {
		buffer.WriteChunk(data)
		return nil
	})

	admin.AnalyseRequest(form.Model, buffer, err)
	if err != nil {
		auth.RevertSubscriptionUsage(db, cache, user, form.Model)
		globals.Warn(fmt.Sprintf("error from chat request api: %s (instance: %s, client: %s)", err, form.Model, c.ClientIP()))

		sendErrorResponse(c, err)
		return
	}

	if !hit {
		CollectQuota(c, user, buffer, plan, err)
	}

	data := getImageDataListFromBuffer(buffer)
	if len(data) == 0 {
		sendErrorResponse(c, fmt.Errorf("no image generated"), "image_generation_error")
		return
	}

	c.JSON(http.StatusOK, RelayImageResponse{
		Created: created,
		Data:    data,
	})
}

func createRelayJimengImageObject(c *gin.Context, form RelayImageForm, prompt string, created int64, user *auth.User, plan bool) {
	db := utils.GetDBFromContext(c)

	messages := []globals.Message{
		{
			Role:    globals.User,
			Content: prompt,
		},
	}

	buffer := utils.NewBuffer(form.Model, messages, channel.ChargeInstance.GetCharge(form.Model))
	props, err := getImageGenerationProps(form, prompt, auth.GetUsernameString(db, user))
	if err != nil {
		sendErrorResponse(c, err, "invalid_request_error")
		return
	}
	err = channel.NewImageGenerationRequestWithChannel(auth.GetGroup(db, user), props, func(data *globals.Chunk) error {
		buffer.WriteChunk(data)
		return nil
	})

	admin.AnalyseRequest(form.Model, buffer, err)
	if err != nil {
		auth.RevertSubscriptionUsage(db, utils.GetCacheFromContext(c), user, form.Model)
		globals.Warn(fmt.Sprintf("error from image generation api: %s (instance: %s, client: %s)", err, form.Model, c.ClientIP()))

		sendErrorResponse(c, err)
		return
	}

	CollectQuota(c, user, buffer, plan, err)

	data := getImageDataListFromBuffer(buffer)
	if len(data) == 0 {
		sendErrorResponse(c, fmt.Errorf("no image generated"), "image_generation_error")
		return
	}

	c.JSON(http.StatusOK, RelayImageResponse{
		Created: created,
		Data:    data,
	})
}
