#!/bin/bash

# Ollama Production Management Script
# This script provides management commands for the production Ollama deployment

# Exit on any error, but allow for graceful error handling
set -e

# Configuration
OLLAMA_PORT="80"
OLLAMA_URL="http://localhost:$OLLAMA_PORT"
OLLAMA_CONTAINER_NAME="ollama-production"
OLLAMA_SERVICE_NAME="ollama"
OLLAMA_LOGS_DIR="/var/log/ollama"
OLLAMA_DATA_DIR="/opt/ollama"

# Error handling function
handle_error() {
    local exit_code=$?
    local line_number=$1
    local command=$2
    
    error "Script failed at line $line_number: $command (exit code: $exit_code)"
    
    # Provide helpful error messages based on common failure points
    case $exit_code in
        1)
            error "General error occurred"
            ;;
        2)
            error "Misuse of shell builtins"
            ;;
        126)
            error "Command invoked cannot execute (permission denied)"
            ;;
        127)
            error "Command not found"
            ;;
        128)
            error "Invalid argument to exit"
            ;;
        130)
            error "Script terminated by Ctrl+C"
            ;;
        *)
            error "Unknown error occurred"
            ;;
    esac
    
    exit $exit_code
}

# Set up error trap
trap 'handle_error ${LINENO} "$BASH_COMMAND"' ERR

# Check if custom URL is provided
if [ -n "$1" ] && [[ "$1" == http* ]]; then
    OLLAMA_URL="$1"
    echo "Using custom Ollama URL: $OLLAMA_URL"
    shift  # Remove the URL from arguments so other commands work
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
}

info() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] INFO: $1${NC}"
}

