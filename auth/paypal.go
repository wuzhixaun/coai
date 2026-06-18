package auth

import (
	"bytes"
	"chat/channel"
	"chat/globals"
	"chat/utils"
	"database/sql"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

const paypalService = "paypal"

type CreatePaymentForm struct {
	Type   string `json:"type" binding:"required"`
	Quota  int    `json:"quota" binding:"required"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
	Device string `json:"device"`
}

type paypalTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type paypalLink struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

type paypalOrderResponse struct {
	ID     string       `json:"id"`
	Status string       `json:"status"`
	Links  []paypalLink `json:"links"`
}

type paypalErrorResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	DebugID string `json:"debug_id"`
}

func paypalBaseURL() string {
	if channel.SystemInstance.Payment.PayPal.GetMode() == "live" {
		return "https://api-m.paypal.com"
	}

	return "https://api-m.sandbox.paypal.com"
}

func paypalApprovalURL(order paypalOrderResponse) string {
	for _, link := range order.Links {
		rel := strings.ToLower(link.Rel)
		if rel == "approve" || rel == "payer-action" {
			return link.Href
		}
	}

	return ""
}

func paypalHTTP(method, path, token string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, paypalBaseURL()+path, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := (&http.Client{Timeout: globals.HttpMaxTimeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var paypalErr paypalErrorResponse
		if err := json.Unmarshal(raw, &paypalErr); err == nil && paypalErr.Message != "" {
			return fmt.Errorf("paypal %s: %s", paypalErr.Name, paypalErr.Message)
		}
		return fmt.Errorf("paypal request failed: %s", resp.Status)
	}

	if out == nil || len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, out)
}

func getPayPalAccessToken() (string, error) {
	conf := channel.SystemInstance.Payment.PayPal
	if !conf.IsValid() {
		return "", fmt.Errorf("paypal payment is not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(
		http.MethodPost,
		paypalBaseURL()+"/v1/oauth2/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(conf.GetClientID(), conf.Secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: globals.HttpMaxTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("paypal token request failed: %s", resp.Status)
	}

	var data paypalTokenResponse
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", err
	}
	if data.AccessToken == "" {
		return "", fmt.Errorf("paypal token response is empty")
	}

	return data.AccessToken, nil
}

func normalizePaymentDomain(c *gin.Context, domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		domain = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}

	return strings.TrimRight(domain, "/")
}

func createPayPalOrder(c *gin.Context, form CreatePaymentForm) (*paypalOrderResponse, error) {
	token, err := getPayPalAccessToken()
	if err != nil {
		return nil, err
	}

	amount := float32(form.Quota) * 0.1
	domain := normalizePaymentDomain(c, form.Domain)
	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}

	body := map[string]interface{}{
		"intent": "CAPTURE",
		"purchase_units": []map[string]interface{}{
			{
				"reference_id": utils.Sha2Encrypt(fmt.Sprintf("%s:%d", utils.GetUserFromContext(c), form.Quota))[:32],
				"description":  name,
				"amount": map[string]string{
					"currency_code": channel.SystemInstance.Payment.PayPal.GetCurrency(),
					"value":         fmt.Sprintf("%.2f", amount),
				},
			},
		},
		"payment_source": map[string]interface{}{
			"paypal": map[string]interface{}{
				"experience_context": map[string]string{
					"payment_method_preference": "IMMEDIATE_PAYMENT_REQUIRED",
					"brand_name":                channel.SystemInstance.GetAppName(),
					"landing_page":              "LOGIN",
					"shipping_preference":       "NO_SHIPPING",
					"user_action":               "PAY_NOW",
					"return_url":                fmt.Sprintf("%s/wallet?payment=paypal", domain),
					"cancel_url":                fmt.Sprintf("%s/wallet?payment=cancel", domain),
				},
			},
		},
	}

	var order paypalOrderResponse
	if err := paypalHTTP(http.MethodPost, "/v2/checkout/orders", token, body, &order); err != nil {
		return nil, err
	}
	if order.ID == "" {
		return nil, fmt.Errorf("paypal order id is empty")
	}

	return &order, nil
}

func insertPaymentOrder(db *sql.DB, user *User, form CreatePaymentForm, orderID string, service string) error {
	amount := float32(form.Quota) * 0.1
	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}

	_, err := globals.ExecDb(db, `
		INSERT INTO payment (user_id, username, type, service, amount, order_id, name, device, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.GetID(db), user.Username, "quota", service, amount, orderID, name, form.Device, false)

	return err
}

func CreatePaymentAPI(c *gin.Context) {
	user := GetUserByCtx(c)
	if user == nil {
		return
	}

	var form CreatePaymentForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
			"error":  err.Error(),
		})
		return
	}

	if form.Quota <= 0 || form.Quota > 99999 {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
			"error":  "invalid quota range (1 ~ 99999)",
		})
		return
	}

	paymentType := strings.ToLower(strings.TrimSpace(form.Type))
	form.Type = paymentType
	if channel.SystemInstance.Payment.EPay.Accepts(paymentType) {
		paymentURL, orderID, err := createEPayOrder(c, user, form)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status": false,
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": true,
			"data": gin.H{
				"url": paymentURL,
				"params": gin.H{
					"order": orderID,
				},
			},
		})
		return
	}

	if paymentType == stripeService {
		session, err := createStripeSession(c, user, form)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status": false,
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": true,
			"data": gin.H{
				"url": session.URL,
				"params": gin.H{
					"order": session.ID,
				},
			},
		})
		return
	}

	if paymentType != paypalService {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
			"error":  "unsupported payment provider",
		})
		return
	}

	order, err := createPayPalOrder(c, form)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
			"error":  err.Error(),
		})
		return
	}

	approvalURL := paypalApprovalURL(*order)
	if approvalURL == "" {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
			"error":  "paypal approval url is empty",
		})
		return
	}

	db := utils.GetDBFromContext(c)
	if err := insertPaymentOrder(db, user, form, order.ID, paypalService); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"url": approvalURL,
			"params": gin.H{
				"order": order.ID,
			},
		},
	})
}

