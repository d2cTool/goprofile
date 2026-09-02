package domain

import (
	"errors"
	"fmt"
)

var (
	ErrAvatarNotFound     = errors.New("avatar not found")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidUserID      = errors.New("invalid user id")
	ErrFileRequired       = errors.New("file is required")
	ErrFileTooLarge       = errors.New("file too large")
	ErrInvalidFileFormat  = errors.New("invalid file format")
	ErrInvalidImage       = errors.New("invalid image")
	ErrAlreadyProcessed   = errors.New("already processed")
	ErrAlreadyDeleted     = errors.New("already deleted")
	ErrInvalidSizeParam   = errors.New("invalid size")
	ErrInvalidFormatParam = errors.New("invalid format")
	ErrPermanent          = errors.New("permanent failure")
	ErrImageTooLarge      = errors.New("image dimensions too large")
)

func Permanent(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPermanent) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrPermanent, err)
}

func IsPermanent(err error) bool {
	return errors.Is(err, ErrPermanent) ||
		errors.Is(err, ErrInvalidFileFormat) ||
		errors.Is(err, ErrInvalidImage) ||
		errors.Is(err, ErrImageTooLarge) ||
		errors.Is(err, ErrInvalidSizeParam) ||
		errors.Is(err, ErrInvalidFormatParam)
}
