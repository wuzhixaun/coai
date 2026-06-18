package auth

import (
	"chat/channel"
	"chat/globals"
	"chat/utils"
	"crypto/md5"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const ePayService = "epay"

func isEPayService(service string) bool {
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "epay", "alipay", "wxpay", "wechatpay", "wechat", "bank", "unionpay", "qqpay":
		return true
	default:
		return false
	}
}

func createEPayOrderID(username string) string {
	raw := fmt.Sprintf("%s:%d:%s", username, time.Now().UnixNano(), utils.GenerateChar(12))
	return "epay_" + utils.Sha2Encrypt(raw)[:24]
}

func ePayAmount(quota int) float32 {
	return float32(quota) * 0.1
}

func ePaySign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}

	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}

func ePayValuesToMap(values url.Values) map[string]string {
	data := make(map[string]string, len(values))
	for k, v := range values {
		if len(v) == 0 {
			continue
		}
		data[k] = v[0]
	}
	return data
}

func verifyEPaySign(values url.Values, key string) bool {
	actual := strings.ToLower(strings.TrimSpace(values.Get("sign")))
	if actual == "" {
		return false
	}

	expected := ePaySign(ePayValuesToMap(values), key)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func getEPayCallbackValues(c *gin.Context) url.Values {
	_ = c.Request.ParseForm()
	values := make(url.Values)
	for k, v := range c.Request.Form {
		values[k] = v
	}
	return values
}

func isEPaySuccess(values url.Values) bool {
	switch strings.ToUpper(strings.TrimSpace(values.Get("trade_status"))) {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		return true
	}

	switch strings.ToLower(strings.TrimSpace(values.Get("status"))) {
	case "1", "success", "paid":
		return true
	default:
		return false
	}
}

func ePayCallbackBase(c *gin.Context, frontendDomain string) string {
	if backend := channel.SystemInstance.GetBackend(); backend != "" {
		return strings.TrimRight(backend, "/")
	}

	return normalizePaymentDomain(c, frontendDomain)
}

func createEPayOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	conf := channel.SystemInstance.Payment.EPay
	if !conf.Accepts(form.Type) {
		return "", "", fmt.Errorf("epay payment is not configured for %s", form.Type)
	}

	method := conf.NormalizeMethod(form.Type)
	orderID := createEPayOrderID(user.Username)
	amount := ePayAmount(form.Quota)
	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}

	db := utils.GetDBFromContext(c)
	if err := insertPaymentOrder(db, user, form, orderID, method); err != nil {
		return "", "", err
	}

	callbackBase := ePayCallbackBase(c, form.Domain)
	frontendDomain := normalizePaymentDomain(c, form.Domain)
	params := map[string]string{
		"pid":          conf.GetBusinessID(),
		"out_trade_no": orderID,
		"notify_url":   callbackBase + "/payment/epay/notify",
		"return_url":   callbackBase + "/payment/epay/return",
		"name":         name,
		"money":        fmt.Sprintf("%.2f", amount),
		"param":        frontendDomain,
		"sitename":     channel.SystemInstance.GetAppName(),
	}
	if payType := conf.GatewayType(method); payType != "" {
		params["type"] = payType
	}
	params["sign"] = ePaySign(params, conf.GetBusinessKey())
	params["sign_type"] = "MD5"

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	return conf.GetDomain() + "/submit.php?" + values.Encode(), orderID, nil
}

type ePayLocalOrder struct {
	UserID  int64
	Service string
	Amount  float32
	State   bool
}

func queryEPayLocalOrder(db *sql.DB, orderID string) (*ePayLocalOrder, error) {
	var order ePayLocalOrder
	err := globals.QueryRowDb(db, `
		SELECT user_id, service, amount, state FROM payment
		WHERE order_id = ?
	`, orderID).Scan(&order.UserID, &order.Service, &order.Amount, &order.State)
	if err != nil {
		return nil, err
	}
	if !isEPayService(order.Service) {
		return nil, fmt.Errorf("unsupported payment provider")
	}

	return &order, nil
}

func handleEPayCallback(c *gin.Context) (string, string, error) {
	conf := channel.SystemInstance.Payment.EPay
	if !conf.IsValid() {
		return "", "", fmt.Errorf("epay payment is not configured")
	}

	values := getEPayCallbackValues(c)
	if !verifyEPaySign(values, conf.GetBusinessKey()) {
		return values.Get("out_trade_no"), "", fmt.Errorf("invalid epay signature")
	}

	orderID := strings.TrimSpace(values.Get("out_trade_no"))
	if orderID == "" {
		return "", values.Get("param"), fmt.Errorf("missing order id")
	}
	if !isEPaySuccess(values) {
		return orderID, values.Get("param"), fmt.Errorf("epay order is not paid")
	}

	db := utils.GetDBFromContext(c)
	order, err := queryEPayLocalOrder(db, orderID)
	if err != nil {
		return orderID, values.Get("param"), err
	}

	if money := strings.TrimSpace(values.Get("money")); money != "" {
		paidAmount, err := strconv.ParseFloat(money, 64)
		if err != nil {
			return orderID, values.Get("param"), fmt.Errorf("invalid paid amount")
		}
		if math.Abs(paidAmount-float64(order.Amount)) > 0.01 {
			return orderID, values.Get("param"), fmt.Errorf("paid amount mismatch")
		}
	}

	if order.State {
		return orderID, values.Get("param"), nil
	}

	if err := completePaymentOrder(db, orderID, order.UserID, order.Amount); err != nil {
		return orderID, values.Get("param"), err
	}

	return orderID, values.Get("param"), nil
}

func ePayReturnURL(frontendDomain string, orderID string, err error) string {
	frontendDomain = strings.TrimRight(strings.TrimSpace(frontendDomain), "/")
	if frontendDomain == "" || (!strings.HasPrefix(frontendDomain, "http://") && !strings.HasPrefix(frontendDomain, "https://")) {
		frontendDomain = "/"
	}

	base := frontendDomain
	if strings.HasSuffix(base, "/") {
		base += "wallet"
	} else {
		base += "/wallet"
	}

	values := url.Values{}
	values.Set("payment", ePayService)
	if orderID != "" {
		values.Set("order", orderID)
	}
	if err != nil {
		values.Set("error", err.Error())
	}

	return base + "?" + values.Encode()
}

func EPayNotifyAPI(c *gin.Context) {
	if _, _, err := handleEPayCallback(c); err != nil {
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

func EPayReturnAPI(c *gin.Context) {
	orderID, frontendDomain, err := handleEPayCallback(c)
	c.Redirect(http.StatusFound, ePayReturnURL(frontendDomain, orderID, err))
}
