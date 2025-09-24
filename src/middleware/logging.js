import { createLogger, format, transports } from "winston";
import * as api from "@opentelemetry/api";

// Custom format for OpenTelemetry correlation
const otelFormat = format((info) => {
  const activeSpan = api.trace.getActiveSpan();
  if (activeSpan) {
    const spanContext = activeSpan.spanContext();
    info.traceId = spanContext.traceId;
    info.spanId = spanContext.spanId;
  }
  return info;
});

// Create a logger instance with OpenTelemetry correlation
const logger = createLogger({
  level: process.env.LOG_LEVEL || "info",
  format: format.combine(
    format.timestamp(),
    otelFormat(),
    format.errors({ stack: true }),
    format.json()
  ),
  transports: [
    new transports.Console({
      format: format.combine(format.colorize(), format.simple()),
    }),
  ],
});

// Enhanced middleware function with OpenTelemetry integration
const loggingMiddleware = (req, res, next) => {
  const start = Date.now();
  const requestId = Math.random().toString(36).substring(2, 15);

  // Add request ID to request object
  req.requestId = requestId;

  // Get current span context
  const span = api.trace.getActiveSpan();
  const traceId = span ? span.spanContext().traceId : "no-trace";
  const spanId = span ? span.spanContext().spanId : "no-span";

  // Log the incoming request with structured data
  logger.info({
    message: "Incoming HTTP request",
    http: {
      method: req.method,
      url: req.originalUrl,
      path: req.path,
      user_agent: req.get("User-Agent"),
      content_length: req.get("content-length"),
      remote_addr: req.ip,
      host: req.get("host"),
    },
    request: {
      id: requestId,
      headers: {
        authorization: req.get("authorization") ? "[REDACTED]" : undefined,
        "content-type": req.get("content-type"),
      },
    },
    trace: {
      trace_id: traceId,
      span_id: spanId,
    },
    timestamp: new Date().toISOString(),
  });

  // Listen for the response to log it after it's sent
  res.on("finish", () => {
    const duration = Date.now() - start;
    const logLevel =
      res.statusCode >= 500 ? "error" : res.statusCode >= 400 ? "warn" : "info";

    logger.log(logLevel, {
      message: "HTTP response sent",
      http: {
        method: req.method,
        url: req.originalUrl,
        path: req.path,
        status_code: res.statusCode,
        response_size: res.get("content-length"),
        duration_ms: duration,
      },
      request: {
        id: requestId,
      },
      trace: {
        trace_id: traceId,
        span_id: spanId,
      },
      timestamp: new Date().toISOString(),
    });
  });

  // Handle request errors
  res.on("error", (error) => {
    logger.error({
      message: "HTTP request error",
      error: {
        name: error.name,
        message: error.message,
        stack: error.stack,
      },
      http: {
        method: req.method,
        url: req.originalUrl,
        path: req.path,
      },
      request: {
        id: requestId,
      },
      trace: {
        trace_id: traceId,
        span_id: spanId,
      },
      timestamp: new Date().toISOString(),
    });
  });

  next();
};

export { logger, loggingMiddleware };
export default loggingMiddleware;
