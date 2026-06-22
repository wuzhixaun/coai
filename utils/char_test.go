package utils

import "testing"

func TestExtractImagesFromMarkdownSupportsLocalResultURLs(t *testing.T) {
	got := ExtractImagesFromMarkdown("done ![image](/storage/results/jimeng.png)")
	if len(got) != 1 || got[0] != "/storage/results/jimeng.png" {
		t.Fatalf("unexpected images: %#v", got)
	}
}
