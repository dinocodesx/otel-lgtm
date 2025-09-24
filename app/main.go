package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

// Response structures
type SuccessResponse struct {
	Status    int         `json:"status"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"requestId"`
	Timestamp string      `json:"timestamp"`
}

type ErrorResponse struct {
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
	Location  string `json:"location,omitempty"`
	RequestID string `json:"requestId"`
	Timestamp string `json:"timestamp"`
}

type HealthResponse struct {
	Status    string  `json:"status"`
	Timestamp string  `json:"timestamp"`
	Uptime    float64 `json:"uptime"`
}

type RootResponse struct {
	Message string `json:"message"`
}

type NotFoundResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// Scenario represents a response scenario
type Scenario struct {
	Status   int
	Error    string
	Message  string
	Details  string
	Location string
	Data     interface{}
}

var startTime = time.Now()

// Generate random request ID
func generateRequestID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 13)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Custom response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Log request
		logData := map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"method":    r.Method,
			"path":      r.URL.Path,
			"status":    wrapped.statusCode,
			"duration":  fmt.Sprintf("%.3fms", float64(duration.Nanoseconds())/1e6),
			"ip":        r.RemoteAddr,
			"userAgent": r.UserAgent(),
		}

		logJSON, _ := json.Marshal(logData)
		log.Printf("Request: %s", string(logJSON))
	})
}

// Custom response writer to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Root route handler
func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := RootResponse{
		Message: "App is running",
	}

	json.NewEncoder(w).Encode(response)
}

// Health check handler
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	uptime := time.Since(startTime).Seconds()

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Uptime:    uptime,
	}

	json.NewEncoder(w).Encode(response)
}

