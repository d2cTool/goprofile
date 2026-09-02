package domain

type ProcessingOp struct {
	Name   string `json:"name"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type AvatarUploadEvent struct {
	EventID  string `json:"event_id"`
	AvatarID string `json:"avatar_id"`
	UserID   string `json:"user_id"`
	S3Key    string `json:"s3_key"`
}

type AvatarProcessEvent struct {
	EventID    string         `json:"event_id"`
	AvatarID   string         `json:"avatar_id"`
	Operations []ProcessingOp `json:"operations"`
}

type AvatarDeleteEvent struct {
	EventID  string   `json:"event_id"`
	AvatarID string   `json:"avatar_id"`
	S3Keys   []string `json:"s3_keys"`
}

func UploadEventID(avatarID string) string {
	return "upload:" + avatarID
}

func DeleteEventID(avatarID string) string {
	return "delete:" + avatarID
}

func ProcessEventID(avatarID string) string {
	return "process:" + avatarID
}
