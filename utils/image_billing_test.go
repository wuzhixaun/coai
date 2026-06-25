package utils

import (
	"chat/globals"
	"math"
	"testing"
)

// mockCharge 实现 utils.Charge 接口，用于隔离测试计费逻辑。
type mockCharge struct {
	typ    string
	output float32
}

func (m mockCharge) GetType() string             { return m.typ }
func (m mockCharge) GetModels() []string         { return nil }
func (m mockCharge) GetInput() float32           { return 0 }
func (m mockCharge) GetOutput() float32          { return m.output }
func (m mockCharge) SupportAnonymous() bool      { return false }
func (m mockCharge) IsBilling() bool             { return m.typ != globals.NonBilling }
func (m mockCharge) IsBillingType(t string) bool { return m.typ == t }
func (m mockCharge) GetLimit() float32           { return m.output }

func approxEqual(a, b float32) bool {
	return math.Abs(float64(a-b)) < 1e-4
}

func TestCountOutputToken_ImageBilling(t *testing.T) {
	charge := mockCharge{typ: globals.ImageBilling, output: 0.22}
	if got := CountOutputToken(charge, 3); !approxEqual(got, 0.66) {
		t.Fatalf("image-billing 3 images: got %v, want 0.66", got)
	}
	if got := CountOutputToken(charge, 1); !approxEqual(got, 0.22) {
		t.Fatalf("image-billing 1 image: got %v, want 0.22", got)
	}
}

func TestBufferGetQuota_ImageBilling_PerImage(t *testing.T) {
	charge := mockCharge{typ: globals.ImageBilling, output: 0.22}
	buf := NewBuffer("jimeng-seedream-4.6", nil, charge)
	// 每张图片以一个 markdown chunk 写入。
	buf.Write("![image](/storage/results/a.png)")
	buf.Write("![image](/storage/results/b.png)")
	buf.Write("![image](/storage/results/c.png)")

	if got := buf.GetQuota(); !approxEqual(got, 0.66) {
		t.Fatalf("GetQuota for 3 images: got %v, want 0.66", got)
	}
	// 记录用额度也应按张计费，且与扣费一致（避免按文本 token 记账）。
	if got := buf.GetRecordQuota(); !approxEqual(got, 0.66) {
		t.Fatalf("GetRecordQuota for 3 images: got %v, want 0.66", got)
	}
}

func TestBufferGetQuota_TimesBilling_FlatRegression(t *testing.T) {
	// 回归：times-billing 仍是「每请求固定费」，不受写入次数影响。
	charge := mockCharge{typ: globals.TimesBilling, output: 0.5}
	buf := NewBuffer("some-chat-model", nil, charge)
	buf.Write("chunk1")
	buf.Write("chunk2")
	buf.Write("chunk3")
	if got := buf.GetQuota(); !approxEqual(got, 0.5) {
		t.Fatalf("times-billing flat fee: got %v, want 0.5", got)
	}
}
