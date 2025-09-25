/*
Package main provides comprehensive OpenTelemetry setup for the LGTM stack with extensive resource attribute detection.

Resource Attributes Supported:
- Service: name, version, instance.id, namespace, team, owner
- Deployment: environment
- Build: time, commit, branch
- Runtime: Go compiler, version, description
- Container: id, name, image name/tag (Docker)
- Kubernetes: pod name/uid, namespace, node, deployment
- Cloud: provider, platform, region, AZ, account (AWS/GCP/Azure)
- Host: automatically detected via resource.WithHost()
- Process: automatically detected via resource.WithProcess()
- OS: automatically detected via resource.WithOS()

Environment Variables for Resource Detection:
- SERVICE_NAME, OTEL_SERVICE_NAME: service name
- SERVICE_VERSION, OTEL_SERVICE_VERSION: service version
- SERVICE_NAMESPACE, OTEL_SERVICE_NAMESPACE: service namespace
- SERVICE_INSTANCE_ID, OTEL_SERVICE_INSTANCE_ID: instance ID
- ENVIRONMENT, DEPLOYMENT_ENVIRONMENT, OTEL_DEPLOYMENT_ENVIRONMENT: environment
- SERVICE_TEAM, TEAM: team ownership
- SERVICE_OWNER, OWNER: service owner
- BUILD_TIME, SOURCE_DATE_EPOCH: build timestamp
- GIT_COMMIT, BUILD_COMMIT, VCS_REF: git commit hash
- GIT_BRANCH, BUILD_BRANCH: git branch
- Container vars: CONTAINER_ID, CONTAINER_NAME, CONTAINER_IMAGE, IMAGE_NAME, IMAGE_TAG
- K8s vars: K8S_POD_NAME, POD_NAME, K8S_NAMESPACE, POD_NAMESPACE, K8S_NODE_NAME, etc.
- Cloud vars: CLOUD_PROVIDER, AWS_REGION, GOOGLE_CLOUD_PROJECT, AZURE_SUBSCRIPTION_ID, etc.

OTEL_RESOURCE_ATTRIBUTES environment variable is also supported for additional attributes.
*/
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for OpenTelemetry setup
type Config struct {
	// Service Configuration
	ServiceName      string
	ServiceVersion   string
	ServiceNamespace string
	Environment      string

	// OTLP Endpoints
	TraceEndpoint   string
	MetricsEndpoint string
	LogsEndpoint    string

	// Export Configuration
	EnablePrometheus bool
	SampleRate       float64
	InsecureMode     bool

	// Resource Attributes Override
	ServiceInstanceID string
	ServiceTeam       string
	ServiceOwner      string

	// Build Information (can be set at compile time)
	BuildTime string
	GitCommit string
	GitBranch string
}

// TelemetryProvider holds all OpenTelemetry providers and resources
type TelemetryProvider struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *log.LoggerProvider
	Resource       *resource.Resource
	Config         *Config

	// Shutdown functions
	shutdownFuncs []func(context.Context) error
}

// DefaultConfig returns a default configuration for local development with LGTM stack
func DefaultConfig() *Config {
	return &Config{
		// Service Configuration
		ServiceName:      getServiceName(),
		ServiceVersion:   getVersion(),
		ServiceNamespace: getServiceNamespace(),
		Environment:      getEnvironment(),

		// OTLP Endpoints
		TraceEndpoint:   "http://localhost:4318/v1/traces",  // Tempo via OTEL Collector
		MetricsEndpoint: "http://localhost:4318/v1/metrics", // Prometheus via OTEL Collector
		LogsEndpoint:    "http://localhost:4318/v1/logs",    // Loki via OTEL Collector

		// Export Configuration
		EnablePrometheus: true, // Enable /metrics endpoint for Prometheus scraping
		SampleRate:       1.0,  // 100% sampling for development (adjust for production)
		InsecureMode:     true, // Allow insecure connections for local development

		// Resource Attributes (will use env detection if empty)
		ServiceInstanceID: getServiceInstanceID(),
		ServiceTeam:       getServiceTeam(),
		ServiceOwner:      getServiceOwner(),

		// Build Information (populated at build time or from environment)
		BuildTime: getBuildTime(),
		GitCommit: getGitCommit(),
		GitBranch: getGitBranch(),
	}
}

