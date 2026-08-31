package domain

import (
	"errors"
	"testing"
)

func TestIsPermanent(t *testing.T) {
	t.Parallel()
	if !IsPermanent(Permanent(errors.New("bad json"))) {
		t.Fatal("wrapped")
	}
	if !IsPermanent(ErrInvalidImage) {
		t.Fatal("invalid image")
	}
	if IsPermanent(errors.New("connection reset")) {
		t.Fatal("transient")
	}
}
