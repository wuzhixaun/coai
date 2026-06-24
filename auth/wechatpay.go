package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chat/channel"
	"chat/utils"

	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	wxutils "github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const wechatPayService = "wxpay"

func wechatAmountFen(quota int) int64 {
	return int64(quota) * 10 // quota*0.1 元 = quota*10 分
}

func createWechatOrderID(username string) string {
	raw := fmt.Sprintf("%s:%d:%s", username, time.Now().UnixNano(), utils.GenerateChar(12))
	return "wxpay_" + utils.Sha2Encrypt(raw)[:24]
}

func newWechatClient(ctx context.Context) (*core.Client, error) {
	conf := channel.SystemInstance.Payment.WechatPay
	if !conf.IsValid() {
		return nil, fmt.Errorf("微信支付未配置")
	}

	priv, err := wxutils.LoadPrivateKey(conf.MchPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("加载微信商户私钥失败: %w", err)
	}

	opts := []core.ClientOption{
		option.WithWechatPayAutoAuthCipher(conf.MchID, conf.MchCertSerialNo, priv, conf.APIv3Key),
	}
	return core.NewClient(ctx, opts...)
}

func createWechatOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	conf := channel.SystemInstance.Payment.WechatPay
	ctx := context.Background()

	client, err := newWechatClient(ctx)
	if err != nil {
		return "", "", err
	}

	db := utils.GetDBFromContext(c)
	orderID := createWechatOrderID(user.Username)
	if err := insertPaymentOrder(db, user, form, orderID, wechatPayService); err != nil {
		return "", "", err
	}

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}
	notifyURL := normalizePaymentDomain(c, form.Domain) + "/payment/wechat/notify"
	total := wechatAmountFen(form.Quota)

	svc := native.NativeApiService{Client: client}
	resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(conf.AppID),
		Mchid:       core.String(conf.MchID),
		Description: core.String(name),
		OutTradeNo:  core.String(orderID),
		NotifyUrl:   core.String(notifyURL),
		Amount:      &native.Amount{Total: core.Int64(total), Currency: core.String("CNY")},
	})
	if err != nil {
		return "", "", fmt.Errorf("微信下单失败: %w", err)
	}
	if resp.CodeUrl == nil {
		return "", "", fmt.Errorf("微信下单未返回二维码")
	}

	return *resp.CodeUrl, orderID, nil
}
