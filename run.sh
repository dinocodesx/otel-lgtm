#!/bin/bash

# OTEL LGTM Observability Stack Runner
# This script starts the complete observability stack with Express app, OpenTelemetry Collector, Grafana, Loki, Tempo, and Prometheus

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${PURPLE}========================================${NC}"
    echo -e "${PURPLE}$1${NC}"
    echo -e "${PURPLE}========================================${NC}"
}

# Function to check if Docker is running
check_docker() {
    print_status "Checking Docker daemon..."
    if ! timeout 10 docker info > /dev/null 2>&1; then
        print_error "Docker is not running or not responding. Please:"
        print_error "1. Make sure Docker Desktop is running"
        print_error "2. Check Docker daemon status: systemctl status docker"
        print_error "3. Restart Docker if needed: sudo systemctl restart docker"
        exit 1
    fi
    print_success "Docker is running"
}

# Function to check if Docker Compose is available
check_docker_compose() {
    print_status "Checking Docker Compose..."
    if ! timeout 5 docker compose version > /dev/null 2>&1 && ! timeout 5 docker-compose --version > /dev/null 2>&1; then
        print_error "Docker Compose is not available. Please install Docker Compose."
        exit 1
    fi
    print_success "Docker Compose is available"
}

# Function to clean up previous containers
cleanup() {
    print_status "Cleaning up previous containers and volumes..."
    
    # Stop and remove containers with timeout
    if timeout 30 docker compose ps -q > /dev/null 2>&1; then
        timeout 60 docker compose down -v --remove-orphans || print_warning "Cleanup took longer than expected"
    elif timeout 30 docker-compose ps -q > /dev/null 2>&1; then
        timeout 60 docker-compose down -v --remove-orphans || print_warning "Cleanup took longer than expected"
    fi
    
    # Prune unused networks
    timeout 10 docker network prune -f > /dev/null 2>&1 || true
    
    print_success "Cleanup completed"
}

# Function to wait for service health
wait_for_service() {
    local service_name=$1
    local url=$2
    local max_attempts=30
    local attempt=1
    
    print_status "Waiting for $service_name to be healthy..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "$url" > /dev/null 2>&1; then
            print_success "$service_name is healthy!"
            return 0
        fi
        
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    print_warning "$service_name health check timed out, but continuing..."
    return 1
}

# Function to display service URLs
show_urls() {
    print_header "🌐 SERVICE URLS"
    echo -e "${CYAN}Express Application:${NC}     http://localhost:8080"
    echo -e "${CYAN}Grafana Dashboard:${NC}       http://localhost:3000 (admin/admin)"
    echo -e "${CYAN}Prometheus:${NC}              http://localhost:9090"
    echo -e "${CYAN}OpenTelemetry Collector:${NC} http://localhost:8888/metrics"
    echo -e "${CYAN}Loki:${NC}                    http://localhost:3100"
    echo -e "${CYAN}Tempo:${NC}                   http://localhost:3200"
    echo -e "${CYAN}App Metrics (Prometheus):${NC} http://localhost:8889/metrics"
    echo ""
    echo -e "${YELLOW}📊 In Grafana, you can:${NC}"
    echo -e "  • View application metrics from Prometheus"
    echo -e "  • Explore traces from Tempo"
    echo -e "  • Search logs from Loki"
    echo -e "  • See correlated data across all three pillars"
}

# Function to show sample requests
show_sample_requests() {
    print_header "🧪 SAMPLE REQUESTS"
    echo -e "${CYAN}Test the application with these commands:${NC}"
    echo ""
    echo -e "${GREEN}# Health check${NC}"
    echo "curl http://localhost:8080/health"
    echo ""
    echo -e "${GREEN}# Root endpoint${NC}"
    echo "curl http://localhost:8080/"
    echo ""
    echo -e "${GREEN}# API endpoint (random responses)${NC}"
    echo "curl http://localhost:8080/api"
    echo ""
    echo -e "${GREEN}# Generate load for testing${NC}"
    echo "for i in {1..10}; do curl http://localhost:8080/api & done"
    echo ""
    echo -e "${GREEN}# View metrics${NC}"
    echo "curl http://localhost:9090/metrics"
}

# Function to monitor logs
monitor_logs() {
    print_header "📝 CONTAINER LOGS"
    echo -e "${YELLOW}Press Ctrl+C to stop log monitoring${NC}"
    echo ""
    
    # Use docker-compose or docker compose based on availability
    if command -v docker-compose > /dev/null 2>&1; then
        docker-compose logs -f
    else
        docker compose logs -f
    fi
}