// NewTelemetryProvider creates and initializes all OpenTelemetry providers
func NewTelemetryProvider(ctx context.Context, config *Config) (*TelemetryProvider, error) {
	if config == nil {
		config = DefaultConfig()
	}

	provider := &TelemetryProvider{
		Config:        config,
		shutdownFuncs: make([]func(context.Context) error, 0),
	}

	// Create resource with service information
	res, err := provider.createResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	provider.Resource = res

	// Set up trace provider
	if err := provider.setupTraceProvider(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup trace provider: %w", err)
	}

	// Set up metrics provider
	if err := provider.setupMetricsProvider(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup metrics provider: %w", err)
	}

	// Set up logging provider
	if err := provider.setupLoggingProvider(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup logging provider: %w", err)
	}

	// Configure global propagators
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider, nil
}

// createResource creates the OpenTelemetry resource with comprehensive service attributes
func (tp *TelemetryProvider) createResource(ctx context.Context) (*resource.Resource, error) {
	// Use config values first, then fall back to environment detection
	instanceID := tp.Config.ServiceInstanceID
	if instanceID == "" {
		instanceID = generateInstanceID()
	}

	buildTime := tp.Config.BuildTime
	if buildTime == "" {
		buildTime = getBuildTime()
	}

	gitCommit := tp.Config.GitCommit
	if gitCommit == "" {
		gitCommit = getGitCommit()
	}

	gitBranch := tp.Config.GitBranch
	if gitBranch == "" {
		gitBranch = getGitBranch()
	}

	serviceTeam := tp.Config.ServiceTeam
	if serviceTeam == "" {
		serviceTeam = getServiceTeam()
	}

	serviceOwner := tp.Config.ServiceOwner
	if serviceOwner == "" {
		serviceOwner = getServiceOwner()
	}

	// Build comprehensive resource attributes following OpenTelemetry semantic conventions
	resourceAttributes := []attribute.KeyValue{
		// Core Service Attributes (Required/Recommended)
		semconv.ServiceName(tp.Config.ServiceName),
		semconv.ServiceVersion(tp.Config.ServiceVersion),
		semconv.ServiceInstanceID(instanceID),

		// Optional Service Namespace
		attribute.String("service.namespace", tp.Config.ServiceNamespace),

		// Deployment Environment (Recommended)
		semconv.DeploymentEnvironmentName(tp.Config.Environment),

		// Telemetry SDK Attributes (Required by spec)
		semconv.TelemetrySDKName("opentelemetry"),
		attribute.String("telemetry.sdk.language", "go"), // Use attribute.String since semconv doesn't have this
		semconv.TelemetrySDKVersion(otel.Version()),

		// Build and Version Information
		attribute.String("service.build.time", buildTime),
		attribute.String("service.build.commit", gitCommit),
		attribute.String("service.build.branch", gitBranch),

		// Runtime Information
		attribute.String("process.runtime.name", runtime.Compiler),
		attribute.String("process.runtime.version", runtime.Version()),
		attribute.String("process.runtime.description", getRuntimeDescription()),

		// Container Information (if available)
		attribute.String("container.id", getContainerID()),
		attribute.String("container.name", getContainerName()),
		attribute.String("container.image.name", getContainerImageName()),
		attribute.String("container.image.tag", getContainerImageTag()),

		// Kubernetes Information (if available)
		attribute.String("k8s.pod.name", getK8sPodName()),
		attribute.String("k8s.pod.uid", getK8sPodUID()),
		attribute.String("k8s.namespace.name", getK8sNamespace()),
		attribute.String("k8s.node.name", getK8sNodeName()),
		attribute.String("k8s.deployment.name", getK8sDeployment()),

		// Cloud Provider Information (if available)
		attribute.String("cloud.provider", getCloudProvider()),
		attribute.String("cloud.platform", getCloudPlatform()),
		attribute.String("cloud.region", getCloudRegion()),
		attribute.String("cloud.availability_zone", getCloudAZ()),
		attribute.String("cloud.account.id", getCloudAccountID()),

		// Service Organization Attributes
		attribute.String("service.team", serviceTeam),
		attribute.String("service.owner", serviceOwner),
	}

	// Filter out empty attributes
	validAttributes := make([]attribute.KeyValue, 0, len(resourceAttributes))
	for _, attr := range resourceAttributes {
		if attr.Value.AsString() != "" {
			validAttributes = append(validAttributes, attr)
		}
	}

	// Create resource with comprehensive detection
	return resource.New(ctx,
		resource.WithAttributes(validAttributes...),
		resource.WithFromEnv(),      // Support OTEL_RESOURCE_ATTRIBUTES env var
		resource.WithTelemetrySDK(), // Add OpenTelemetry SDK info
		resource.WithProcess(),      // Add process information
		resource.WithOS(),           // Add operating system information
		resource.WithContainer(),    // Add container information (if available)
		resource.WithHost(),         // Add host information
	)
}

