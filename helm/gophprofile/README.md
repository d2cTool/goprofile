# GophProfile Helm chart

Converts [`docker/docker-compose.yml`](../../docker/docker-compose.yml) into a
Kubernetes deployment: the `server`/`worker` Go app, PostgreSQL, MinIO,
Kafka, and the OTel Collector -> Loki/Jaeger/Prometheus/Alertmanager/Grafana
observability pipeline.

See [`docs/architecture.md`](docs/architecture.md) for a diagram,
[`docs/monitoring.md`](docs/monitoring.md) for dashboards/alerts, and
[`docs/openapi.yaml`](docs/openapi.yaml) for the REST API.

## Prerequisites

* Kubernetes >= 1.26
* Helm >= 3.14 (chart uses `autoscaling/v2`, `policy/v1`)
* A default `StorageClass` supporting `ReadWriteOnce` (PostgreSQL, MinIO,
  Kafka and Loki all use PVCs)
* An Ingress controller (e.g. ingress-nginx) if `server.ingress.enabled: true`
* Cluster-admin access on first install, to install the Prometheus Operator
  CRDs (`kube-prometheus-stack` dependency)

## Quickstart

```bash
# 1. Build the app image (from repo root) and load it into your cluster.
docker build -f docker/Dockerfile -t gophprofile:latest .
kind load docker-image gophprofile:latest          # kind
# minikube image load gophprofile:latest            # minikube

# 2. Fetch the kube-prometheus-stack dependency.
cd helm/gophprofile
helm dependency update .

# 3. Install.
helm install gophprofile . \
  --namespace gophprofile --create-namespace \
  --set server.ingress.host=gophprofile.local

# 4. Watch it come up.
kubectl -n gophprofile get pods -w
```

Then follow the printed `NOTES.txt` (also viewable any time with
`helm get notes gophprofile -n gophprofile`) for port-forward commands to the
app, Grafana, Prometheus, Alertmanager and Jaeger.

Run the bundled smoke test once everything is `Ready`:

```bash
helm test gophprofile -n gophprofile
```

## Configuration

Full reference in [`values.yaml`](values.yaml) (every key is commented).
Highlights:

| Key | Default | Purpose |
| --- | --- | --- |
| `image.repository` / `image.tag` | `gophprofile` / `latest` | App image built from `docker/Dockerfile` |
| `server.autoscaling.*` / `worker.autoscaling.*` | 2-10 / 2-8 replicas, 70-80% CPU/mem | HPA targets |
| `server.ingress.host` | `gophprofile.local` | Public hostname |
| `secrets.postgresPassword` / `secrets.s3SecretKey` | auto-generated | Leave empty to have the chart generate & persist random values |
| `secrets.existingSecret` | `""` | Point at a Secret you manage externally (Vault/External Secrets) instead |
| `postgresql.enabled` / `minio.enabled` / `kafka.enabled` | `true` | Set `false` to use managed/external services instead (update `config.*`/`secrets.*` accordingly) |
| `kubePrometheusStack.enabled` | `true` | Installs Prometheus Operator + Prometheus + Alertmanager + Grafana + node-exporter |
| `loki.enabled` / `jaeger.enabled` / `otelCollector.enabled` | `true` | Logs/traces pipeline |

See [`values-production.yaml`](values-production.yaml) for an example
overlay (custom image/registry, TLS ingress, externally-managed secret,
managed Postgres/MinIO/Kafka instead of the in-chart ones):

```bash
helm upgrade --install gophprofile . -f values-production.yaml -n gophprofile
```

## Acceptance checklist

