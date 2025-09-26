package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/prometheus/otlptranslator"
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
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for OpenTelemetry setup
type Config struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceEndpoint   string
	MetricsEndpoint string
	LogsEndpoint    string
	SampleRate      float64
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

// DefaultConfig returns a default configuration for LGTM stack
func DefaultConfig() *Config {
	return &Config{
		ServiceName:     getServiceName(),
		ServiceVersion:  getServiceVersion(),
		Environment:     getEnvironment(),
		TraceEndpoint:   "http://otel-collector:4318", // OTLP HTTP endpoint
		MetricsEndpoint: "http://otel-collector:4318", // OTLP HTTP endpoint
		LogsEndpoint:    "http://otel-collector:4318", // OTLP HTTP endpoint
		SampleRate:      1.0,                          // 100% sampling for development
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

// createResource creates the OpenTelemetry resource with basic service identification
func (tp *TelemetryProvider) createResource(ctx context.Context) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", tp.Config.ServiceName),
		attribute.String("service.version", tp.Config.ServiceVersion),
		attribute.String("deployment.environment", tp.Config.Environment),
	}

	// Add hostname as service instance ID
	if hostname, err := os.Hostname(); err == nil {
		attrs = append(attrs, attribute.String("service.instance.id", hostname))
	}

	// Create resource with basic detection
	return resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithFromEnv(), // Support OTEL_RESOURCE_ATTRIBUTES env var
		resource.WithProcess(), // Add process information
		resource.WithHost(),    // Add host information
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
	prometheusExporter, err := prometheus.New(
		prometheus.WithTranslationStrategy(otlptranslator.UnderscoreEscapingWithoutSuffixes), // Modern replacement for WithoutUnits
		prometheus.WithoutScopeInfo(), // Simplify metric names
	)
	if err != nil {
		return fmt.Errorf("failed to create prometheus exporter: %w", err)
	}
	readers = append(readers, prometheusExporter)

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

// Simple helper functions for basic configuration
func getServiceName() string {
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	if name := os.Getenv("SERVICE_NAME"); name != "" {
		return name
	}
	return "otel-demo-app"
}

func getServiceVersion() string {
	if version := os.Getenv("OTEL_SERVICE_VERSION"); version != "" {
		return version
	}
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	return "1.0.0"
}

func getEnvironment() string {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	return "development"
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
