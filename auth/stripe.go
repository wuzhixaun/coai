package auth

import (
	"chat/channel"
	"chat/globals"
	"chat/utils"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

const stripeService = "stripe"

type stripeSessionResponse struct {
	ID            string            `json:"id"`
	URL           string            `json:"url"`
	Status        string            `json:"status"`
	PaymentStatus string            `json:"payment_status"`
	Currency      string            `json:"currency"`
	AmountTotal   int64             `json:"amount_total"`
	Metadata      map[string]string `json:"metadata"`
}

type stripeErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

type stripeLocalOrder struct {
	UserID  int64
	Service string
	Amount  float32
	State   bool
}

type stripeWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		Object stripeSessionResponse `json:"object"`
	} `json:"data"`
}

var stripeZeroDecimalCurrencies = map[string]bool{
	"bif": true,
	"clp": true,
	"djf": true,
	"gnf": true,
	"jpy": true,
	"kmf": true,
	"krw": true,
	"mga": true,
	"pyg": true,
	"rwf": true,
	"ugx": true,
	"vnd": true,
	"vuv": true,
	"xaf": true,
	"xof": true,
	"xpf": true,
}

func stripeAmount(quota int) float32 {
	return float32(quota) * 0.1
}

func stripeMinorUnits(amount float32, currency string) int64 {
	multiplier := 100.0
	if stripeZeroDecimalCurrencies[strings.ToLower(strings.TrimSpace(currency))] {
		multiplier = 1
	}

	return int64(math.Round(float64(amount) * multiplier))
}

func stripeHTTP(method, path string, body url.Values, out interface{}) error {
	conf := channel.SystemInstance.Payment.Stripe
	if !conf.IsValid() {
		return fmt.Errorf("stripe payment is not configured")
	}

	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(body.Encode())
	}

	req, err := http.NewRequest(method, "https://api.stripe.com"+path, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+conf.GetSecretKey())
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		var stripeErr stripeErrorResponse
		if err := json.Unmarshal(raw, &stripeErr); err == nil && stripeErr.Error.Message != "" {
			return fmt.Errorf("stripe %s: %s", stripeErr.Error.Type, stripeErr.Error.Message)
		}
		return fmt.Errorf("stripe request failed: %s", resp.Status)
	}

	if out == nil || len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, out)
}

func createStripeSession(c *gin.Context, user *User, form CreatePaymentForm) (*stripeSessionResponse, error) {
	conf := channel.SystemInstance.Payment.Stripe
	if !conf.IsValid() {
		return nil, fmt.Errorf("stripe payment is not configured")
	}

	amount := stripeAmount(form.Quota)
	currency := conf.GetCurrency()
	unitAmount := stripeMinorUnits(amount, currency)
	if unitAmount <= 0 {
		return nil, fmt.Errorf("invalid stripe amount")
	}

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}

	db := utils.GetDBFromContext(c)
	userID := user.GetID(db)
	domain := normalizePaymentDomain(c, form.Domain)

	cancelValues := url.Values{}
	cancelValues.Set("payment", "cancel")
	cancelValues.Set("provider", stripeService)

	params := url.Values{}
	params.Set("mode", "payment")
	params.Set("success_url", fmt.Sprintf("%s/wallet?payment=stripe&order={CHECKOUT_SESSION_ID}", domain))
	params.Set("cancel_url", fmt.Sprintf("%s/wallet?%s", domain, cancelValues.Encode()))
	params.Set("client_reference_id", fmt.Sprintf("%d", userID))
	params.Set("payment_method_types[0]", "card")
	params.Set("line_items[0][quantity]", "1")
	params.Set("line_items[0][price_data][currency]", currency)
	params.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(unitAmount, 10))
	params.Set("line_items[0][price_data][product_data][name]", name)
	params.Set("metadata[user_id]", fmt.Sprintf("%d", userID))
	params.Set("metadata[username]", user.Username)
	params.Set("metadata[quota]", fmt.Sprintf("%d", form.Quota))
	params.Set("metadata[device]", form.Device)

	var session stripeSessionResponse
	if err := stripeHTTP(http.MethodPost, "/v1/checkout/sessions", params, &session); err != nil {
		return nil, err
	}
	if session.ID == "" || session.URL == "" {
		return nil, fmt.Errorf("stripe checkout session response is incomplete")
	}

	if err := insertPaymentOrder(db, user, form, session.ID, stripeService); err != nil {
		return nil, err
	}

	return &session, nil
}

