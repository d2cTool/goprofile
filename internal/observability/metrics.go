package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests by method, route and status",
	}, []string{"method", "path", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	UploadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "avatars_uploads_total",
		Help: "Total number of avatar uploads",
	}, []string{"status"})

	UploadDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "avatars_upload_duration_seconds",
		Help:    "Avatar upload duration",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"status"})

	DeletesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "avatars_deletes_total",
		Help: "Total number of avatar deletes",
	}, []string{"status"})

	ProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "avatars_processed_total",
		Help: "Worker processing results",
	}, []string{"kind", "status"})

	StorageUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avatars_storage_bytes",
		Help: "Total storage used by avatars",
	})

	OutboxUnpublished = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avatars_outbox_unpublished",
		Help: "Unpublished outbox events seen on last flush",
	})

	KafkaMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "avatars_kafka_messages_total",
		Help: "Kafka produce and consume results",
	}, []string{"op", "topic", "status"})
)

func StatusOK(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