# Check if required commands are available
check_dependencies() {
    local missing_deps=()
    
    # Check for required commands
    for cmd in docker curl jq systemctl; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_deps+=("$cmd")
        fi
    done
    
    if [ ${#missing_deps[@]} -gt 0 ]; then
        error "Missing required dependencies: ${missing_deps[*]}"
        error "Please install the missing packages and try again."
        exit 1
    fi
}

# Check if Ollama is running
check_ollama_status() {
    # If using remote URL, we can't check local container status
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        return 0  # Assume remote is running if we can't check
    fi
    
    if docker ps --format "table {{.Names}}" | grep -q "$OLLAMA_CONTAINER_NAME" 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Check if Ollama is responding
check_ollama_health() {
    if curl -s --max-time 5 "$OLLAMA_URL" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Safe command execution with error handling
safe_exec() {
    local cmd="$1"
    local description="${2:-$1}"
    
    if ! eval "$cmd"; then
        error "Failed to execute: $description"
        return 1
    fi
}

# Show status
show_status() {
    log "=== Ollama Production Status ==="
    echo
    
    # Container status
    if check_ollama_status; then
        if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
            info "Container Status: ✅ Remote server"
            echo "  • URL: $OLLAMA_URL"
        else
            info "Container Status: ✅ Running"
            if ! container_info=$(docker ps --filter "name=$OLLAMA_CONTAINER_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null); then
                warn "Could not retrieve container information"
            else
                echo "$container_info"
            fi
        fi
    else
        error "Container Status: ❌ Not running"
    fi
    
    echo
    
    # Service status
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        info "Systemd Service: 🔗 Remote (not applicable)"
    elif systemctl is-active --quiet "$OLLAMA_SERVICE_NAME" 2>/dev/null; then
        info "Systemd Service: ✅ Active"
    else
        warn "Systemd Service: ⚠️ Inactive"
    fi
    
    echo
    
    # Health check
    if check_ollama_health; then
        info "API Health: ✅ Responding"
    else
        error "API Health: ❌ Not responding"
    fi
    
    echo
    
    # Models
    info "Installed Models:"
    if check_ollama_health; then
        if ! models_response=$(curl -s "$OLLAMA_URL/api/tags" 2>/dev/null); then
            echo "  Failed to retrieve models list"
        elif echo "$models_response" | jq -e '.models' > /dev/null 2>&1; then
            echo "$models_response" | jq -r '.models[] | "  • \(.name) (\(.size))"'
        else
            echo "  No models found or API error"
        fi
    else
        echo "  Cannot retrieve models - API not responding"
    fi
    
    echo
    
    # Resource usage
    info "Resource Usage:"
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        echo "  Remote server - resource stats not available"
    elif check_ollama_status; then
        if ! docker stats "$OLLAMA_CONTAINER_NAME" --no-stream --format "table {{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" 2>/dev/null; then
            echo "  Could not retrieve resource stats"
        fi
    else
        echo "  Container not running"
    fi
    
    echo
    
    # Storage usage
    info "Storage Usage:"
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        echo "  Remote server - storage stats not available"
    elif [ -d "$OLLAMA_DATA_DIR" ]; then
        if ! du_output=$(du -sh "$OLLAMA_DATA_DIR" 2>/dev/null); then
            echo "  Cannot access data directory"
        else
            echo "  $du_output"
        fi
    else
        echo "  Data directory not found"
    fi
    
    echo
    
    # Recent logs
    info "Recent Logs (last 10 lines):"
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        echo "  Remote server - logs not available"
    elif [ -f "$OLLAMA_LOGS_DIR/health.log" ]; then
        if ! tail -n 10 "$OLLAMA_LOGS_DIR/health.log" 2>/dev/null; then
            echo "  Cannot read health log"
        fi
    else
        echo "  Health log not found"
    fi
}

# Start Ollama
start_ollama() {
    log "Starting Ollama..."
    
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        error "Cannot start remote Ollama service from local machine"
        return 1
    fi
    
    if check_ollama_status; then
        info "Ollama is already running"
        return 0
    fi
    
    if ! systemctl start "$OLLAMA_SERVICE_NAME"; then
        error "Failed to start Ollama service"
        return 1
    fi
    
    log "Ollama started successfully"
    
    # Wait for it to be ready
    info "Waiting for Ollama to be ready..."
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if check_ollama_health; then
            log "Ollama is ready!"
            return 0
        fi
        
        info "Attempt $attempt/$max_attempts - waiting 10 seconds..."
        sleep 10
        attempt=$((attempt + 1))
    done
    
    warn "Ollama did not become ready within 5 minutes"
    return 1
}

# Stop Ollama
stop_ollama() {
    log "Stopping Ollama..."
    
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        error "Cannot stop remote Ollama service from local machine"
        return 1
    fi
    
    if ! check_ollama_status; then
        info "Ollama is not running"
        return 0
    fi
    
    if ! systemctl stop "$OLLAMA_SERVICE_NAME"; then
        error "Failed to stop Ollama"
        return 1
    fi
    
    log "Ollama stopped successfully"
}

# Restart Ollama
restart_ollama() {
    log "Restarting Ollama..."
    
    if [[ "$OLLAMA_URL" != "http://localhost:$OLLAMA_PORT" ]]; then
        error "Cannot restart remote Ollama service from local machine"
        return 1
    fi
    
    if ! stop_ollama; then
        warn "Failed to stop Ollama, but continuing with restart..."
    fi
    
    sleep 5
    
    if ! start_ollama; then
        error "Failed to restart Ollama"
        return 1
    fi
}

# Show logs
show_logs() {
    local log_type="${1:-all}"
    
    case "$log_type" in
        "health")
            if [ -f "$OLLAMA_LOGS_DIR/health.log" ]; then
                if ! tail -f "$OLLAMA_LOGS_DIR/health.log"; then
                    error "Failed to read health log"
                    return 1
                fi
            else
                error "Health log not found"
                return 1
            fi
            ;;
        "monitor")
            if [ -f "$OLLAMA_LOGS_DIR/monitor.log" ]; then
                if ! tail -f "$OLLAMA_LOGS_DIR/monitor.log"; then
                    error "Failed to read monitor log"
                    return 1
                fi
            else
                error "Monitor log not found"
                return 1
            fi
            ;;
        "container")
            if ! docker logs -f "$OLLAMA_CONTAINER_NAME"; then
                error "Failed to read container logs"
                return 1
            fi
            ;;
        "service")
            if ! journalctl -u "$OLLAMA_SERVICE_NAME" -f; then
                error "Failed to read service logs"
                return 1
            fi
            ;;
        "all")
            info "Available logs:"
            echo "  • health - Health check logs"
            echo "  • monitor - Monitoring logs"
            echo "  • container - Docker container logs"
            echo "  • service - Systemd service logs"
            echo
            echo "Usage: $0 logs <type>"
            ;;
        *)
            error "Unknown log type: $log_type"
            echo "Available types: health, monitor, container, service"
            return 1
            ;;
    esac
}

