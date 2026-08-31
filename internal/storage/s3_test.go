package storage

import (
	"testing"

	"github.com/d2cTool/goprofile/internal/config"
)

func TestNewS3(t *testing.T) {
	s, err := NewS3(config.Config{
		S3Endpoint:  "localhost:9000",
		S3AccessKey: "minioadmin",
		S3SecretKey: "minioadmin",
		S3Bucket:    "avatars",
	})
	if err != nil || s == nil || s.bucket != "avatars" {
		t.Fatalf("%v %#v", err, s)
	}
}
