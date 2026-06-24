package auth

import (
	"strings"
	"testing"
)

func TestAlipayAmount(t *testing.T) {
	if got := alipayAmount(10); got != 1.0 {
		t.Fatalf("alipayAmount(10) = %v, want 1.0", got)
	}
	if got := alipayAmount(200); got != 20.0 {
		t.Fatalf("alipayAmount(200) = %v, want 20.0", got)
	}
}

func TestCreateAlipayOrderID(t *testing.T) {
	id := createAlipayOrderID("alice")
	if !strings.HasPrefix(id, "alipay_") {
		t.Fatalf("order id %q missing alipay_ prefix", id)
	}
	if len(id) != len("alipay_")+24 {
		t.Fatalf("order id %q unexpected length %d", id, len(id))
	}
	if createAlipayOrderID("alice") == id {
		t.Fatalf("order id should be unique per call")
	}
}
