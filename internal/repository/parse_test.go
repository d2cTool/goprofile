package repository

import (
	"testing"

	"github.com/d2cTool/goprofile/internal/domain"
)

func TestJSONBytes(t *testing.T) {
	t.Parallel()
	b, err := jsonBytes(map[string]string{"100x100": "k"})
	if err != nil {
		t.Fatal(err)
	}
	got := domain.ParseThumbnailKeys(b)
	if got["100x100"] != "k" {
		t.Fatalf("%s", b)
	}
	b, err = jsonBytes(nil)
	if err != nil || string(b) != "{}" {
		t.Fatalf("%s %v", b, err)
	}
}
