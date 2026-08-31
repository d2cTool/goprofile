package domain

import "testing"

func TestValidateUserID(t *testing.T) {
	t.Parallel()
	if err := ValidateUserID("alice"); err != nil {
		t.Fatalf("valid user: %v", err)
	}
	if err := ValidateUserID(""); err == nil {
		t.Fatal("empty user must fail")
	}
	if err := ValidateUserID("bad\nuser"); err == nil {
		t.Fatal("control chars must fail")
	}
}

func TestNormalizeSizeAndFormat(t *testing.T) {
	t.Parallel()
	size, err := NormalizeSize("")
	if err != nil || size != SizeOriginal {
		t.Fatalf("default size: %s %v", size, err)
	}
	if _, err := NormalizeSize("512x512"); err == nil {
		t.Fatal("unknown size must fail")
	}
	format, err := NormalizeFormat("JPG")
	if err != nil || format != "jpeg" {
		t.Fatalf("jpg alias: %s %v", format, err)
	}
	if _, err := NormalizeFormat("gif"); err == nil {
		t.Fatal("gif must fail")
	}
}
