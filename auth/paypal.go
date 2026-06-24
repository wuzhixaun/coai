package auth

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"chat/channel"
	"chat/globals"
	"chat/utils"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

const paypalService = "paypal"

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
