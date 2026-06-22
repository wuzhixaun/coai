package jimengapi

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestVolcenginePostSignatureDeterministic(t *testing.T) {
	ts, err := time.Parse("20060102T150405Z", "20250329T180937Z")
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"Limit":10,"BillPeriod":"2023-08"}`)
	query := url.Values{}
	query.Set("Action", "ListBill")
	query.Set("Version", "2022-01-01")

	// 占位测试密钥（非真实凭证）——仅用于校验签名算法的确定性输出。
	headers := signHeadersWithScope(http.MethodPost, "/", query, map[string]string{
		"Host": "billing.volcengineapi.com",
	}, body,
		"test-access-key-id-not-a-secret",
		"test-secret-access-key-not-a-secret",
		ts,
		"cn-beijing",
		"billing",
	)

	auth := headers["Authorization"]
	if !strings.Contains(auth, "SignedHeaders=host;x-date") {
		t.Fatalf("unexpected signed headers: %s", auth)
	}
	if !strings.Contains(auth, "Signature=23c760c81af856b1cc3d978f7c66ad90c097abc2add030c36a35193e133a5a0e") {
		t.Fatalf("unexpected authorization: %s", auth)
	}
}

func TestSeedream46ModelSpec(t *testing.T) {
	spec, ok := GetModelSpec("jimeng-seedream-4.6")
	if !ok {
		t.Fatal("jimeng-seedream-4.6 spec missing")
	}
	if spec.ReqKey != "jimeng_seedream46_cvtob" {
		t.Fatalf("unexpected req_key: %s", spec.ReqKey)
	}
	if spec.DefaultScale != 50 {
		t.Fatalf("unexpected default scale: %g", spec.DefaultScale)
	}
}