# Test backend APIs
test_backend_apis() {
    log "Testing Backend API Endpoints..."
    echo
    
    local all_passed=true
    
    # 1. Test Health Check API
    info "1. Testing Health Check API..."
    if ! health_response=$(curl -s --max-time 10 "$OLLAMA_URL" 2>/dev/null); then
        error "❌ Health Check API - Failed"
        all_passed=false
    else
        info "✅ Health Check API - Working"
    fi
    echo
    
    # 2. Test Models List API
    info "2. Testing Models List API (/api/tags)..."
    if ! models_response=$(curl -s --max-time 10 "$OLLAMA_URL/api/tags" 2>/dev/null); then
        error "❌ Models List API - Failed"
        all_passed=false
    elif echo "$models_response" | jq -e '.models' > /dev/null 2>&1; then
        info "✅ Models List API - Working"
        echo "   Models found:"
        echo "$models_response" | jq -r '.models[] | "   • \(.name) (\(.size))"'
    else
        error "❌ Models List API - Failed"
        all_passed=false
    fi
    echo
    
    # 3. Test LLM Generation API
    info "3. Testing LLM Generation API (/api/generate)..."
    if ! llm_response=$(curl -s -X POST "$OLLAMA_URL/api/generate" \
        -H "Content-Type: application/json" \
        --max-time 30 \
        -d '{
            "model": "codellama:7b",
            "prompt": "Hello, this is a test.",
            "stream": false,
            "options": {
                "temperature": 0.2,
                "top_p": 0.8
            }
        }' 2>/dev/null); then
        error "❌ LLM Generation API - Failed"
        all_passed=false
    elif echo "$llm_response" | jq -e '.response' > /dev/null 2>&1; then
        info "✅ LLM Generation API - Working"
        response_text=$(echo "$llm_response" | jq -r '.response' | head -c 100)
        echo "   Sample response: \"$response_text...\""
    else
        error "❌ LLM Generation API - Failed"
        echo "   Response: $llm_response"
        all_passed=false
    fi
    echo
    
    # 4. Test Embedding API
    info "4. Testing Embedding API (/api/embeddings)..."
    if ! embed_response=$(curl -s -X POST "$OLLAMA_URL/api/embeddings" \
        -H "Content-Type: application/json" \
        --max-time 30 \
        -d '{
            "model": "nomic-embed-text:latest",
            "prompt": "Error: Connection timeout to database server"
        }' 2>/dev/null); then
        error "❌ Embedding API - Failed"
        all_passed=false
    elif echo "$embed_response" | jq -e '.embedding' > /dev/null 2>&1; then
        info "✅ Embedding API - Working"
        embed_size=$(echo "$embed_response" | jq -r '.embedding | length')
        echo "   Embedding size: $embed_size dimensions"
    else
        error "❌ Embedding API - Failed"
        echo "   Response: $embed_response"
        all_passed=false
    fi
    echo
    
    # 5. Test Model Pull API (dry run)
    info "5. Testing Model Pull API (/api/pull)..."
    if ! pull_response=$(curl -s -X POST "$OLLAMA_URL/api/pull" \
        -H "Content-Type: application/json" \
        --max-time 10 \
        -d '{"name": "llama3:8b"}' 2>/dev/null); then
        error "❌ Model Pull API - Failed"
        all_passed=false
    elif echo "$pull_response" | jq -e '.status' > /dev/null 2>&1; then
        info "✅ Model Pull API - Working"
    else
        error "❌ Model Pull API - Failed"
        echo "   Response: $pull_response"
        all_passed=false
    fi
    echo
    
    # 6. Test Model Delete API (dry run)
    info "6. Testing Model Delete API (/api/delete)..."
    if ! delete_response=$(curl -s -X DELETE "$OLLAMA_URL/api/delete" \
        -H "Content-Type: application/json" \
        --max-time 10 \
        -d '{"name": "nonexistent-model"}' 2>/dev/null); then
        error "❌ Model Delete API - Failed"
        all_passed=false
    elif echo "$delete_response" | jq -e '.status' > /dev/null 2>&1; then
        info "✅ Model Delete API - Working"
    else
        error "❌ Model Delete API - Failed"
        echo "   Response: $delete_response"
        all_passed=false
    fi
    echo
    
    # 7. Test Backend-Specific Integration
    info "7. Testing Backend Integration APIs..."
    
    # Test log analysis prompt (similar to what AutoLog backend would send)
    if ! log_analysis_response=$(curl -s -X POST "$OLLAMA_URL/api/generate" \
        -H "Content-Type: application/json" \
        --max-time 60 \
        -d '{
            "model": "codellama:7b",
            "prompt": "Analyze this log error: Connection timeout to database server. Provide a brief analysis.",
            "stream": false,
            "options": {
                "temperature": 0.2,
                "top_p": 0.8
            }
        }' 2>/dev/null); then
        error "❌ Log Analysis API - Failed"
        all_passed=false
    elif echo "$log_analysis_response" | jq -e '.response' > /dev/null 2>&1; then
        info "✅ Log Analysis API - Working"
        analysis_text=$(echo "$log_analysis_response" | jq -r '.response' | head -c 150)
        echo "   Sample analysis: \"$analysis_text...\""
    else
        error "❌ Log Analysis API - Failed"
        all_passed=false
    fi
    echo
    
    # Test embedding for log similarity (AutoLog backend use case)
    if ! log_embedding_response=$(curl -s -X POST "$OLLAMA_URL/api/embeddings" \
        -H "Content-Type: application/json" \
        --max-time 30 \
        -d '{
            "model": "nomic-embed-text:latest",
            "prompt": "Summary: Database connection timeout occurred. Root Cause: Network connectivity issues. Severity: High. Error Patterns: Connection timeout, Network error"
        }' 2>/dev/null); then
        error "❌ Log Embedding API - Failed"
        all_passed=false
    elif echo "$log_embedding_response" | jq -e '.embedding' > /dev/null 2>&1; then
        info "✅ Log Embedding API - Working"
        log_embed_size=$(echo "$log_embedding_response" | jq -r '.embedding | length')
        echo "   Log embedding size: $log_embed_size dimensions"
    else
        error "❌ Log Embedding API - Failed"
        all_passed=false
    fi
    echo
    
    # Summary
    if [ "$all_passed" = true ]; then
        log "🎉 All Backend API Tests Passed!"
        echo "   Your Ollama instance is ready for AutoLog backend integration."
        echo "   Backend Configuration:"
        echo "   • URL: $OLLAMA_URL"
        echo "   • LLM Model: codellama:7b"
        echo "   • Embedding Model: nomic-embed-text:latest"
    else
        error "❌ Some Backend API Tests Failed!"
        echo "   Please check the failed endpoints above."
        return 1
    fi
}

