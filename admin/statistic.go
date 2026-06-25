package admin

import (
	"chat/adapter"
	"chat/connection"
	"chat/utils"
	"time"

	"github.com/go-redis/redis/v8"
)

func IncrErrorRequest(cache *redis.Client) {
	utils.IncrOnce(cache, getErrorFormat(getDay()), time.Hour*24*7*2)
}

func IncrBillingRequest(cache *redis.Client, amount int64) {
	utils.IncrWithExpire(cache, getBillingFormat(getDay()), amount, time.Hour*24*30*2)
	utils.IncrWithExpire(cache, getMonthBillingFormat(getMonth()), amount, time.Hour*24*30*2)
}

func IncrRequest(cache *redis.Client) {
	utils.IncrOnce(cache, getRequestFormat(getDay()), time.Hour*24*7*2)
}

func IncrModelRequest(cache *redis.Client, model string, tokens int64) {
	utils.IncrWithExpire(cache, getModelFormat(getDay(), model), tokens, time.Hour*24*7*2)
}

func AnalyseRequest(model string, buffer *utils.Buffer, err error) {
	instance := connection.Cache

	if adapter.IsAvailableError(err) {
		IncrErrorRequest(instance)
		return
	}

	IncrRequest(instance)
	IncrModelRequest(instance, model, int64(buffer.CountToken()))
}

// AnalyseImageRequest 把一次图片生成（Photo 链路，无 buffer）计入与聊天/API 一致的分析计数器：
// 失败计入错误数；成功计入请求量与模型用量（以图片张数作为模型用量度量）。
// 这样「数据分析」面板的请求量/模型使用/错误统计也能反映 Photo 图片处理活动。
func AnalyseImageRequest(model string, imageCount int, err error) {
	instance := connection.Cache

	if err != nil {
		IncrErrorRequest(instance)
		return
	}

	IncrRequest(instance)
	if imageCount < 1 {
		imageCount = 1
	}
	IncrModelRequest(instance, model, int64(imageCount))
}
