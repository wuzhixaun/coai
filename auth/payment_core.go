package auth

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strings"

	"chat/globals"
	"chat/utils"

	"github.com/gin-gonic/gin"
)

const alipayService = "alipay"
const wechatPayService = "wxpay"

func createAlipayOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	return "", "", fmt.Errorf("alipay not configured")
}
func createWechatOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	return "", "", fmt.Errorf("wechat pay not configured")
}
func queryAlipayOrder(db *sql.DB, orderID string) (bool, error) { return false, nil }
func queryWechatOrder(db *sql.DB, orderID string) (bool, error) { return false, nil }

type CreatePaymentForm struct {
	Type   string `json:"type" binding:"required"`
	Quota  int    `json:"quota" binding:"required"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
	Device string `json:"device"`
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

	switch paymentType {
	case alipayService:
		qrcode, orderID, err := createAlipayOrder(c, user, form)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"qrcode": qrcode, "params": gin.H{"order": orderID}}})
	case wechatPayService:
		codeURL, orderID, err := createWechatOrder(c, user, form)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"qrcode": codeURL, "params": gin.H{"order": orderID}}})
	default:
		c.JSON(http.StatusOK, gin.H{"status": false, "error": "unsupported payment provider"})
	}
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

	switch service {
	case alipayService:
		paid, err := queryAlipayOrder(db, orderID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error(), "order_state": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": true, "order_state": paid})
	case wechatPayService:
		paid, err := queryWechatOrder(db, orderID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error(), "order_state": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": true, "order_state": paid})
	default:
		c.JSON(http.StatusOK, gin.H{"status": false, "error": "unsupported payment provider", "order_state": false})
	}
}
