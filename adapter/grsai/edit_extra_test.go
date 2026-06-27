package grsai

import "testing"

func TestNormalizeImageSize(t *testing.T) {
	cases := map[string]string{
		"2k": "2K", "2K": "2K", "4k": "4K", "8k": "4K", "": "2K", "garbage": "2K",
	}
	for in, want := range cases {
		if got := normalizeImageSize(in); got != want {
			t.Fatalf("normalizeImageSize(%q)=%q want %q", in, got, want)
		}
	}
}
