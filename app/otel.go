package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	ServiceName      string
	ServiceVersion   string
	Environment      string
	TraceEndpoint    string
	MetricsEndpoint  string
	LogsEndpoint     string
	EnablePrometheus bool
	SampleRate       float64
	InsecureMode     bool
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

// DefaultConfig returns a default configuration for OpenTelemetry
func DefaultConfig() *Config {
	return &Config{
		ServiceName:      "otel-lgtm-api",
		ServiceVersion:   getVersion(),
		Environment:      getEnvironment(),
		TraceEndpoint:    "http://localhost:4318/v1/traces",
		MetricsEndpoint:  "http://localhost:4318/v1/metrics",
		LogsEndpoint:     "http://localhost:4318/v1/logs",
		EnablePrometheus: true,
		SampleRate:       1.0,  // 100% sampling for development
		InsecureMode:     true, // Allow insecure connections for local development
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

// createResource creates the OpenTelemetry resource with service attributes
func (tp *TelemetryProvider) createResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			// Service attributes
			semconv.ServiceName(tp.Config.ServiceName),
			semconv.ServiceVersion(tp.Config.ServiceVersion),
			semconv.DeploymentEnvironmentName(tp.Config.Environment),
			semconv.ServiceInstanceID(generateInstanceID()),

			// Additional custom attributes
			attribute.String("telemetry.sdk.name", "opentelemetry"),
			attribute.String("telemetry.sdk.language", "go"),
			attribute.String("telemetry.sdk.version", otel.Version()),
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
}

// setupTraceProvider initializes the trace provider with OTLP exporter
func (tp *TelemetryProvider) setupTraceProvider(ctx context.Context) error {
	// Create OTLP trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(tp.Config.TraceEndpoint),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithHeaders(map[string]string{
			"Content-Type": "application/x-protobuf",
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create batch span processor
	batchProcessor := sdktrace.NewBatchSpanProcessor(
		traceExporter,
		sdktrace.WithBatchTimeout(5*time.Second),
		sdktrace.WithMaxExportBatchSize(512),
		sdktrace.WithMaxQueueSize(2048),
	)

	// Create trace provider
	tp.TracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(tp.Resource),
		sdktrace.WithSpanProcessor(batchProcessor),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(tp.Config.SampleRate)),
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

	// Set up Prometheus exporter if enabled
	if tp.Config.EnablePrometheus {
		prometheusExporter, err := prometheus.New()
		if err != nil {
			return fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		readers = append(readers, prometheusExporter)
	}

	// Set up OTLP metrics exporter
	otlpMetricsExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(tp.Config.MetricsEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
	}

	// Create periodic reader for OTLP exporter
	otlpReader := sdkmetric.NewPeriodicReader(
		otlpMetricsExporter,
		sdkmetric.WithInterval(30*time.Second),
	)
	readers = append(readers, otlpReader)

	// Create metrics provider with multiple readers
	options := []sdkmetric.Option{
		sdkmetric.WithResource(tp.Resource),
	}

	// Add each reader individually
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
	// Create OTLP log exporter
	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(tp.Config.LogsEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create log exporter: %w", err)
	}

	// Create batch log processor
	batchProcessor := log.NewBatchProcessor(
		logExporter,
		log.WithExportInterval(5*time.Second),
		log.WithExportTimeout(30*time.Second),
		log.WithExportMaxBatchSize(512),
		log.WithMaxQueueSize(2048),
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

// getVersion returns the service version from environment or default
func getVersion() string {
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
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
	return "development"
}

// generateInstanceID creates a unique instance identifier
func generateInstanceID() string {
	if instanceID := os.Getenv("SERVICE_INSTANCE_ID"); instanceID != "" {
		return instanceID
	}

	// Use hostname as fallback
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return "unknown"
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
