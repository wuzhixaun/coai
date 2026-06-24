package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"chat/channel"
	"chat/globals"
	"chat/utils"

	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
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

// completeWechatByOrder 根据订单号查出对应用户与金额，幂等地完成充值。
func completeWechatByOrder(db *sql.DB, orderID string) error {
	var userID int64
	var amount float32
	if err := globals.QueryRowDb(db,
		`SELECT user_id, amount FROM payment WHERE order_id = ? AND service = ?`,
		orderID, wechatPayService).Scan(&userID, &amount); err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	return completePaymentOrder(db, orderID, userID, amount)
}

// queryWechatOrder 主动向微信查询订单状态，作为 /payment/check 的兜底：
// 本地无公网回调或回调丢失时，前端轮询可借此确认支付结果。
// 未支付（trade_state 为空或非 SUCCESS）返回 (false, nil)，仅传输错误才向上传播。
func queryWechatOrder(db *sql.DB, orderID string) (bool, error) {
	conf := channel.SystemInstance.Payment.WechatPay
	ctx := context.Background()

	client, err := newWechatClient(ctx)
	if err != nil {
		return false, err
	}

	svc := native.NativeApiService{Client: client}
	resp, _, err := svc.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(orderID),
		Mchid:      core.String(conf.MchID),
	})
	if err != nil {
		return false, fmt.Errorf("微信查单失败: %w", err)
	}

	if resp.TradeState == nil || *resp.TradeState != "SUCCESS" {
		// 尚未支付——轮询中的正常状态，不视为错误。
		return false, nil
	}

	if err := completeWechatByOrder(db, orderID); err != nil {
		return false, err
	}
	return true, nil
}

// newWechatNotifyHandler 使用自动下载并轮换的微信支付平台证书构造通知处理器。
// 处理器内部会先用平台证书做 SHA256-RSA 验签，再用 APIv3Key 做 AES-GCM 解密，
// 二者均通过后 ParseNotifyRequest 才会把明文写入 content，从而保证验签+解密先于充值。
func newWechatNotifyHandler(ctx context.Context, client *core.Client, apiV3Key string) (*notify.Handler, error) {
	conf := channel.SystemInstance.Payment.WechatPay

	mgr := downloader.MgrInstance()
	if err := mgr.RegisterDownloaderWithClient(ctx, client, conf.MchID, apiV3Key); err != nil {
		return nil, fmt.Errorf("注册微信平台证书下载器失败: %w", err)
	}
	certVisitor := mgr.GetCertificateVisitor(conf.MchID)

	// NewRSANotifyHandler 自带 AES-GCM 解密能力（NewNotifyHandler 已废弃）。
	handler, err := notify.NewRSANotifyHandler(apiV3Key, verifiers.NewSHA256WithRSAVerifier(certVisitor))
	if err != nil {
		return nil, fmt.Errorf("构造微信通知处理器失败: %w", err)
	}
	return handler, nil
}

// WechatNotifyAPI 处理微信支付异步回调：验签+解密成功且交易成功后幂等充值。
// 微信要求以 HTTP 200 + {"code":"SUCCESS"} 表示已成功接收；其它响应会触发微信重试。
func WechatNotifyAPI(c *gin.Context) {
	conf := channel.SystemInstance.Payment.WechatPay
	ctx := context.Background()

	client, err := newWechatClient(ctx)
	if err != nil {
		c.JSON(500, gin.H{"code": "FAIL", "message": "not configured"})
		return
	}

	handler, err := newWechatNotifyHandler(ctx, client, conf.APIv3Key)
	if err != nil {
		globals.Warn(fmt.Sprintf("[wechat] init notify handler failed: %v", err))
		c.JSON(500, gin.H{"code": "FAIL", "message": "init handler failed"})
		return
	}

	transaction := make(map[string]interface{})
	if _, err := handler.ParseNotifyRequest(ctx, c.Request, &transaction); err != nil {
		globals.Warn(fmt.Sprintf("[wechat] notify verify failed: %v", err))
		c.JSON(401, gin.H{"code": "FAIL", "message": "verify failed"})
		return
	}

	if state, _ := transaction["trade_state"].(string); state != "SUCCESS" {
		// 非成功状态仅确认接收，不充值。
		c.JSON(200, gin.H{"code": "SUCCESS", "message": "OK"})
		return
	}

	orderID, _ := transaction["out_trade_no"].(string)
	db := utils.GetDBFromContext(c)
	if err := completeWechatByOrder(db, orderID); err != nil {
		globals.Warn(fmt.Sprintf("[wechat] complete order %s failed: %v", orderID, err))
		c.JSON(500, gin.H{"code": "FAIL", "message": "process failed"})
		return
	}

	c.JSON(200, gin.H{"code": "SUCCESS", "message": "OK"})
}
