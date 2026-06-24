package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chat/channel"
	"chat/utils"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"
)

const alipayService = "alipay"

func alipayAmount(quota int) float32 {
	return float32(quota) * 0.1
}

func createAlipayOrderID(username string) string {
	raw := fmt.Sprintf("%s:%d:%s", username, time.Now().UnixNano(), utils.GenerateChar(12))
	return "alipay_" + utils.Sha2Encrypt(raw)[:24]
}

func newAlipayClient() (*alipay.Client, error) {
	conf := channel.SystemInstance.Payment.Alipay
	if !conf.IsValid() {
		return nil, fmt.Errorf("支付宝支付未配置")
	}
	client, err := alipay.New(conf.AppID, conf.PrivateKey, conf.IsProd)
	if err != nil {
		return nil, fmt.Errorf("支付宝客户端初始化失败: %w", err)
	}
	if err := client.LoadAliPayPublicKey(conf.AlipayPublicKey); err != nil {
		return nil, fmt.Errorf("加载支付宝公钥失败: %w", err)
	}
	return client, nil
}

func createAlipayOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	client, err := newAlipayClient()
	if err != nil {
		return "", "", err
	}

	db := utils.GetDBFromContext(c)
	orderID := createAlipayOrderID(user.Username)
	if err := insertPaymentOrder(db, user, form, orderID, alipayService); err != nil {
		return "", "", err
	}

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}
	notifyURL := normalizePaymentDomain(c, form.Domain) + "/payment/alipay/notify"

	var p = alipay.TradePreCreate{}
	p.OutTradeNo = orderID
	p.Subject = name
	p.TotalAmount = fmt.Sprintf("%.2f", alipayAmount(form.Quota))
	p.NotifyURL = notifyURL

	rsp, err := client.TradePreCreate(context.Background(), p)
	if err != nil {
		return "", "", fmt.Errorf("支付宝下单失败: %w", err)
	}
	if rsp.Code != alipay.CodeSuccess {
		return "", "", fmt.Errorf("支付宝下单失败: %s", rsp.SubMsg)
	}
	return rsp.QRCode, orderID, nil
}
