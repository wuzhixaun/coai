package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"
	"time"
)

func signedStripeHeader(payload []byte, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	rawTimestamp := strconv.FormatInt(timestamp, 10)
	mac.Write([]byte(rawTimestamp))
	mac.Write([]byte("."))
	mac.Write(payload)

	return fmt.Sprintf("t=%s,v1=%s", rawTimestamp, hex.EncodeToString(mac.Sum(nil)))
}

func TestVerifyStripeSignature(t *testing.T) {
	payload := []byte(`{"id":"evt_test"}`)
	secret := "whsec_test"
	header := signedStripeHeader(payload, secret, time.Now().Unix())

	if err := verifyStripeSignature(payload, header, secret); err != nil {
		t.Fatalf("expected signature to verify: %v", err)
	}
}

func TestVerifyStripeSignatureRejectsBadPayload(t *testing.T) {
	payload := []byte(`{"id":"evt_test"}`)
	secret := "whsec_test"
	header := signedStripeHeader(payload, secret, time.Now().Unix())

	if err := verifyStripeSignature([]byte(`{"id":"evt_tampered"}`), header, secret); err == nil {
		t.Fatal("expected tampered payload to fail signature verification")
	}
}

func TestStripeMinorUnits(t *testing.T) {
	if got := stripeMinorUnits(1.23, "usd"); got != 123 {
		t.Fatalf("expected usd minor units to be 123, got %d", got)
	}
	if got := stripeMinorUnits(123, "jpy"); got != 123 {
		t.Fatalf("expected jpy minor units to be 123, got %d", got)
	}
}
