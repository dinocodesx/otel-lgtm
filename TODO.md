# OpenTelemetry LGTM Stack Implementation TODO

## Overview

This TODO outlines the implementation of OpenTelemetry instrumentation for a Gorilla Mux Go HTTP API with the LGTM stack (Loki, Grafana, Tempo, and Mimir/Prometheus).

**LGTM Stack Components:**

- **L**oki: Log aggregation system
- **G**rafana: Visualization and dashboards
- **T**empo: Distributed tracing backend
- **M**imir/Prometheus: Metrics storage and querying

**Current Stack:**

- Go HTTP API using Gorilla Mux
- OpenTelemetry SDK for Go
- Docker containers for LGTM stack components

---

## Phase 1: Project Setup and Dependencies

### 1.1 Update Go Module Dependencies

- [x] Add OpenTelemetry core dependencies to `go.mod`:

  ```bash
  go get go.opentelemetry.io/otel
  go get go.opentelemetry.io/otel/sdk/trace
  go get go.opentelemetry.io/otel/sdk/metric
  go get go.opentelemetry.io/otel/sdk/log
  go get go.opentelemetry.io/otel/sdk/resource
  go get go.opentelemetry.io/otel/propagation
  ```

- [x] Add OpenTelemetry instrumentation libraries:

  ```bash
  go get go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux
  go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
  go get go.opentelemetry.io/contrib/bridges/otelslog
  ```

- [x] Add OpenTelemetry exporters for LGTM stack:
  ```bash
  go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
  go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
  go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp
  go get go.opentelemetry.io/otel/exporters/prometheus
  ```

### 1.2 Create Docker Infrastructure

- [x] Create `docker-compose.yml` with LGTM stack components:

  - [x] Grafana (port 3000)
  - [x] Prometheus (port 9090)
  - [x] Loki (port 3100)
  - [x] Tempo (port 3200, 4317 for OTLP)
  - [x] OpenTelemetry Collector (port 4317, 4318)

- [x] Create configuration files:
  - [x] `config/prometheus.yml` - Prometheus scraping configuration
  - [x] `config/loki.yml` - Loki configuration
  - [x] `config/tempo.yml` - Tempo configuration
  - [x] `config/grafana/datasources.yml` - Grafana data sources
  - [x] `config/otel-collector.yml` - OTEL Collector configuration

---

## Phase 2: OpenTelemetry SDK Implementation

### 2.1 Create OpenTelemetry Setup Module

- [x] Create `app/otel.go` with:
  - [x] Resource configuration (service name, version, environment)
  - [x] Trace provider setup with OTLP HTTP exporter to Tempo
  - [x] Metrics provider setup with Prometheus and OTLP exporters
  - [x] Logging provider setup with OTLP HTTP exporter to Loki
  - [x] Graceful shutdown handling

### 2.2 Configure Exporters