// setupTraceProvider initializes the trace provider with OTLP exporter
func (tp *TelemetryProvider) setupTraceProvider(ctx context.Context) error {
	// Create OTLP trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(tp.Config.TraceEndpoint),
		otlptracehttp.WithInsecure(), // Only for development - use WithTLSClientConfig in production
		otlptracehttp.WithHeaders(map[string]string{
			"Content-Type": "application/x-protobuf",
		}),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression), // Enable compression for efficiency
	)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create batch span processor with optimized settings for LGTM stack
	batchProcessor := sdktrace.NewBatchSpanProcessor(
		traceExporter,
		// Batch timeout: how long to wait before sending incomplete batches
		sdktrace.WithBatchTimeout(5*time.Second),
		// Export timeout: maximum time for a single export operation
		sdktrace.WithExportTimeout(30*time.Second),
		// Maximum batch size: optimal for most OTLP receivers including Tempo
		sdktrace.WithMaxExportBatchSize(512),
		// Queue size: buffer for high-throughput scenarios
		sdktrace.WithMaxQueueSize(2048),
		// Enable blocking mode for critical applications (optional)
		// sdktrace.WithBlocking(), // Uncomment if you need guaranteed delivery over performance
	)

	// Create trace provider with optimized sampling for development
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(tp.Config.SampleRate),
		// Configure parent-based sampling for distributed tracing
		sdktrace.WithRemoteParentSampled(sdktrace.AlwaysSample()),
		sdktrace.WithRemoteParentNotSampled(sdktrace.NeverSample()),
		sdktrace.WithLocalParentSampled(sdktrace.AlwaysSample()),
		sdktrace.WithLocalParentNotSampled(sdktrace.TraceIDRatioBased(tp.Config.SampleRate)),
	)

	tp.TracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(tp.Resource),
		sdktrace.WithSpanProcessor(batchProcessor),
		sdktrace.WithSampler(sampler),
		// Configure span limits to prevent memory issues
		sdktrace.WithRawSpanLimits(sdktrace.SpanLimits{
			AttributeValueLengthLimit:   4096, // Limit attribute value length
			AttributeCountLimit:         128,  // Maximum attributes per span
			EventCountLimit:             128,  // Maximum events per span
			LinkCountLimit:              128,  // Maximum links per span
			AttributePerEventCountLimit: 128,  // Maximum attributes per event
			AttributePerLinkCountLimit:  128,  // Maximum attributes per link
		}),
	)

	// Set as global trace provider
	otel.SetTracerProvider(tp.TracerProvider)

	// Add shutdown function
	tp.shutdownFuncs = append(tp.shutdownFuncs, tp.TracerProvider.Shutdown)

	return nil
}