# Test models (original function)
test_models() {
    log "Testing models..."
    
    if ! check_ollama_health; then
        error "Ollama is not responding"
        return 1
    fi
    
    # Get list of models
    if ! models_response=$(curl -s "$OLLAMA_URL/api/tags" 2>/dev/null); then
        error "Failed to get models list"
        return 1
    fi
    
    if ! echo "$models_response" | jq -e '.models' > /dev/null 2>&1; then
        error "Failed to parse models list"
        return 1
    fi
    
    # Test each model
    echo "$models_response" | jq -r '.models[].name' | while read -r model_name; do
        info "Testing model: $model_name"
        
        # Determine if it's an embedding model
        if [[ "$model_name" == *"embed"* ]] || [[ "$model_name" == *"nomic"* ]]; then
            # Test as embedding model
            if ! response=$(curl -s -X POST "$OLLAMA_URL/api/embeddings" \
                -H "Content-Type: application/json" \
                -d "{
                    \"model\": \"$model_name\",
                    \"prompt\": \"Test embedding generation\"
                }" 2>/dev/null); then
                error "❌ $model_name (embedding) - Failed"
            elif echo "$response" | jq -e '.embedding' > /dev/null 2>&1; then
                info "✅ $model_name (embedding) - Working"
            else
                error "❌ $model_name (embedding) - Failed"
            fi
        else
            # Test as LLM model
            if ! response=$(curl -s -X POST "$OLLAMA_URL/api/generate" \
                -H "Content-Type: application/json" \
                -d "{
                    \"model\": \"$model_name\",
                    \"prompt\": \"Hello, this is a test.\",
                    \"stream\": false
                }" 2>/dev/null); then
                error "❌ $model_name (LLM) - Failed"
            elif echo "$response" | jq -e '.response' > /dev/null 2>&1; then
                info "✅ $model_name (LLM) - Working"
            else
                error "❌ $model_name (LLM) - Failed"
            fi
        fi
    done
}

