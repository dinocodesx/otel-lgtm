import express from "express";

const app = express();
const PORT = process.env.PORT || 3000;

app.use(express.json());

// Root route - showing "App is running"
app.get("/", (req, res) => {
  res.status(200).json({ message: "App is running" });
});

// API route with delayed responses and various status codes
app.get("/api", async (req, res) => {
  // Random delay between 100ms to 3000ms
  const delay = Math.floor(Math.random() * 2900) + 100;
  await new Promise((resolve) => setTimeout(resolve, delay));

  // Generate random response scenarios
  const scenarios = [
    // 2xx Success responses
    {
      status: 200,
      message: "Success response",
      data: { timestamp: new Date().toISOString(), delay: `${delay}ms` },
    },
    {
      status: 200,
      message: "Data retrieved successfully",
      data: { users: ["Alice", "Bob", "Charlie"], delay: `${delay}ms` },
    },
    {
      status: 200,
      message: "Search results found",
      data: {
        results: [
          { id: 1, name: "Product A", price: 29.99 },
          { id: 2, name: "Product B", price: 49.99 },
        ],
        total: 2,
        delay: `${delay}ms`,
      },
    },
    {
      status: 201,
      message: "Resource created successfully",
      data: { id: Math.floor(Math.random() * 1000), delay: `${delay}ms` },
    },
    {
      status: 201,
      message: "User account created",
      data: {
        userId: Math.floor(Math.random() * 10000),
        username: `user_${Math.floor(Math.random() * 1000)}`,
        delay: `${delay}ms`,
      },
    },
    {
      status: 202,
      message: "Request accepted for processing",
      data: {
        jobId: Math.random().toString(36).substring(2, 15),
        status: "queued",
        delay: `${delay}ms`,
      },
    },
    {
      status: 204,
      message: "No content - operation successful",
    },
    {
      status: 206,
      message: "Partial content delivered",
      data: {
        range: "bytes 0-1023/2048",
        contentLength: 1024,
        delay: `${delay}ms`,
      },
    },

    // 3xx Redirection responses
    {
      status: 301,
      message: "Moved permanently",
      location: "/api/v2/endpoint",
      data: { redirect: true, delay: `${delay}ms` },
    },
    {
      status: 302,
      message: "Found - temporary redirect",
      location: "/api/temp-endpoint",
      data: { redirect: true, delay: `${delay}ms` },
    },
    {
      status: 304,
      message: "Not modified",
      data: { cached: true, delay: `${delay}ms` },
    },
    {
      status: 307,
      message: "Temporary redirect",
      location: "/api/v1/fallback",
      data: { redirect: true, delay: `${delay}ms` },
    },
    {
      status: 308,
      message: "Permanent redirect",
      location: "/api/v3/endpoint",
      data: { redirect: true, delay: `${delay}ms` },
    },

    // 4xx Client error responses
    {
      status: 400,
      error: "Bad Request",
      message: "Invalid request parameters",
      details: "Missing required field 'email'",
    },
    {
      status: 400,
      error: "Bad Request",
      message: "Invalid JSON format",
      details: "Malformed JSON in request body",
    },
    {
      status: 401,
      error: "Unauthorized",
      message: "Authentication required",
      details: "Please provide a valid API key",
    },
    {
      status: 401,
      error: "Unauthorized",
      message: "Token expired",
      details: "JWT token has expired, please refresh",
    },
    {
      status: 402,
      error: "Payment Required",
      message: "Subscription expired",
      details: "Please upgrade your plan to continue",
    },
    {
      status: 403,
      error: "Forbidden",
      message: "Access denied",
      details: "Insufficient permissions for this resource",
    },
    {
      status: 403,
      error: "Forbidden",
      message: "IP address blocked",
      details: "Your IP has been temporarily blocked",
    },
    {
      status: 404,
      error: "Not Found",
      message: "Resource not found",
      details: "The requested endpoint does not exist",
    },
    {
      status: 404,
      error: "Not Found",
      message: "User not found",
      details: `User with ID ${Math.floor(
        Math.random() * 1000
      )} does not exist`,
    },
    {
      status: 405,
      error: "Method Not Allowed",
      message: "HTTP method not supported",
      details: "Only GET and POST methods are allowed",
    },
    {
      status: 406,
      error: "Not Acceptable",
      message: "Content type not acceptable",
      details: "Server cannot produce content matching Accept header",
    },
    {
      status: 408,
      error: "Request Timeout",
      message: "Request took too long",
      details: "Client did not send request within timeout period",
    },
    {
      status: 409,
      error: "Conflict",
      message: "Resource conflict",
      details: "Email address already exists",
    },
    {
      status: 410,
      error: "Gone",
      message: "Resource no longer available",
      details: "This API version has been deprecated",
    },
    {
      status: 411,
      error: "Length Required",
      message: "Content-Length header required",
      details: "Request must include Content-Length header",
    },
    {
      status: 412,
      error: "Precondition Failed",
      message: "Precondition not met",
      details: "If-Match header condition failed",
    },
    {
      status: 413,
      error: "Payload Too Large",
      message: "Request entity too large",
      details: "File size exceeds 10MB limit",
    },
    {
      status: 414,
      error: "URI Too Long",
      message: "Request URI too long",
      details: "URL exceeds maximum length of 2048 characters",
    },
    {
      status: 415,
      error: "Unsupported Media Type",
      message: "Media type not supported",
      details: "Content-Type 'text/plain' not supported",
    },
    {
      status: 416,
      error: "Range Not Satisfiable",
      message: "Requested range not satisfiable",
      details: "Range header specifies invalid byte range",
    },
    {
      status: 417,
      error: "Expectation Failed",
      message: "Expectation cannot be met",
      details: "Expect header requirements cannot be satisfied",
    },
    {
      status: 418,
      error: "I'm a teapot",
      message: "Cannot brew coffee",
      details: "This teapot cannot brew coffee (RFC 2324)",
    },
    {
      status: 421,
      error: "Misdirected Request",
      message: "Request misdirected",
      details: "Server cannot produce response for this request",
    },
    {
      status: 422,
      error: "Unprocessable Entity",
      message: "Validation failed",
      details: "Email format is invalid",
    },
    {
      status: 423,
      error: "Locked",
      message: "Resource is locked",
      details: "Resource is currently being modified by another process",
    },
    {
      status: 424,
      error: "Failed Dependency",
      message: "Dependent request failed",
      details: "Previous operation in sequence failed",
    },
    {
      status: 425,
      error: "Too Early",
      message: "Request sent too early",
      details: "Server unwilling to process replayed request",
    },
    {
      status: 426,
      error: "Upgrade Required",
      message: "Protocol upgrade required",
      details: "Client must upgrade to secure protocol",
    },
    {
      status: 428,
      error: "Precondition Required",
      message: "Precondition header required",
      details: "Request must include If-Match header",
    },
    {
      status: 429,
      error: "Too Many Requests",
      message: "Rate limit exceeded",
      details: "Maximum 100 requests per minute exceeded",
    },
    {
      status: 431,
      error: "Request Header Fields Too Large",
      message: "Headers too large",
      details: "Request headers exceed maximum size limit",
    },
    {
      status: 451,
      error: "Unavailable For Legal Reasons",
      message: "Content blocked",
      details: "Content unavailable due to legal restrictions",
    },

    // 5xx Server error responses
    {
      status: 500,
      error: "Internal Server Error",
      message: "Something went wrong on our end",
      details: "Unexpected server error occurred",
    },
    {
      status: 500,
      error: "Internal Server Error",
      message: "Database connection failed",
      details: "Unable to connect to primary database",
    },
    {
      status: 501,
      error: "Not Implemented",
      message: "Feature not implemented",
      details: "This functionality is not yet available",
    },
    {
      status: 502,
      error: "Bad Gateway",
      message: "Upstream service unavailable",
      details: "Authentication service is not responding",
    },
    {
      status: 502,
      error: "Bad Gateway",
      message: "Invalid response from upstream",
      details: "Received malformed response from backend service",
    },
    {
      status: 503,
      error: "Service Unavailable",
      message: "Service temporarily unavailable",
      details: "Server is temporarily overloaded",
    },
    {
      status: 503,
      error: "Service Unavailable",
      message: "Maintenance mode",
      details: "Service under scheduled maintenance",
    },
    {
      status: 504,
      error: "Gateway Timeout",
      message: "Request timeout",
      details: "Upstream server did not respond within timeout",
    },
    {
      status: 505,
      error: "HTTP Version Not Supported",
      message: "HTTP version not supported",
      details: "Server does not support HTTP/2.0 protocol",
    },
    {
      status: 506,
      error: "Variant Also Negotiates",
      message: "Content negotiation error",
      details: "Server configuration error in content negotiation",
    },
    {
      status: 507,
      error: "Insufficient Storage",
      message: "Server storage full",
      details: "Unable to store representation needed for request",
    },
    {
      status: 508,
      error: "Loop Detected",
      message: "Infinite loop detected",
      details: "Server detected infinite loop while processing request",
    },
    {
      status: 510,
      error: "Not Extended",
      message: "Further extensions required",
      details: "Policy for accessing resource has not been met",
    },
    {
      status: 511,
      error: "Network Authentication Required",
      message: "Network authentication required",
      details: "Client needs to authenticate to gain network access",
    },
  ];

  // Weight the scenarios to maintain realistic distribution
  // 2xx: Success responses (60% chance)
  const successScenarios = scenarios.filter(
    (s) => s.status >= 200 && s.status < 300
  );
  // 3xx: Redirection responses (5% chance)
  const redirectScenarios = scenarios.filter(
    (s) => s.status >= 300 && s.status < 400
  );
  // 4xx: Client error responses (25% chance)
  const clientErrorScenarios = scenarios.filter(
    (s) => s.status >= 400 && s.status < 500
  );
  // 5xx: Server error responses (10% chance)
  const serverErrorScenarios = scenarios.filter(
    (s) => s.status >= 500 && s.status < 600
  );

  const weightedScenarios = [
    // Success responses - 60% (12 entries)
    ...Array(12)
      .fill()
      .flatMap(
        () =>
          successScenarios[Math.floor(Math.random() * successScenarios.length)]
      ),
    // Redirection responses - 5% (1 entry)
    redirectScenarios[Math.floor(Math.random() * redirectScenarios.length)],
    // Client errors - 25% (5 entries)
    ...Array(5)
      .fill()
      .flatMap(
        () =>
          clientErrorScenarios[
            Math.floor(Math.random() * clientErrorScenarios.length)
          ]
      ),
    // Server errors - 10% (2 entries)
    ...Array(2)
      .fill()
      .flatMap(
        () =>
          serverErrorScenarios[
            Math.floor(Math.random() * serverErrorScenarios.length)
          ]
      ),
  ];

  // Select random scenario
  const randomScenario =
    weightedScenarios[Math.floor(Math.random() * weightedScenarios.length)];

  // Log the response for debugging
  console.log(
    `[${new Date().toISOString()}] API Response: ${
      randomScenario.status
    } - Delay: ${delay}ms`
  );

  // Send the response
  res.status(randomScenario.status).json({
    ...randomScenario,
    requestId: Math.random().toString(36).substring(2, 15),
    timestamp: new Date().toISOString(),
  });
});

// Health check endpoint
app.get("/health", (req, res) => {
  res.status(200).json({
    status: "healthy",
    timestamp: new Date().toISOString(),
    uptime: process.uptime(),
  });
});

// 404 handler for undefined routes
app.use((req, res) => {
  res.status(404).json({
    error: "Not Found",
    message: `Route ${req.originalUrl} not found`,
    timestamp: new Date().toISOString(),
  });
});

// Error handling middleware
app.use((err, req, res, next) => {
  console.error("Error:", err);
  res.status(500).json({
    error: "Internal Server Error",
    message: "Something went wrong!",
    timestamp: new Date().toISOString(),
  });
});

// Start the server
app.listen(PORT, () => {
  console.log(`🚀 Express server is running on port ${PORT}`);
  console.log(`📍 Root endpoint: http://localhost:${PORT}/`);
  console.log(`🎲 API endpoint: http://localhost:${PORT}/api`);
  console.log(`❤️  Health check: http://localhost:${PORT}/health`);
});

export default app;