// setupMetricsProvider initializes the metrics provider with exporters
func (tp *TelemetryProvider) setupMetricsProvider(ctx context.Context) error {
	var readers []sdkmetric.Reader

	// Set up Prometheus exporter for scraping endpoint (/metrics)
	if tp.Config.EnablePrometheus {
		prometheusExporter, err := prometheus.New(
			prometheus.WithoutUnits(),     // Remove units from metric names for Prometheus compatibility
			prometheus.WithoutScopeInfo(), // Simplify metric names
		)
		if err != nil {
			return fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		readers = append(readers, prometheusExporter)
	}

	// Set up OTLP metrics exporter for sending to collector/backend
	otlpMetricsExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(tp.Config.MetricsEndpoint),
		otlpmetrichttp.WithInsecure(), // Only for development
		otlpmetrichttp.WithHeaders(map[string]string{
			"Content-Type": "application/x-protobuf",
		}),
		otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression), // Enable compression
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
	}

	// Create periodic reader for OTLP exporter with optimized intervals
	otlpReader := sdkmetric.NewPeriodicReader(
		otlpMetricsExporter,
		// Collection interval: balance between freshness and resource usage
		sdkmetric.WithInterval(30*time.Second), // Good for development, consider 60s+ for production
		// Timeout for each export
		sdkmetric.WithTimeout(30*time.Second),
	)
	readers = append(readers, otlpReader)

	// Create metrics provider with multiple readers and resource attributes
	options := []sdkmetric.Option{
		sdkmetric.WithResource(tp.Resource),
		// Configure view aggregations for better performance (optional)
		sdkmetric.WithView(
			// Example: Configure histogram buckets for HTTP request duration
			sdkmetric.NewView(
				sdkmetric.Instrument{
					Name: "http.server.request.duration",
					Kind: sdkmetric.InstrumentKindHistogram,
				},
				sdkmetric.Stream{
					Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
						Boundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, // HTTP latency buckets
					},
				},
			),
		),
	}

	// Add each reader individually (required pattern for multiple readers)
	for _, reader := range readers {
		options = append(options, sdkmetric.WithReader(reader))
	}

	tp.MeterProvider = sdkmetric.NewMeterProvider(options...)

	// Set as global metrics provider
	otel.SetMeterProvider(tp.MeterProvider)

	// Add shutdown function
	tp.shutdownFuncs = append(tp.shutdownFuncs, tp.MeterProvider.Shutdown)

	return nil
}

// setupLoggingProvider initializes the logging provider with OTLP exporter
func (tp *TelemetryProvider) setupLoggingProvider(ctx context.Context) error {
	// Create OTLP log exporter for sending to collector/Loki
	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(tp.Config.LogsEndpoint),
		otlploghttp.WithInsecure(), // Only for development
		otlploghttp.WithHeaders(map[string]string{
			"Content-Type": "application/x-protobuf",
		}),
		otlploghttp.WithCompression(otlploghttp.GzipCompression), // Enable compression
	)
	if err != nil {
		return fmt.Errorf("failed to create log exporter: %w", err)
	}

	// Create batch log processor with optimized settings for log aggregation
	batchProcessor := log.NewBatchProcessor(
		logExporter,
		// Export interval: how often to send log batches
		log.WithExportInterval(5*time.Second), // Frequent exports for real-time logs
		// Export timeout: maximum time for a single export operation
		log.WithExportTimeout(30*time.Second),
		// Max batch size: balance between throughput and memory usage
		log.WithExportMaxBatchSize(512), // Good for development, adjust based on log volume
		// Queue size: buffer for high log volume scenarios
		log.WithMaxQueueSize(2048), // Sufficient for moderate to high log volumes
	)

	// Create log provider
	tp.LoggerProvider = log.NewLoggerProvider(
		log.WithResource(tp.Resource),
		log.WithProcessor(batchProcessor),
	)

	// Set as global log provider
	global.SetLoggerProvider(tp.LoggerProvider)

	// Add shutdown function
	tp.shutdownFuncs = append(tp.shutdownFuncs, tp.LoggerProvider.Shutdown)

	return nil
}

// GetTracer returns a tracer instance
func (tp *TelemetryProvider) GetTracer(name string) trace.Tracer {
	return tp.TracerProvider.Tracer(name)
}

// GetMeter returns a meter instance
func (tp *TelemetryProvider) GetMeter(name string) metric.Meter {
	return tp.MeterProvider.Meter(name)
}

// GetLogger returns a structured logger with OpenTelemetry integration
func (tp *TelemetryProvider) GetLogger(name string) *slog.Logger {
	return otelslog.NewLogger(name)
}

// Shutdown gracefully shuts down all OpenTelemetry providers
func (tp *TelemetryProvider) Shutdown(ctx context.Context) error {
	var errors []error

	// Execute all shutdown functions
	for i := len(tp.shutdownFuncs) - 1; i >= 0; i-- {
		if err := tp.shutdownFuncs[i](ctx); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to shutdown telemetry providers: %v", errors)
	}

	return nil
}

// getServiceName returns the service name from environment or default
func getServiceName() string {
	if name := os.Getenv("SERVICE_NAME"); name != "" {
		return name
	}
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	return "otel-lgtm-api" // Default service name
}

