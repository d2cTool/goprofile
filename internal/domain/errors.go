package domain

import "errors"

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
)
