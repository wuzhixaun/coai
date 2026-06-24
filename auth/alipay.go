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

	p := alipay.TradePreCreate{}
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

// completeAlipayByOrder 按订单号查回 user_id/amount 后入账（幂等由 completePaymentOrder 保证）。
func completeAlipayByOrder(db *sql.DB, orderID string) error {
	var userID int64
	var amount float32
	if err := globals.QueryRowDb(db,
		`SELECT user_id, amount FROM payment WHERE order_id = ? AND service = ?`,
		orderID, alipayService).Scan(&userID, &amount); err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	return completePaymentOrder(db, orderID, userID, amount)
}

// queryAlipayOrder 主动向支付宝查单作为回调兜底：已支付则入账并返回 true。
// 未支付/不存在返回 (false, nil)（轮询期间属正常，不视为错误）；仅真正的错误（如客户端构建失败）才返回 error。
func queryAlipayOrder(db *sql.DB, orderID string) (bool, error) {
	client, err := newAlipayClient()
	if err != nil {
		return false, err
	}

	rsp, err := client.TradeQuery(context.Background(), alipay.TradeQuery{OutTradeNo: orderID})
	if err != nil {
		return false, fmt.Errorf("支付宝查单失败: %w", err)
	}

	// v3.2.29：TradeQueryRsp 内嵌 Error，成功码/交易状态直接在 rsp 上（非 rsp.Content.*）。
	if rsp.Code != alipay.CodeSuccess {
		return false, nil // 订单未支付/不存在
	}
	if rsp.TradeStatus != alipay.TradeStatusSuccess && rsp.TradeStatus != alipay.TradeStatusFinished {
		return false, nil
	}

	if err := completeAlipayByOrder(db, orderID); err != nil {
		return false, err
	}
	return true, nil
}

// AlipayNotifyAPI 处理支付宝异步回调：验签后入账。验签由 DecodeNotification 内部完成。
func AlipayNotifyAPI(c *gin.Context) {
	client, err := newAlipayClient()
	if err != nil {
		c.String(200, "failure")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		globals.Warn(fmt.Sprintf("[alipay] notify parse form failed: %v", err))
		c.String(200, "failure")
		return
	}

	// DecodeNotification 内部完成验签（v3.2.29 签名为 (ctx, url.Values)）。
	noti, err := client.DecodeNotification(c.Request.Context(), c.Request.Form)
	if err != nil {
		globals.Warn(fmt.Sprintf("[alipay] notify verify failed: %v", err))
		c.String(200, "failure")
		return
	}

	if noti.TradeStatus != alipay.TradeStatusSuccess && noti.TradeStatus != alipay.TradeStatusFinished {
		// 非成功状态也要回 success 避免支付宝重试风暴
		c.String(200, "success")
		return
	}

	db := utils.GetDBFromContext(c)
	if err := completeAlipayByOrder(db, noti.OutTradeNo); err != nil {
		globals.Warn(fmt.Sprintf("[alipay] complete order %s failed: %v", noti.OutTradeNo, err))
		c.String(200, "failure")
		return
	}

	c.String(200, "success")
}
