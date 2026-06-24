package auth

import (
	"strings"
	"testing"
)

func TestWechatAmountFen(t *testing.T) {
	if got := wechatAmountFen(10); got != 100 {
		t.Fatalf("wechatAmountFen(10) = %d, want 100", got)
	}
	if got := wechatAmountFen(200); got != 2000 {
		t.Fatalf("wechatAmountFen(200) = %d, want 2000", got)
	}
}

func TestCreateWechatOrderID(t *testing.T) {
	id := createWechatOrderID("bob")
	if !strings.HasPrefix(id, "wxpay_") {
		t.Fatalf("order id %q missing wxpay_ prefix", id)
	}
	if createWechatOrderID("bob") == id {
		t.Fatalf("order id should be unique")
	}
}
