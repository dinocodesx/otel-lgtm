import { NodeSDK } from "@opentelemetry/sdk-node";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import semanticConventions from "@opentelemetry/semantic-conventions";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-http";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import {
  LoggerProvider,
  BatchLogRecordProcessor,
} from "@opentelemetry/sdk-logs";
import * as api from "@opentelemetry/api";
import dotenv from "dotenv";

// Load environment variables
dotenv.config();

const initTelemetry = () => {
  // Service information
  const serviceName = process.env.SERVICE_NAME || "otel-lgtm-express-app";
  const serviceVersion = process.env.SERVICE_VERSION || "1.0.0";
  const serviceNamespace = process.env.SERVICE_NAMESPACE || "development";

  // OpenTelemetry Collector endpoints
  const otelCollectorUrl =
    process.env.OTEL_EXPORTER_OTLP_ENDPOINT || "http://localhost:4318";
  const traceEndpoint = `${otelCollectorUrl}/v1/traces`;
  const metricsEndpoint = `${otelCollectorUrl}/v1/metrics`;
  const logsEndpoint = `${otelCollectorUrl}/v1/logs`;

  // Create resource with service information
  const resource = Resource.default().merge(
    new Resource({
      [semanticConventions.ATTR_SERVICE_NAME]: serviceName,
      [semanticConventions.ATTR_SERVICE_VERSION]: serviceVersion,
      [semanticConventions.ATTR_SERVICE_NAMESPACE]: serviceNamespace,
      [semanticConventions.ATTR_DEPLOYMENT_ENVIRONMENT]:
        process.env.NODE_ENV || "development",
    })
  );

  // Configure trace exporter
  const traceExporter = new OTLPTraceExporter({
    url: traceEndpoint,
    headers: {},
  });

  // Configure metrics exporters
  const otlpMetricExporter = new OTLPMetricExporter({
    url: metricsEndpoint,
    headers: {},
  });

  // Configure log exporter
  const logExporter = new OTLPLogExporter({
    url: logsEndpoint,
    headers: {},
  });

  // Create logger provider for OpenTelemetry logs
  const loggerProvider = new LoggerProvider({
    resource: resource,
  });

  loggerProvider.addLogRecordProcessor(
    new BatchLogRecordProcessor(logExporter)
  );

  // Register logger provider
  api.logs.setGlobalLoggerProvider(loggerProvider);

  // Initialize NodeSDK
  const sdk = new NodeSDK({
    resource: resource,
    traceExporter: traceExporter,
    metricReader: new PeriodicExportingMetricReader({
      exporter: otlpMetricExporter,
      exportIntervalMillis: 5000, // Export metrics every 5 seconds
    }),
    instrumentations: [
      getNodeAutoInstrumentations({
        // Disable file system instrumentation to reduce noise
        "@opentelemetry/instrumentation-fs": {
          enabled: false,
        },
        // Configure HTTP instrumentation
        "@opentelemetry/instrumentation-http": {
          enabled: true,
          ignoreIncomingRequestHook: (req) => {
            // Ignore health check and metrics endpoints
            return (
              req.url?.includes("/health") || req.url?.includes("/metrics")
            );
          },
        },
        // Configure Express instrumentation
        "@opentelemetry/instrumentation-express": {
          enabled: true,
        },
      }),
    ],
  });

  // Error handling for SDK initialization
  sdk
    .start()
    .then(() => {
      console.log("🔍 OpenTelemetry initialized successfully");
      console.log(`📊 Service: ${serviceName} (${serviceVersion})`);
      console.log(`🌍 Environment: ${process.env.NODE_ENV || "development"}`);
      console.log(`📡 OTLP Endpoint: ${otelCollectorUrl}`);
      console.log(`📈 Metrics will be available via OpenTelemetry Collector`);
    })
    .catch((error) => {
      console.error("❌ Error initializing OpenTelemetry:", error);
      process.exit(1);
    });

  // Graceful shutdown
  process.on("SIGTERM", () => {
    sdk
      .shutdown()
      .then(() => {
        console.log("🔍 OpenTelemetry terminated gracefully");
        process.exit(0);
      })
      .catch((error) => {
        console.error("❌ Error terminating OpenTelemetry:", error);
        process.exit(1);
      });
  });

  return { sdk, resource };
};

export default initTelemetry;