// getServiceNamespace returns the service namespace from environment
func getServiceNamespace() string {
	if namespace := os.Getenv("SERVICE_NAMESPACE"); namespace != "" {
		return namespace
	}
	if namespace := os.Getenv("OTEL_SERVICE_NAMESPACE"); namespace != "" {
		return namespace
	}
	return "" // Optional attribute
}

// getServiceInstanceID returns the service instance ID from environment
func getServiceInstanceID() string {
	if instanceID := os.Getenv("SERVICE_INSTANCE_ID"); instanceID != "" {
		return instanceID
	}
	if instanceID := os.Getenv("OTEL_SERVICE_INSTANCE_ID"); instanceID != "" {
		return instanceID
	}
	return "" // Will be generated by generateInstanceID if empty
}

// getVersion returns the service version from environment or default
func getVersion() string {
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	if version := os.Getenv("OTEL_SERVICE_VERSION"); version != "" {
		return version
	}
	// You could also read from build info or git tags
	return "dev"
}

// getEnvironment returns the deployment environment
func getEnvironment() string {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("DEPLOYMENT_ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("OTEL_DEPLOYMENT_ENVIRONMENT"); env != "" {
		return env
	}
	return "development"
}

// generateInstanceID creates a unique instance identifier following OpenTelemetry best practices
func generateInstanceID() string {
	// Check for explicitly set service instance ID
	if instanceID := os.Getenv("SERVICE_INSTANCE_ID"); instanceID != "" {
		return instanceID
	}

	// Check for container/pod specific IDs
	if containerID := getContainerID(); containerID != "" {
		return containerID[:12] // Use first 12 chars like Docker
	}

	// Check for Kubernetes pod UID
	if podUID := getK8sPodUID(); podUID != "" {
		return podUID
	}

	// Use hostname as fallback
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	// Generate random UUID as last resort
	return generateUUID()
}

// generateUUID creates a simple UUID v4 for instance identification
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Build Information Functions
func getBuildTime() string {
	if buildTime := os.Getenv("BUILD_TIME"); buildTime != "" {
		return buildTime
	}
	if buildTime := os.Getenv("SOURCE_DATE_EPOCH"); buildTime != "" {
		return buildTime
	}
	return ""
}

func getGitCommit() string {
	if commit := os.Getenv("GIT_COMMIT"); commit != "" {
		return commit
	}
	if commit := os.Getenv("BUILD_COMMIT"); commit != "" {
		return commit
	}
	if commit := os.Getenv("VCS_REF"); commit != "" {
		return commit
	}
	return ""
}

func getGitBranch() string {
	if branch := os.Getenv("GIT_BRANCH"); branch != "" {
		return branch
	}
	if branch := os.Getenv("BUILD_BRANCH"); branch != "" {
		return branch
	}
	return ""
}

// Runtime Information Functions
func getRuntimeDescription() string {
	return fmt.Sprintf("%s %s %s/%s", runtime.Compiler, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Container Information Functions
func getContainerID() string {
	if id := os.Getenv("CONTAINER_ID"); id != "" {
		return id
	}
	// Check for Docker container ID from cgroup
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "/docker/") {
				parts := strings.Split(line, "/docker/")
				if len(parts) > 1 {
					return parts[1][:12] // Return first 12 chars
				}
			}
		}
	}
	return ""
}

func getContainerName() string {
	if name := os.Getenv("CONTAINER_NAME"); name != "" {
		return name
	}
	if name := os.Getenv("HOSTNAME"); name != "" {
		return name // Often the container name in Docker
	}
	return ""
}

func getContainerImageName() string {
	if image := os.Getenv("CONTAINER_IMAGE"); image != "" {
		return strings.Split(image, ":")[0] // Remove tag
	}
	if image := os.Getenv("IMAGE_NAME"); image != "" {
		return strings.Split(image, ":")[0]
	}
	return ""
}

func getContainerImageTag() string {
	if image := os.Getenv("CONTAINER_IMAGE"); image != "" {
		parts := strings.Split(image, ":")
		if len(parts) > 1 {
			return parts[1]
		}
	}
	if tag := os.Getenv("IMAGE_TAG"); tag != "" {
		return tag
	}
	return ""
}

