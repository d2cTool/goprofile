package domain

import (
	"strings"
	"unicode"
)

func ValidateUserID(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || len(userID) > 255 {
		return ErrInvalidUserID
	}
	for _, r := range userID {
		if unicode.IsControl(r) {
			return ErrInvalidUserID
		}
	}
	return nil
}

func NormalizeSize(size string) (string, error) {
	size = strings.TrimSpace(strings.ToLower(size))
	if size == "" {
		return SizeOriginal, nil
	}
	if _, ok := AllowedSizes[size]; !ok {
		return "", ErrInvalidSizeParam
	}
	return size, nil
}

func NormalizeFormat(format string) (string, error) {
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		return "", nil
	}
	if format == "jpg" {
		format = "jpeg"
	}
	if _, ok := AllowedFormats[format]; !ok {
		return "", ErrInvalidFormatParam
	}
	return format, nil
}
