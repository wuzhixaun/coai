package manager

import (
	"fmt"
	"testing"
)

func TestParseJimengError(t *testing.T) {
	cases := []struct {
		name    string
		message string
		code    int
		request string
	}{
		{
			name:    "full jimeng error",
			message: "jimeng submit failed: code=50411 message=input image is invalid request_id=20240601abcdef",
			code:    50411,
			request: "20240601abcdef",
		},
		{
			name:    "code only, no request id",
			message: "jimeng query failed: code=50500 message=internal error",
			code:    50500,
			request: "",
		},
		{
			name:    "request id before message tail",
			message: "code=429 message=rate limited request_id=req-xyz extra tail",
			code:    429,
			request: "req-xyz",
		},
		{
			name:    "no structured fields",
			message: "context deadline exceeded",
			code:    0,
			request: "",
		},
		{
			name:    "empty",
			message: "",
			code:    0,
			request: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, request := parseJimengError(tc.message)
			if code != tc.code {
				t.Errorf("code: got %d, want %d", code, tc.code)
			}
			if request != tc.request {
				t.Errorf("request_id: got %q, want %q", request, tc.request)
			}
		})
	}
}

func TestImageOutcome(t *testing.T) {
	status, message, code, request := imageOutcome(nil)
	if status != imageStatusSuccess {
		t.Errorf("nil err status: got %q, want %q", status, imageStatusSuccess)
	}
	if message != "" || code != 0 || request != "" {
		t.Errorf("nil err should yield empty diagnostics, got message=%q code=%d request=%q", message, code, request)
	}

	err := fmt.Errorf("jimeng submit failed: code=50411 message=bad input request_id=rid-1")
	status, message, code, request = imageOutcome(err)
	if status != imageStatusFailed {
		t.Errorf("err status: got %q, want %q", status, imageStatusFailed)
	}
	if message != err.Error() {
		t.Errorf("message: got %q, want full error text", message)
	}
	if code != 50411 {
		t.Errorf("code: got %d, want 50411", code)
	}
	if request != "rid-1" {
		t.Errorf("request_id: got %q, want rid-1", request)
	}
}

func TestImageOutcomeMessageTruncation(t *testing.T) {
	long := make([]byte, 0, 600)
	for i := 0; i < 600; i++ {
		long = append(long, 'x')
	}
	_, message, _, _ := imageOutcome(fmt.Errorf("%s", string(long)))
	if len(message) > imageMessageMaxLen {
		t.Errorf("message length %d exceeds cap %d", len(message), imageMessageMaxLen)
	}
}