- [ ] **Traces to Tempo:**

  - [ ] OTLP HTTP exporter pointing to Tempo endpoint (http://tempo:4318/v1/traces)
  - [ ] Batch span processor configuration
  - [ ] Trace sampling configuration

- [ ] **Metrics to Prometheus:**

  - [ ] Prometheus exporter for scraping endpoint (/metrics)
  - [ ] OTLP HTTP exporter to collector (optional)
  - [ ] Periodic reader configuration (30s interval)

- [ ] **Logs to Loki:**
  - [ ] OTLP HTTP exporter pointing to collector
  - [ ] Batch log processor configuration
  - [ ] Structured logging integration

### 2.3 Add Resource Attributes

- [ ] Configure standard resource attributes:
  - [ ] `service.name=otel-lgtm-api`
  - [ ] `service.version` (from build/git)
  - [ ] `deployment.environment` (dev/staging/prod)
  - [ ] `service.instance.id` (unique instance identifier)

---

## Phase 3: HTTP API Instrumentation

### 3.1 Instrument Gorilla Mux Router

- [ ] Update `main.go` to integrate OpenTelemetry:

  - [ ] Initialize OpenTelemetry SDK on startup
  - [ ] Add graceful shutdown for telemetry exporters
  - [ ] Handle context propagation properly

- [ ] Replace HTTP router setup with instrumented version:
  - [ ] Use `otelmux.Middleware()` for automatic HTTP instrumentation
  - [ ] Configure route-based span naming
  - [ ] Add custom attributes (HTTP method, status code, user agent)

### 3.2 Add Custom Instrumentation

- [ ] Create tracer and meter instances:

  ```go
  var (
    tracer = otel.Tracer("otel-lgtm-api")
    meter  = otel.Meter("otel-lgtm-api")
    logger = otelslog.NewLogger("otel-lgtm-api")
  )
  ```

- [ ] Instrument existing endpoints with custom spans:
  - [ ] Health check endpoint
  - [ ] Error simulation endpoints
  - [ ] Business logic operations

### 3.3 Add Custom Metrics

- [ ] HTTP request duration histogram
- [ ] HTTP request counter by method and status code
- [ ] Active connections gauge
- [ ] Business-specific metrics (e.g., response scenarios)
- [ ] Error rate and latency percentiles

### 3.4 Add Structured Logging

- [ ] Replace existing log statements with OpenTelemetry logger
- [ ] Add trace correlation IDs to logs
- [ ] Include relevant context in log entries
- [ ] Implement different log levels (DEBUG, INFO, WARN, ERROR)

---

## Phase 4: LGTM Stack Configuration

### 4.1 Prometheus Configuration

- [ ] Configure scraping of Go application metrics endpoint
- [ ] Set appropriate scrape intervals (15s-30s)
- [ ] Add service discovery for dynamic targets
- [ ] Configure retention and storage settings

### 4.2 Tempo Configuration

- [ ] Configure OTLP receivers (gRPC and HTTP)
- [ ] Set up storage backend (local for development)
- [ ] Configure trace retention policies
- [ ] Enable search and service graphs

### 4.3 Loki Configuration

- [ ] Configure ingestion endpoints
- [ ] Set up log retention and storage
- [ ] Configure log parsing and labeling rules
- [ ] Enable structured metadata extraction

### 4.4 Grafana Dashboard Setup

- [ ] Configure data sources:

  - [ ] Prometheus for metrics
  - [ ] Tempo for traces
  - [ ] Loki for logs

- [ ] Create dashboards:

  - [ ] **Application Overview Dashboard:**

    - [ ] Request rate, latency, error rate (RED metrics)
    - [ ] HTTP status code distribution
    - [ ] Response time percentiles
    - [ ] Active connections and throughput

  - [ ] **Infrastructure Dashboard:**

    - [ ] Go runtime metrics (goroutines, memory, GC)
    - [ ] System metrics (CPU, memory, disk)
    - [ ] Container metrics (if using Docker)

  - [ ] **Traces Dashboard:**
    - [ ] Service map and dependencies
    - [ ] Trace search and filtering
    - [ ] Latency analysis and bottlenecks
    - [ ] Error traces investigation

### 4.5 OpenTelemetry Collector Setup

- [ ] Configure receivers for OTLP data
- [ ] Set up processors for data transformation:
  - [ ] Batch processor for performance
  - [ ] Memory limiter to prevent OOM
  - [ ] Resource processor for attribute addition
- [ ] Configure exporters:
  - [ ] Tempo for traces
  - [ ] Loki for logs
  - [ ] Prometheus for metrics (optional)

---

## Phase 5: Testing and Validation

### 5.1 Local Development Testing

- [ ] Start LGTM stack with `docker-compose up`
- [ ] Build and run Go application
- [ ] Verify telemetry data flow:
  - [ ] Check metrics endpoint (`/metrics`)
  - [ ] Generate test traffic to create traces
  - [ ] Verify logs are structured and contain trace IDs

### 5.2 Grafana Verification

- [ ] Access Grafana UI (http://localhost:3000)
- [ ] Verify data source connections:
  - [ ] Prometheus metrics query test
  - [ ] Tempo trace search test
  - [ ] Loki log query test
- [ ] Test dashboards functionality:
  - [ ] Real-time metrics display
  - [ ] Trace-to-logs correlation
  - [ ] Logs-to-traces correlation

### 5.3 Integration Testing

- [ ] **Distributed Tracing Test:**

  - [ ] Create multi-step request flow
  - [ ] Verify trace propagation across HTTP boundaries
  - [ ] Test error trace correlation

- [ ] **Metrics Collection Test:**

  - [ ] Generate various HTTP response codes
  - [ ] Verify custom business metrics
  - [ ] Test metrics aggregation and alerting

- [ ] **Logging Integration Test:**
  - [ ] Generate structured logs with trace context
  - [ ] Test log level filtering
  - [ ] Verify log-to-trace correlation in Grafana

---

## Phase 6: Production Readiness

### 6.1 Performance Optimization

- [ ] Configure appropriate sampling rates for traces
- [ ] Optimize batch processing settings
- [ ] Set resource limits for OTEL collector
- [ ] Configure efficient storage retention policies

### 6.2 Monitoring and Alerting

- [ ] Create Grafana alerts for:
  - [ ] High error rates (>5%)
  - [ ] High latency (p99 >1s)
  - [ ] Low application availability
  - [ ] Infrastructure resource exhaustion

### 6.3 Security Configuration

- [ ] Configure authentication for Grafana
- [ ] Set up TLS for external endpoints
- [ ] Implement proper RBAC for dashboards
- [ ] Secure inter-service communication

### 6.4 Documentation

- [ ] Create operational runbook:
  - [ ] How to investigate performance issues
  - [ ] Common troubleshooting scenarios
  - [ ] Dashboard usage guide
  - [ ] Alert response procedures

---

## Phase 7: Advanced Features (Optional)

### 7.1 Enhanced Observability

- [ ] Add business-specific SLIs/SLOs
- [ ] Implement custom metrics for business KPIs
- [ ] Add user session tracking
- [ ] Implement feature flag instrumentation

### 7.2 Advanced Grafana Features

- [ ] Set up Grafana Alerting with notification channels
- [ ] Create custom visualization panels
- [ ] Implement dashboard templating and variables
- [ ] Add annotation support for deployments

### 7.3 Performance Profiling Integration

- [ ] Add Grafana Pyroscope for continuous profiling
- [ ] Instrument CPU and memory profiling
- [ ] Correlate profiles with traces and metrics

---

## Dependencies and Prerequisites

### Required Tools

- Go 1.23+
- Docker and Docker Compose
- Git (for version tracking)

### External Dependencies

- OpenTelemetry Go SDK v1.33+
- Gorilla Mux router
- Prometheus, Loki, Tempo, Grafana containers

### Environment Configuration

- **Development:** All components running locally with Docker Compose
- **Staging/Production:** Consider using managed services or Kubernetes deployment

---

## Useful Resources

### Documentation Links

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/)
- [Grafana LGTM Stack Guide](https://grafana.com/docs/opentelemetry/)
- [Tempo Configuration](https://grafana.com/docs/tempo/latest/configuration/)
- [Loki Configuration](https://grafana.com/docs/loki/latest/configure/)
- [Prometheus Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)

### Example Repositories

- [OpenTelemetry Go Examples](https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/examples)
- [Grafana LGTM Examples](https://github.com/grafana/intro-to-mlt)
- [Tempo Docker Compose Examples](https://github.com/grafana/tempo/tree/main/example/docker-compose)

---

## Success Criteria

✅ **Traces:** Distributed traces visible in Grafana with proper service correlation  
✅ **Metrics:** HTTP and business metrics collected and visualized in real-time  
✅ **Logs:** Structured logs with trace correlation searchable in Loki  
✅ **Dashboards:** Comprehensive observability dashboards for operations team  
✅ **Alerts:** Automated alerting for critical application and infrastructure issues  
✅ **Performance:** <5ms overhead for instrumentation, proper sampling configuration