# Pull a model
pull_model() {
    local model_name="$1"
    
    if [ -z "$model_name" ]; then
        error "Model name is required"
        echo "Usage: $0 pull <model_name>"
        return 1
    fi
    
    if ! check_ollama_health; then
        error "Ollama is not responding"
        return 1
    fi
    
    log "Pulling model: $model_name"
    
    # Check if model already exists
    if ! models_response=$(curl -s "$OLLAMA_URL/api/tags" 2>/dev/null); then
        error "Failed to get models list"
        return 1
    fi
    
    if echo "$models_response" | jq -e --arg name "$model_name" '.models[] | select(.name == $name)' > /dev/null 2>&1; then
        info "Model $model_name already exists"
        return 0
    fi
    
    # Pull the model
    if ! response=$(curl -s -X POST "$OLLAMA_URL/api/pull" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"$model_name\"}" 2>/dev/null); then
        error "Failed to pull $model_name"
        return 1
    fi
    
    if echo "$response" | jq -e '.status' > /dev/null 2>&1; then
        log "Successfully initiated pull for $model_name"
        info "This may take several minutes for large models..."
    else
        error "Failed to pull $model_name"
        error "Response: $response"
        return 1
    fi
}

# Remove a model
remove_model() {
    local model_name="$1"
    
    if [ -z "$model_name" ]; then
        error "Model name is required"
        echo "Usage: $0 remove <model_name>"
        return 1
    fi
    
    if ! check_ollama_health; then
        error "Ollama is not responding"
        return 1
    fi
    
    log "Removing model: $model_name"
    
    if ! response=$(curl -s -X DELETE "$OLLAMA_URL/api/delete" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"$model_name\"}" 2>/dev/null); then
        error "Failed to remove $model_name"
        return 1
    fi
    
    if echo "$response" | jq -e '.status' > /dev/null 2>&1; then
        log "Successfully removed $model_name"
    else
        error "Failed to remove $model_name"
        error "Response: $response"
        return 1
    fi
}

# Show system info
show_system_info() {
    log "=== System Information ==="
    echo
    
    info "System Resources:"
    if ! cpu_cores=$(nproc 2>/dev/null); then
        echo "  • CPU: Unknown"
    else
        echo "  • CPU: $cpu_cores cores"
    fi
    
    if ! memory_total=$(free -h | awk 'NR==2{print $2}' 2>/dev/null); then
        echo "  • Memory: Unknown"
    else
        echo "  • Memory: $memory_total total"
    fi
    
    if ! disk_available=$(df -h / | awk 'NR==2{print $4}' 2>/dev/null); then
        echo "  • Disk: Unknown"
    else
        echo "  • Disk: $disk_available available"
    fi
    echo
    
    info "Docker Information:"
    if ! docker_version=$(docker --version 2>/dev/null); then
        echo "  • Version: Not available"
    else
        echo "  • Version: $docker_version"
    fi
    
    if ! docker_status=$(systemctl is-active docker 2>/dev/null); then
        echo "  • Status: Unknown"
    else
        echo "  • Status: $docker_status"
    fi
    echo
    
    info "Ollama Configuration:"
    echo "  • Port: $OLLAMA_PORT"
    echo "  • Data Directory: $OLLAMA_DATA_DIR"
    echo "  • Logs Directory: $OLLAMA_LOGS_DIR"
    echo "  • Container Name: $OLLAMA_CONTAINER_NAME"
    echo "  • Service Name: $OLLAMA_SERVICE_NAME"
    echo
    
    info "Network Information:"
    echo "  • Local URL: $OLLAMA_URL"
    if ! external_ip=$(curl -s ifconfig.me 2>/dev/null); then
        echo "  • External IP: Unknown"
    else
        echo "  • External IP: $external_ip"
    fi
    echo
}

