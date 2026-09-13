package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("KAFKA_BROKERS", "kafka:9092, kafka2:9092")
	t.Setenv("S3_USE_SSL", "true")
	t.Setenv("RATE_LIMIT_RPS", "7")
	t.Setenv("MAX_UPLOAD_BYTES", "2048")
	t.Setenv("OTEL_SERVICE_NAME", "gophprofile-test")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("METRICS_ADDR", ":9191")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.S3UseSSL || cfg.RateLimitRPS != 7 || len(cfg.KafkaBrokers) != 2 || cfg.MaxUploadBytes != 2048 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.ServiceName != "gophprofile-test" || cfg.OTLPEndpoint != "http://otel-collector:4318" || cfg.LogLevel != "debug" || cfg.MetricsAddr != ":9191" {
		t.Fatalf("otel %+v", cfg)
	}
}

func TestLoadRequiresDatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestSplitAndEnvHelpers(t *testing.T) {
	t.Parallel()
	if got := splitCSV("a, b,,c"); len(got) != 3 {
		t.Fatalf("%v", got)
	}
	if envBool("MISSING_BOOL_XYZ", true) != true {
		t.Fatal("fallback bool")
	}
	if ShutdownTimeout() <= 0 {
		t.Fatal("shutdown timeout")
	}
}