| Requirement | How this chart satisfies it |
| --- | --- |
| All components deploy to Kubernetes | `helm install` brings up app + Postgres + MinIO + Kafka + OTel Collector + Loki + Jaeger + Prometheus/Alertmanager/Grafana |
| Health checks work | `server`: `startupProbe`/`readinessProbe`/`livenessProbe` on `GET /health` (checks Postgres/S3/Kafka); `worker`: TCP probe on the metrics port; every infra component has its own liveness/readiness probe |
| Service reachable via Ingress | `templates/server-ingress.yaml`, enabled by default |
| HPA scales pods correctly | `templates/server-hpa.yaml` / `worker-hpa.yaml` (CPU + memory, `autoscaling/v2`) |
| Load balancing across replicas | Standard `Service` (round-robin/iptables/IPVS) in front of each Deployment; `topologySpreadConstraints` + preferred anti-affinity spread replicas across nodes |
| Graceful shutdown | `server`: `preStop` sleep + `terminationGracePeriodSeconds: 30` gives the Endpoints controller time to deregister the pod before `srv.Shutdown(ctx)` drains in-flight requests (`cmd/server/main.go`); `worker`: `ctx.Done()` stops the Kafka consumer loop before `terminationGracePeriodSeconds` expires |
| Metrics collected in Kubernetes | `ServiceMonitor` per component, scraped by kube-prometheus-stack's Prometheus |
| Dashboards show cluster state | Grafana sidecar-provisioned dashboards (app dashboard + Node Exporter dashboard) |
| ServiceMonitor configured correctly | `templates/server-servicemonitor.yaml`, `templates/worker-servicemonitor.yaml` |
| NetworkPolicy restricts traffic | One `NetworkPolicy` per component (app + every infra piece); default-deny via explicit `podSelector` ingress/egress lists (see `docs/architecture.md`) |
| Secrets for sensitive data | `templates/secret-app.yaml` (DB/S3 credentials); never in the ConfigMap or image |
| Non-root containers | Every container sets `runAsNonRoot: true` + explicit non-root uid/gid (verified against each upstream image's own Dockerfile - see comments in the relevant template) and drops all Linux capabilities |
| Resource limits | `requests`/`limits` set per component in `values.yaml` |

Two items from the broader assignment brief are **service-level, not
chart-level**, and are called out here rather than silently assumed done:

* **Rate limiting** and **structured error handling** already exist in the
  Go service (`internal/middleware`, `internal/handlers/json.go`) - the
  chart just exposes their config (`RATE_LIMIT_RPS`, `CORS_ORIGINS`, etc.)
  via the `ConfigMap`.
* **Circuit breaker for external dependencies** (Postgres/S3/Kafka) is
  **not yet implemented** in the application code as of this chart's
  authoring - `internal/repository`, `internal/storage` and
  `internal/broker` call out directly with no breaker/backoff wrapper
  beyond the worker's existing retry logic. Adding one (e.g.
  `sony/gobreaker`) is an application change tracked separately from this
  chart.

## Security notes

* Default images (`postgres:16-alpine`, `minio/minio`, `apache/kafka`,
  `otel/opentelemetry-collector`, `grafana/loki`, `jaegertracing/all-in-one`)
  are pinned to the same tags docker-compose uses; bump them deliberately.
* `secrets.postgresPassword`/`secrets.s3SecretKey` are generated with
  `randAlphaNum 32` on first install and re-read via `lookup` on upgrade, so
  they don't rotate on every `helm upgrade`. For a GitOps workflow (Argo CD
  etc., where `lookup` isn't reliable at diff-time) set `secrets.existingSecret`
  instead and manage the Secret with your own tooling.
* `NetworkPolicy` resources assume a CNI that enforces them (Calico, Cilium,
  etc.) - on a CNI without enforcement (e.g. plain kindnet) these are
  advisory only.

## Uninstall

```bash
helm uninstall gophprofile -n gophprofile
kubectl -n gophprofile delete pvc -l app.kubernetes.io/instance=gophprofile
```

(PVCs are not deleted automatically by `helm uninstall`, to avoid data loss.)

## Known limitations

* PostgreSQL, MinIO and Kafka run as a single non-HA replica each, exactly
  matching `docker-compose.yml`. This chart does not implement replication,
  backups, or multi-broker Kafka - use managed services (RDS, S3, MSK/managed
  Kafka) for a real production deployment and set the corresponding
  `<component>.enabled: false`.
* `kube-prometheus-stack` is a large dependency (Prometheus Operator, CRDs,
  kube-state-metrics, node-exporter DaemonSet). If your cluster already runs
  one, set `kubePrometheusStack.enabled: false` - this chart's own
  `ServiceMonitor`/`PrometheusRule`/dashboard `ConfigMap`s will still be
  picked up by it.