# Main execution
main() {
    print_header "🚀 OTEL LGTM OBSERVABILITY STACK"
    echo -e "${CYAN}Starting complete observability stack with:${NC}"
    echo -e "  📱 Express Application with OpenTelemetry"
    echo -e "  📊 Prometheus (Metrics)"  
    echo -e "  🔍 Tempo (Traces)"
    echo -e "  📝 Loki (Logs)"
    echo -e "  📈 Grafana (Visualization)"
    echo -e "  🔄 OpenTelemetry Collector"
    echo ""
    
    # Preliminary checks
    print_status "Performing preliminary checks..."
    check_docker
    check_docker_compose
    
    # Create .env file if it doesn't exist
    if [ ! -f .env ]; then
        print_warning ".env file not found, creating default one..."
        cat > .env << EOF
NODE_ENV=development
PORT=8080
SERVICE_NAME=otel-lgtm-express-app
SERVICE_VERSION=1.0.0
SERVICE_NAMESPACE=development
LOG_LEVEL=info

OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_SERVICE_NAME=otel-lgtm-express-app
OTEL_SERVICE_VERSION=1.0.0
OTEL_RESOURCE_ATTRIBUTES=service.name=otel-lgtm-express-app,service.version=1.0.0,deployment.environment=development

PROMETHEUS_PORT=9090
METRICS_EXPORT_INTERVAL_SECONDS=5

OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp,prometheus
OTEL_LOGS_EXPORTER=otlp

OTEL_NODE_ENABLED_INSTRUMENTATIONS=http,express,fs
OTEL_NODE_DISABLED_INSTRUMENTATIONS=
EOF
        print_success ".env file created"
    fi
    
    # Clean up previous runs
    cleanup
    
    # Create necessary directories with proper permissions
    print_status "Creating necessary directories..."
    mkdir -p ./data/loki/chunks ./data/loki/rules ./data/loki/tsdb-index ./data/loki/tsdb-cache ./data/loki/boltdb-shipper-compactor
    mkdir -p ./data/tempo ./data/prometheus ./data/grafana
    chmod -R 777 ./data/ 2>/dev/null || true
    
    print_status "Setting up Loki directories..."
    # Ensure Loki has all required directories (without sudo to avoid hanging)
    mkdir -p ./data/loki/chunks ./data/loki/rules ./data/loki/tsdb-index ./data/loki/tsdb-cache ./data/loki/boltdb-shipper-compactor 2>/dev/null || true
    chmod -R 777 ./data/loki/ 2>/dev/null || true
    
    # Build and start services
    print_status "Building and starting all services..."
    if command -v docker-compose > /dev/null 2>&1; then
        docker-compose up -d --build
    else
        docker compose up -d --build
    fi
    
    print_success "All services started!"
    
    # Wait for services to be healthy
    echo ""
    print_status "Checking service health..."
    
    # Wait for core services
    wait_for_service "OpenTelemetry Collector" "http://localhost:13133"
    wait_for_service "Prometheus" "http://localhost:9090"
    wait_for_service "Loki" "http://localhost:3100/ready"
    wait_for_service "Tempo" "http://localhost:3200/ready"
    wait_for_service "Grafana" "http://localhost:3000/api/health"
    wait_for_service "Express Application" "http://localhost:8080/health"
    
    # Check for any failed containers
    print_status "Checking for any failed containers..."
    if command -v docker-compose > /dev/null 2>&1; then
        failed_containers=$(docker-compose ps --services --filter "status=exited")
    else
        failed_containers=$(docker compose ps --services --filter "status=exited")
    fi
    
    if [ ! -z "$failed_containers" ]; then
        print_warning "Some containers have failed:"
        echo "$failed_containers"
        print_status "To troubleshoot, check logs with: docker-compose logs <service-name>"
    fi
    
    # Show service information
    echo ""
    show_urls
    echo ""
    show_sample_requests
    
    echo ""
    print_success "🎉 OTEL LGTM Stack is ready!"
    print_status "The observability stack is now running and collecting:"
    echo -e "  ${GREEN}✓${NC} Metrics (via Prometheus)"
    echo -e "  ${GREEN}✓${NC} Traces (via Tempo)"  
    echo -e "  ${GREEN}✓${NC} Logs (via Loki)"
    echo -e "  ${GREEN}✓${NC} Visualization (via Grafana)"
    
    echo ""
    echo -e "${YELLOW}Choose an option:${NC}"
    echo -e "  ${CYAN}1)${NC} Monitor logs (Ctrl+C to stop)"
    echo -e "  ${CYAN}2)${NC} Keep running in background"
    echo -e "  ${CYAN}3)${NC} Stop all services"
    echo ""
    
    read -p "Enter your choice (1-3): " choice
    
    case $choice in
        1)
            monitor_logs
            ;;
        2)
            print_success "Services are running in the background"
            print_status "To stop services later, run: docker-compose down"
            ;;
        3)
            print_status "Stopping all services..."
            if command -v docker-compose > /dev/null 2>&1; then
                docker-compose down
            else
                docker compose down
            fi
            print_success "All services stopped"
            ;;
        *)
            print_success "Services are running in the background"
            ;;
    esac
}

# Handle script interruption
trap 'echo ""; print_warning "Script interrupted. Services may still be running."; print_status "To stop services: docker-compose down"; exit 1' INT TERM

# Run main function
main "$@"