func getStripeSession(sessionID string) (*stripeSessionResponse, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("invalid stripe session id")
	}

	var session stripeSessionResponse
	if err := stripeHTTP(http.MethodGet, "/v1/checkout/sessions/"+url.PathEscape(sessionID), nil, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func isStripeSessionPaid(session *stripeSessionResponse) bool {
	if session == nil {
		return false
	}

	return strings.ToLower(strings.TrimSpace(session.PaymentStatus)) == "paid"
}

func validateStripePaidAmount(session *stripeSessionResponse, amount float32) error {
	if session == nil || session.AmountTotal <= 0 || strings.TrimSpace(session.Currency) == "" {
		return nil
	}

	expected := stripeMinorUnits(amount, session.Currency)
	if session.AmountTotal != expected {
		return fmt.Errorf("stripe paid amount mismatch")
	}

	return nil
}

func queryStripeLocalOrder(db *sql.DB, orderID string) (*stripeLocalOrder, error) {
	var order stripeLocalOrder
	err := globals.QueryRowDb(db, `
		SELECT user_id, service, amount, state FROM payment
		WHERE order_id = ?
	`, orderID).Scan(&order.UserID, &order.Service, &order.Amount, &order.State)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(order.Service)) != stripeService {
		return nil, fmt.Errorf("unsupported payment provider")
	}

	return &order, nil
}

func completeStripeSession(db *sql.DB, session *stripeSessionResponse) error {
	if !isStripeSessionPaid(session) {
		return fmt.Errorf("stripe session is not paid")
	}

	order, err := queryStripeLocalOrder(db, session.ID)
	if err != nil {
		return err
	}
	if err := validateStripePaidAmount(session, order.Amount); err != nil {
		return err
	}
	if order.State {
		return nil
	}

	return completePaymentOrder(db, session.ID, order.UserID, order.Amount)
}

func parseStripeSignatureHeader(header string) (string, []string, error) {
	parts := strings.Split(header, ",")
	timestamp := ""
	signatures := make([]string, 0)
	for _, part := range parts {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = strings.TrimSpace(value)
		case "v1":
			signatures = append(signatures, strings.TrimSpace(value))
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return "", nil, fmt.Errorf("invalid stripe signature header")
	}

	return timestamp, signatures, nil
}

func verifyStripeSignature(payload []byte, header string, secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("stripe webhook secret is not configured")
	}

	timestamp, signatures, err := parseStripeSignatureHeader(header)
	if err != nil {
		return err
	}

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid stripe signature timestamp")
	}
	if math.Abs(time.Since(time.Unix(seconds, 0)).Seconds()) > 300 {
		return fmt.Errorf("stripe signature timestamp is outside tolerance")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, signature := range signatures {
		if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(signature))) == 1 {
			return nil
		}
	}

	return fmt.Errorf("invalid stripe signature")
}

func StripeWebhookAPI(c *gin.Context) {
	conf := channel.SystemInstance.Payment.Stripe
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid body")
		return
	}

	if err := verifyStripeSignature(body, c.GetHeader("Stripe-Signature"), conf.GetWebhookSecret()); err != nil {
		c.String(http.StatusBadRequest, "invalid signature")
		return
	}

	var event stripeWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.String(http.StatusBadRequest, "invalid event")
		return
	}

	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		if strings.TrimSpace(event.Data.Object.ID) == "" {
			c.String(http.StatusBadRequest, "missing session")
			return
		}
		if err := completeStripeSession(utils.GetDBFromContext(c), &event.Data.Object); err != nil {
			if errors.Is(err, sql.ErrNoRows) ||
				strings.Contains(err.Error(), "unsupported payment provider") ||
				strings.Contains(err.Error(), "paid amount mismatch") ||
				strings.Contains(err.Error(), "not paid") {
				c.String(http.StatusOK, "ignored")
				return
			}

			c.String(http.StatusInternalServerError, "failed")
			return
		}
	}

	c.String(http.StatusOK, "success")
}