// Kubernetes Information Functions
func getK8sPodName() string {
	if podName := os.Getenv("K8S_POD_NAME"); podName != "" {
		return podName
	}
	if podName := os.Getenv("POD_NAME"); podName != "" {
		return podName
	}
	if podName := os.Getenv("HOSTNAME"); podName != "" && strings.Contains(podName, "-") {
		return podName // Pod names often contain hyphens
	}
	return ""
}

func getK8sPodUID() string {
	if podUID := os.Getenv("K8S_POD_UID"); podUID != "" {
		return podUID
	}
	if podUID := os.Getenv("POD_UID"); podUID != "" {
		return podUID
	}
	return ""
}

func getK8sNamespace() string {
	if namespace := os.Getenv("K8S_NAMESPACE"); namespace != "" {
		return namespace
	}
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace
	}
	// Try to read from service account namespace
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func getK8sNodeName() string {
	if nodeName := os.Getenv("K8S_NODE_NAME"); nodeName != "" {
		return nodeName
	}
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		return nodeName
	}
	return ""
}

func getK8sDeployment() string {
	if deployment := os.Getenv("K8S_DEPLOYMENT_NAME"); deployment != "" {
		return deployment
	}
	if deployment := os.Getenv("DEPLOYMENT_NAME"); deployment != "" {
		return deployment
	}
	return ""
}

// Cloud Provider Information Functions
func getCloudProvider() string {
	if provider := os.Getenv("CLOUD_PROVIDER"); provider != "" {
		return provider
	}

	// Auto-detect cloud provider from metadata endpoints or env vars
	if os.Getenv("AWS_REGION") != "" || os.Getenv("AWS_DEFAULT_REGION") != "" {
		return "aws"
	}
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" || os.Getenv("GCP_PROJECT") != "" {
		return "gcp"
	}
	if os.Getenv("AZURE_SUBSCRIPTION_ID") != "" {
		return "azure"
	}
	return ""
}

func getCloudPlatform() string {
	if platform := os.Getenv("CLOUD_PLATFORM"); platform != "" {
		return platform
	}

	// Auto-detect platform
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		return "aws_lambda"
	}
	if os.Getenv("K_SERVICE") != "" { // Google Cloud Run
		return "gcp_cloud_run"
	}
	if os.Getenv("WEBSITE_SITE_NAME") != "" { // Azure App Service
		return "azure_app_service"
	}
	return ""
}

func getCloudRegion() string {
	if region := os.Getenv("CLOUD_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("GOOGLE_CLOUD_REGION"); region != "" {
		return region
	}
	return ""
}

func getCloudAZ() string {
	if az := os.Getenv("CLOUD_AVAILABILITY_ZONE"); az != "" {
		return az
	}
	if az := os.Getenv("AWS_AVAILABILITY_ZONE"); az != "" {
		return az
	}
	return ""
}

func getCloudAccountID() string {
	if account := os.Getenv("CLOUD_ACCOUNT_ID"); account != "" {
		return account
	}
	if account := os.Getenv("AWS_ACCOUNT_ID"); account != "" {
		return account
	}
	if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
		return project
	}
	if subscription := os.Getenv("AZURE_SUBSCRIPTION_ID"); subscription != "" {
		return subscription
	}
	return ""
}

// Service Organization Functions
func getServiceTeam() string {
	if team := os.Getenv("SERVICE_TEAM"); team != "" {
		return team
	}
	if team := os.Getenv("TEAM"); team != "" {
		return team
	}
	return ""
}

func getServiceOwner() string {
	if owner := os.Getenv("SERVICE_OWNER"); owner != "" {
		return owner
	}
	if owner := os.Getenv("OWNER"); owner != "" {
		return owner
	}
	return ""
}

// SetupWithDefaults is a convenience function to set up OpenTelemetry with default configuration
func SetupWithDefaults(ctx context.Context) (*TelemetryProvider, error) {
	return NewTelemetryProvider(ctx, DefaultConfig())
}

// SetupWithConfig is a convenience function to set up OpenTelemetry with custom configuration
func SetupWithConfig(ctx context.Context, serviceName, version, environment string) (*TelemetryProvider, error) {
	config := DefaultConfig()
	config.ServiceName = serviceName
	config.ServiceVersion = version
	config.Environment = environment

	return NewTelemetryProvider(ctx, config)
}
