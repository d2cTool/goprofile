package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHTTPAddr      = ":8080"
	DefaultPublicBaseURL = "http://localhost:8080"
	DefaultMaxUploadSize = 10 << 20
	DefaultS3Bucket      = "avatars"
	DefaultKafkaGroup    = "gophprofile-worker"
)

type Config struct {
	HTTPAddr       string
	PublicBaseURL  string
	MaxUploadBytes int64
	RateLimitRPS   int
	CORSOrigins    []string

	DatabaseURL string

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool
	S3Region    string

	KafkaBrokers []string
	KafkaGroup   string
	TopicUpload  string
	TopicDelete  string
	TopicProcess string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:       env("HTTP_ADDR", DefaultHTTPAddr),
		PublicBaseURL:  strings.TrimRight(env("PUBLIC_BASE_URL", DefaultPublicBaseURL), "/"),
		MaxUploadBytes: envInt64("MAX_UPLOAD_BYTES", DefaultMaxUploadSize),
		RateLimitRPS:   envInt("RATE_LIMIT_RPS", 20),
		CORSOrigins:    splitCSV(env("CORS_ORIGINS", "*")),
		DatabaseURL:    env("DATABASE_URL", ""),
		S3Endpoint:     env("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey:    env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:    env("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:       env("S3_BUCKET", DefaultS3Bucket),
		S3UseSSL:       envBool("S3_USE_SSL", false),
		S3Region:       env("S3_REGION", "us-east-1"),
		KafkaBrokers:   splitCSV(env("KAFKA_BROKERS", "localhost:9094")),
		KafkaGroup:     env("KAFKA_GROUP", DefaultKafkaGroup),
		TopicUpload:    env("KAFKA_TOPIC_UPLOAD", "avatar.uploaded"),
		TopicDelete:    env("KAFKA_TOPIC_DELETE", "avatar.deleted"),
		TopicProcess:   env("KAFKA_TOPIC_PROCESS", "avatar.process"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.KafkaBrokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS is required")
	}
	return cfg, nil
}

func ShutdownTimeout() time.Duration {
	return 10 * time.Second
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