// API handler with random responses and delays
func apiHandler(w http.ResponseWriter, r *http.Request) {
	// Random delay between 100ms to 3000ms
	delay := rand.Intn(2900) + 100
	time.Sleep(time.Duration(delay) * time.Millisecond)

	// Define all response scenarios
	scenarios := []Scenario{
		// 2xx Success responses
		{200, "", "Success response", "", "", map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"delay":     fmt.Sprintf("%dms", delay),
		}},
		{200, "", "Data retrieved successfully", "", "", map[string]interface{}{
			"users": []string{"Alice", "Bob", "Charlie"},
			"delay": fmt.Sprintf("%dms", delay),
		}},
		{200, "", "Search results found", "", "", map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": 1, "name": "Product A", "price": 29.99},
				{"id": 2, "name": "Product B", "price": 49.99},
			},
			"total": 2,
			"delay": fmt.Sprintf("%dms", delay),
		}},
		{201, "", "Resource created successfully", "", "", map[string]interface{}{
			"id":    rand.Intn(1000),
			"delay": fmt.Sprintf("%dms", delay),
		}},
		{201, "", "User account created", "", "", map[string]interface{}{
			"userId":   rand.Intn(10000),
			"username": fmt.Sprintf("user_%d", rand.Intn(1000)),
			"delay":    fmt.Sprintf("%dms", delay),
		}},
		{202, "", "Request accepted for processing", "", "", map[string]interface{}{
			"jobId":  generateRequestID(),
			"status": "queued",
			"delay":  fmt.Sprintf("%dms", delay),
		}},
		{204, "", "No content - operation successful", "", "", nil},
		{206, "", "Partial content delivered", "", "", map[string]interface{}{
			"range":         "bytes 0-1023/2048",
			"contentLength": 1024,
			"delay":         fmt.Sprintf("%dms", delay),
		}},

		// 3xx Redirection responses
		{301, "", "Moved permanently", "", "/api/v2/endpoint", map[string]interface{}{
			"redirect": true,
			"delay":    fmt.Sprintf("%dms", delay),
		}},
		{302, "", "Found - temporary redirect", "", "/api/temp-endpoint", map[string]interface{}{
			"redirect": true,
			"delay":    fmt.Sprintf("%dms", delay),
		}},
		{304, "", "Not modified", "", "", map[string]interface{}{
			"cached": true,
			"delay":  fmt.Sprintf("%dms", delay),
		}},
		{307, "", "Temporary redirect", "", "/api/v1/fallback", map[string]interface{}{
			"redirect": true,
			"delay":    fmt.Sprintf("%dms", delay),
		}},
		{308, "", "Permanent redirect", "", "/api/v3/endpoint", map[string]interface{}{
			"redirect": true,
			"delay":    fmt.Sprintf("%dms", delay),
		}},

		// 4xx Client error responses
		{400, "Bad Request", "Invalid request parameters", "Missing required field 'email'", "", nil},
		{400, "Bad Request", "Invalid JSON format", "Malformed JSON in request body", "", nil},
		{401, "Unauthorized", "Authentication required", "Please provide a valid API key", "", nil},
		{401, "Unauthorized", "Token expired", "JWT token has expired, please refresh", "", nil},
		{402, "Payment Required", "Subscription expired", "Please upgrade your plan to continue", "", nil},
		{403, "Forbidden", "Access denied", "Insufficient permissions for this resource", "", nil},
		{403, "Forbidden", "IP address blocked", "Your IP has been temporarily blocked", "", nil},
		{404, "Not Found", "Resource not found", "The requested endpoint does not exist", "", nil},
		{404, "Not Found", "User not found", fmt.Sprintf("User with ID %d does not exist", rand.Intn(1000)), "", nil},
		{405, "Method Not Allowed", "HTTP method not supported", "Only GET and POST methods are allowed", "", nil},
		{406, "Not Acceptable", "Content type not acceptable", "Server cannot produce content matching Accept header", "", nil},
		{408, "Request Timeout", "Request took too long", "Client did not send request within timeout period", "", nil},
		{409, "Conflict", "Resource conflict", "Email address already exists", "", nil},
		{410, "Gone", "Resource no longer available", "This API version has been deprecated", "", nil},
		{411, "Length Required", "Content-Length header required", "Request must include Content-Length header", "", nil},
		{412, "Precondition Failed", "Precondition not met", "If-Match header condition failed", "", nil},
		{413, "Payload Too Large", "Request entity too large", "File size exceeds 10MB limit", "", nil},
		{414, "URI Too Long", "Request URI too long", "URL exceeds maximum length of 2048 characters", "", nil},
		{415, "Unsupported Media Type", "Media type not supported", "Content-Type 'text/plain' not supported", "", nil},
		{416, "Range Not Satisfiable", "Requested range not satisfiable", "Range header specifies invalid byte range", "", nil},
		{417, "Expectation Failed", "Expectation cannot be met", "Expect header requirements cannot be satisfied", "", nil},
		{418, "I'm a teapot", "Cannot brew coffee", "This teapot cannot brew coffee (RFC 2324)", "", nil},
		{421, "Misdirected Request", "Request misdirected", "Server cannot produce response for this request", "", nil},
		{422, "Unprocessable Entity", "Validation failed", "Email format is invalid", "", nil},
		{423, "Locked", "Resource is locked", "Resource is currently being modified by another process", "", nil},
		{424, "Failed Dependency", "Dependent request failed", "Previous operation in sequence failed", "", nil},
		{425, "Too Early", "Request sent too early", "Server unwilling to process replayed request", "", nil},
		{426, "Upgrade Required", "Protocol upgrade required", "Client must upgrade to secure protocol", "", nil},
		{428, "Precondition Required", "Precondition header required", "Request must include If-Match header", "", nil},
		{429, "Too Many Requests", "Rate limit exceeded", "Maximum 100 requests per minute exceeded", "", nil},
		{431, "Request Header Fields Too Large", "Headers too large", "Request headers exceed maximum size limit", "", nil},
		{451, "Unavailable For Legal Reasons", "Content blocked", "Content unavailable due to legal restrictions", "", nil},

		// 5xx Server error responses
		{500, "Internal Server Error", "Something went wrong on our end", "Unexpected server error occurred", "", nil},
		{500, "Internal Server Error", "Database connection failed", "Unable to connect to primary database", "", nil},
		{501, "Not Implemented", "Feature not implemented", "This functionality is not yet available", "", nil},
		{502, "Bad Gateway", "Upstream service unavailable", "Authentication service is not responding", "", nil},
		{502, "Bad Gateway", "Invalid response from upstream", "Received malformed response from backend service", "", nil},
		{503, "Service Unavailable", "Service temporarily unavailable", "Server is temporarily overloaded", "", nil},
		{503, "Service Unavailable", "Maintenance mode", "Service under scheduled maintenance", "", nil},
		{504, "Gateway Timeout", "Request timeout", "Upstream server did not respond within timeout", "", nil},
		{505, "HTTP Version Not Supported", "HTTP version not supported", "Server does not support HTTP/2.0 protocol", "", nil},
		{506, "Variant Also Negotiates", "Content negotiation error", "Server configuration error in content negotiation", "", nil},
		{507, "Insufficient Storage", "Server storage full", "Unable to store representation needed for request", "", nil},
		{508, "Loop Detected", "Infinite loop detected", "Server detected infinite loop while processing request", "", nil},
		{510, "Not Extended", "Further extensions required", "Policy for accessing resource has not been met", "", nil},
		{511, "Network Authentication Required", "Network authentication required", "Client needs to authenticate to gain network access", "", nil},
	}

	// Create weighted scenarios for realistic distribution
	var weightedScenarios []Scenario

	// Success responses (60% - 12 entries)
	successScenarios := make([]Scenario, 0)
	for _, s := range scenarios {
		if s.Status >= 200 && s.Status < 300 {
			successScenarios = append(successScenarios, s)
		}
	}
	for i := 0; i < 12; i++ {
		weightedScenarios = append(weightedScenarios, successScenarios[rand.Intn(len(successScenarios))])
	}

	// Redirection responses (5% - 1 entry)
	redirectScenarios := make([]Scenario, 0)
	for _, s := range scenarios {
		if s.Status >= 300 && s.Status < 400 {
			redirectScenarios = append(redirectScenarios, s)
		}
	}
	if len(redirectScenarios) > 0 {
		weightedScenarios = append(weightedScenarios, redirectScenarios[rand.Intn(len(redirectScenarios))])
	}

	// Client error responses (25% - 5 entries)
	clientErrorScenarios := make([]Scenario, 0)
	for _, s := range scenarios {
		if s.Status >= 400 && s.Status < 500 {
			clientErrorScenarios = append(clientErrorScenarios, s)
		}
	}
	for i := 0; i < 5; i++ {
		weightedScenarios = append(weightedScenarios, clientErrorScenarios[rand.Intn(len(clientErrorScenarios))])
	}

	// Server error responses (10% - 2 entries)
	serverErrorScenarios := make([]Scenario, 0)
	for _, s := range scenarios {
		if s.Status >= 500 && s.Status < 600 {
			serverErrorScenarios = append(serverErrorScenarios, s)
		}
	}
	for i := 0; i < 2; i++ {
		weightedScenarios = append(weightedScenarios, serverErrorScenarios[rand.Intn(len(serverErrorScenarios))])
	}

	// Select random scenario
	randomScenario := weightedScenarios[rand.Intn(len(weightedScenarios))]

	requestID := generateRequestID()
	timestamp := time.Now().Format(time.RFC3339)

	// Log the response
	var logLevel string
	if randomScenario.Status >= 500 {
		logLevel = "error"
	} else if randomScenario.Status >= 400 {
		logLevel = "warn"
	} else {
		logLevel = "info"
	}

	apiLogData := map[string]interface{}{
		"timestamp": timestamp,
		"level":     logLevel,
		"message":   fmt.Sprintf("API Response: %d - Delay: %dms", randomScenario.Status, delay),
		"api": map[string]interface{}{
			"endpoint":      "/api",
			"status_code":   randomScenario.Status,
			"delay_ms":      delay,
			"response_type": randomScenario.Message,
		},
		"request": map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"user_agent":  r.UserAgent(),
			"remote_addr": r.RemoteAddr,
		},
	}

	apiLogJSON, _ := json.Marshal(apiLogData)
	log.Printf("API: %s", string(apiLogJSON))

	w.Header().Set("Content-Type", "application/json")

	// Set location header for redirect responses
	if randomScenario.Location != "" {
		w.Header().Set("Location", randomScenario.Location)
	}

	w.WriteHeader(randomScenario.Status)

	// Build response based on scenario type
	if randomScenario.Status >= 400 {
		// Error response
		response := ErrorResponse{
			Status:    randomScenario.Status,
			Error:     randomScenario.Error,
			Message:   randomScenario.Message,
			Details:   randomScenario.Details,
			Location:  randomScenario.Location,
			RequestID: requestID,
			Timestamp: timestamp,
		}
		json.NewEncoder(w).Encode(response)
	} else {
		// Success or redirect response
		response := SuccessResponse{
			Status:    randomScenario.Status,
			Message:   randomScenario.Message,
			Data:      randomScenario.Data,
			RequestID: requestID,
			Timestamp: timestamp,
		}
		json.NewEncoder(w).Encode(response)
	}
}

// 404 handler
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)

	response := NotFoundResponse{
		Error:     "Not Found",
		Message:   fmt.Sprintf("Route %s not found", r.URL.Path),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func main() {
	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create router
	r := mux.NewRouter()

	// Add logging middleware
	r.Use(loggingMiddleware)

	// Define routes
	r.HandleFunc("/", rootHandler).Methods("GET")
	r.HandleFunc("/api", apiHandler).Methods("GET")
	r.HandleFunc("/health", healthHandler).Methods("GET")

	// 404 handler for undefined routes
	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)

	// Start server
	fmt.Printf("🚀 Go server is running on port %s\n", port)
	fmt.Printf("📍 Root endpoint: http://localhost:%s/\n", port)
	fmt.Printf("🎲 API endpoint: http://localhost:%s/api\n", port)
	fmt.Printf("❤️  Health check: http://localhost:%s/health\n", port)

	log.Fatal(http.ListenAndServe(":"+port, r))
}
