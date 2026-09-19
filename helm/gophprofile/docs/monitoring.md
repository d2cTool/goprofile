# Monitoring & Alerting

## Stack

| Signal | Tool | How it gets there |
| --- | --- | --- |
| Metrics | Prometheus (kube-prometheus-stack) | `ServiceMonitor` (`templates/server-servicemonitor.yaml`, `templates/worker-servicemonitor.yaml`) scrapes `/metrics` on both `server` and `worker` |
| Traces | Jaeger | App exports OTLP -> OTel Collector -> Jaeger (`otlp/jaeger` exporter) |
| Logs | Loki | App exports OTLP logs -> OTel Collector -> Loki (`otlphttp/loki` exporter) |
| Dashboards | Grafana | Sidecar-discovered `ConfigMap`s (`grafana_dashboard: "1"` label) ported from `docker/grafana-dashboard.json` / `grafana-node-exporter.json` |
| Alerts | Alertmanager | `PrometheusRule` (`templates/monitoring-prometheusrule.yaml`) ported from `docker/alerts.yml`, routed via `values.yaml`'s `kubePrometheusStack.alertmanager.config` |

Enable/disable each piece independently via `values.yaml`:

```yaml
kubePrometheusStack:
  enabled: true        # Prometheus + Alertmanager + Grafana + node-exporter + CRDs
monitoring:
  prometheusRule:
    enabled: true       # app alert rules
  grafanaDashboards:
    enabled: true        # app dashboards
loki:
  enabled: true
jaeger:
  enabled: true
```

## Metrics exposed by the app

From `internal/observability/metrics.go`:

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `http_requests_total` | counter | `method, path, status` | HTTP request outcomes |
| `http_request_duration_seconds` | histogram | `method, path` | HTTP latency |
| `avatars_uploads_total` | counter | `status` | Upload attempts (`ok`/`error`) |
| `avatars_upload_duration_seconds` | histogram | `status` | Upload latency |
| `avatars_deletes_total` | counter | `status` | Delete outcomes |
| `avatars_processed_total` | counter | `kind, status` | Worker thumbnail jobs |
| `avatars_storage_bytes` | gauge | - | Total bytes stored |
| `avatars_outbox_unpublished` | gauge | - | Outbox backlog (should trend to 0) |
| `avatars_kafka_messages_total` | counter | `op, topic, status` | Kafka produce/consume outcomes |

## Alerts (`monitoring-prometheusrule.yaml`)

| Alert | Condition | Severity |
| --- | --- | --- |
| `HighErrorRate` | >10% of avatar uploads fail over 5m | warning |
| `HighResponseTime` | Upload p95 latency > 5s over 2m | critical |
| `HighHTTPErrorRate` | >10% of HTTP requests return 5xx over 5m | warning |
| `HighHTTPResponseTime` | HTTP p95 latency > 2s over 2m | critical |
| `GophProfileServerDown` | No successful scrape of `server` for 2m | critical |
| `GophProfileWorkerDown` | No successful scrape of `worker` for 2m | critical |

`GophProfileServerDown`/`WorkerDown` are new relative to `docker/alerts.yml`,
added because the Kubernetes deployment can lose all replicas of a component
independently of any single container's own error rate (e.g. a bad rollout,
an `ImagePullBackOff`, or the whole Deployment scaled to zero) - a case
docker-compose's single-container-per-service model didn't need to guard
against separately.

Wire these into a real receiver (Slack/PagerDuty/email/etc.) by overriding
`kubePrometheusStack.alertmanager.config.receivers` - the default value ships
a no-op `default` receiver, matching `docker/alertmanager.yml`.

## Runbook pointers

* **HighErrorRate / HighHTTPErrorRate** - check `kubectl logs deploy/<release>-server`
  and Jaeger for failing spans; likely S3 or Postgres connectivity (check
  `/health`).
* **HighResponseTime / HighHTTPResponseTime** - check HPA status
  (`kubectl get hpa`) - the deployment may be under-scaled for current load;
  check `avatars_outbox_unpublished` for a growing backlog.
* **GophProfileWorkerDown** - check `kubectl get pods -l app.kubernetes.io/component=worker`
  and Kafka consumer group lag; a stuck worker blocks thumbnail generation
  but does not affect uploads (they stay in `processing`).

## Dashboards

* **GophProfile overview** (`dashboards/gophprofile.json`) - request rate,
  error rate, latency percentiles, upload/delete counters, outbox backlog.
* **Node Exporter** (`dashboards/node-exporter.json`) - standard cluster
  node CPU/memory/disk/network panels, backed by the node-exporter DaemonSet
  bundled in kube-prometheus-stack.

Both are auto-provisioned into a "GophProfile" Grafana folder by the
chart's sidecar-discovered `ConfigMap`s; no manual import needed.