# Clean up
cleanup() {
    log "Cleaning up Ollama..."
    
    # Stop the service
    if ! systemctl stop "$OLLAMA_SERVICE_NAME" 2>/dev/null; then
        warn "Failed to stop service (may already be stopped)"
    fi
    
    # Remove container
    if ! docker rm -f "$OLLAMA_CONTAINER_NAME" 2>/dev/null; then
        warn "Failed to remove container (may not exist)"
    fi
    
    # Disable service
    if ! systemctl disable "$OLLAMA_SERVICE_NAME" 2>/dev/null; then
        warn "Failed to disable service"
    fi
    
    # Remove service file
    if ! rm -f "/etc/systemd/system/$OLLAMA_SERVICE_NAME.service"; then
        warn "Failed to remove service file"
    fi
    
    # Reload systemd
    if ! systemctl daemon-reload; then
        warn "Failed to reload systemd"
    fi
    
    # Remove cron jobs
    if ! crontab -l 2>/dev/null | grep -v "ollama-health-check.sh" | crontab -; then
        warn "Failed to remove health check cron job"
    fi
    
    if ! crontab -l 2>/dev/null | grep -v "ollama-monitor.sh" | crontab -; then
        warn "Failed to remove monitor cron job"
    fi
    
    # Remove scripts
    if ! rm -f /usr/local/bin/ollama-health-check.sh; then
        warn "Failed to remove health check script"
    fi
    
    if ! rm -f /usr/local/bin/ollama-monitor.sh; then
        warn "Failed to remove monitor script"
    fi
    
    # Remove log rotation
    if ! rm -f /etc/logrotate.d/ollama; then
        warn "Failed to remove log rotation config"
    fi
    
    log "Cleanup completed"
    warn "Note: Model data in $OLLAMA_DATA_DIR has been preserved"
    info "To completely remove everything, run: sudo rm -rf $OLLAMA_DATA_DIR $OLLAMA_LOGS_DIR"
}

# Main script logic
main() {
    # Check dependencies first
    check_dependencies
    
    case "${1:-help}" in
        "status")
            show_status
            ;;
        "start")
            start_ollama
            ;;
        "stop")
            stop_ollama
            ;;
        "restart")
            restart_ollama
            ;;
        "logs")
            show_logs "$2"
            ;;
        "test")
            test_models
            ;;
        "test-apis")
            test_backend_apis
            ;;
        "pull")
            pull_model "$2"
            ;;
        "remove")
            remove_model "$2"
            ;;
        "system")
            show_system_info
            ;;
        "cleanup")
            echo "This will stop and remove Ollama completely."
            echo "Model data will be preserved unless manually deleted."
            read -p "Are you sure? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                cleanup
            else
                info "Cleanup cancelled"
            fi
            ;;
        "help"|*)
            echo "Ollama Production Management Script"
            echo "==================================="
            echo
            echo "Usage: $0 [URL] <command> [options]"
            echo
            echo "URL (optional):"
            echo "  http://your-server:80    - Custom Ollama server URL"
            echo "  http://192.168.1.100:80  - Remote server IP"
            echo "  https://ollama.example.com - HTTPS server"
            echo
            echo "Commands:"
            echo "  status                    - Show Ollama status and health"
            echo "  start                     - Start Ollama service"
            echo "  stop                      - Stop Ollama service"
            echo "  restart                   - Restart Ollama service"
            echo "  logs [type]               - Show logs (health|monitor|container|service|all)"
            echo "  test                      - Test all installed models"
            echo "  test-apis                 - Test all backend API endpoints"
            echo "  pull <model_name>         - Pull a model"
            echo "  remove <model_name>       - Remove a model"
            echo "  system                    - Show system information"
            echo "  cleanup                   - Remove Ollama completely"
            echo "  help                      - Show this help"
            echo
            echo "Examples:"
            echo "  $0 status                 # Check local status"
            echo "  $0 http://your-server:80 status  # Check remote status"
            echo "  $0 http://192.168.1.100:80 test  # Test remote models"
            echo "  $0 http://192.168.1.100:80 test-apis  # Test remote backend APIs"
            echo "  $0 https://ollama.example.com pull llama3:8b  # Pull model on remote server"
            echo "  $0 logs container         # Follow local container logs"
            echo "  $0 test                   # Test local models"
            echo "  $0 test-apis              # Test all backend APIs"
            echo
            echo "Log Types:"
            echo "  health                    - Health check logs"
            echo "  monitor                   - Monitoring logs"
            echo "  container                 - Docker container logs"
            echo "  service                   - Systemd service logs"
            echo "  all                       - Show available log types"
            ;;
    esac
}

# Run main function with all arguments
main "$@" 