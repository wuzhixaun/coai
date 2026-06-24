package auth

import (
	"fmt"
	"time"

	"chat/utils"
)

const alipayService = "alipay"

func alipayAmount(quota int) float32 {
	return float32(quota) * 0.1
}

func createAlipayOrderID(username string) string {
	raw := fmt.Sprintf("%s:%d:%s", username, time.Now().UnixNano(), utils.GenerateChar(12))
	return "alipay_" + utils.Sha2Encrypt(raw)[:24]
}