func capturePayPalOrder(orderID string) (*paypalOrderResponse, error) {
	token, err := getPayPalAccessToken()
	if err != nil {
		return nil, err
	}

	var order paypalOrderResponse
	if err := paypalHTTP(http.MethodGet, fmt.Sprintf("/v2/checkout/orders/%s", url.PathEscape(orderID)), token, nil, &order); err != nil {
		return nil, err
	}

	switch strings.ToUpper(order.Status) {
	case "COMPLETED":
		return &order, nil
	case "APPROVED":
		var captured paypalOrderResponse
		if err := paypalHTTP(http.MethodPost, fmt.Sprintf("/v2/checkout/orders/%s/capture", url.PathEscape(orderID)), token, map[string]string{}, &captured); err != nil {
			return nil, err
		}
		return &captured, nil
	default:
		return &order, nil
	}
}

func completePaymentOrder(db *sql.DB, orderID string, userID int64, amount float32) error {
	quota := float32(math.Round(float64(amount * 10)))
	if quota <= 0 {
		return fmt.Errorf("invalid payment amount")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	result, err := tx.Exec(globals.PreflightSql(`
		UPDATE payment SET state = TRUE, updated_at = CURRENT_TIMESTAMP
		WHERE order_id = ? AND user_id = ? AND state = FALSE
	`), orderID, userID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if affected == 0 {
		_ = tx.Rollback()
		return nil
	}

	_, err = tx.Exec(globals.PreflightSql(`
		INSERT INTO quota (user_id, quota, used) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE quota = quota + ?
	`), userID, quota, 0., quota)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to increase quota: %w", err)
	}

	return tx.Commit()
}

func CheckPaymentAPI(c *gin.Context) {
	user := GetUserByCtx(c)
	if user == nil {
		return
	}

	db := utils.GetDBFromContext(c)
	orderID := strings.TrimSpace(c.Param("order"))
	if orderID == "" {
		c.JSON(http.StatusOK, gin.H{
			"status":      false,
			"error":       "invalid order id",
			"order_state": false,
		})
		return
	}

	var userID int64
	var service string
	var amount float32
	var state bool
	if err := globals.QueryRowDb(db, `
		SELECT user_id, service, amount, state FROM payment
		WHERE order_id = ? AND user_id = ?
	`, orderID, user.GetID(db)).Scan(&userID, &service, &amount, &state); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":      false,
			"error":       "order not found",
			"order_state": false,
		})
		return
	}

	if state {
		c.JSON(http.StatusOK, gin.H{
			"status":      true,
			"order_state": true,
		})
		return
	}

	if isEPayService(service) {
		c.JSON(http.StatusOK, gin.H{
			"status":         true,
			"order_state":    false,
			"payment_status": "WAITING_NOTIFY",
		})
		return
	}

	if service == stripeService {
		session, err := getStripeSession(orderID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":      false,
				"error":       err.Error(),
				"order_state": false,
			})
			return
		}

		if isStripeSessionPaid(session) {
			if err := validateStripePaidAmount(session, amount); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"status":      false,
					"error":       err.Error(),
					"order_state": false,
				})
				return
			}
			if err := completePaymentOrder(db, orderID, userID, amount); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"status":      false,
					"error":       err.Error(),
					"order_state": false,
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"status":      true,
				"order_state": true,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":         true,
			"order_state":    false,
			"payment_status": session.PaymentStatus,
		})
		return
	}

	if service != paypalService {
		c.JSON(http.StatusOK, gin.H{
			"status":      false,
			"error":       "unsupported payment provider",
			"order_state": false,
		})
		return
	}

	order, err := capturePayPalOrder(orderID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":      false,
			"error":       err.Error(),
			"order_state": false,
		})
		return
	}

	if strings.ToUpper(order.Status) == "COMPLETED" {
		if err := completePaymentOrder(db, orderID, userID, amount); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":      false,
				"error":       err.Error(),
				"order_state": false,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":      true,
			"order_state": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         true,
		"order_state":    false,
		"payment_status": order.Status,
	})
}
