#!/bin/bash

# Production Ollama Deployment Script
# This script deploys a production-grade Ollama instance with:
# - Systemd service for auto-restart
# - Docker container with persistent storage
# - Required models (llama3:8b and nomic-embed-text:latest)
# - Health monitoring and logging
# - All endpoints exposed on port 80
# - Configurable resource limits or full resource utilization
#
# INTERACTIVE RESOURCE SELECTION:
# ===============================
# The script will present you with multiple resource configuration options:
# 1) 🚀 FULL SYSTEM RESOURCES - Use all available RAM and CPU
# 2) ⚡ HIGH PERFORMANCE - 16GB RAM, 8 CPU cores
# 3) 🎯 BALANCED - 8GB RAM, 4 CPU cores (default)
# 4) 💡 LIGHTWEIGHT - 4GB RAM, 2 CPU cores
# 5) 🔧 CUSTOM CONFIGURATION - Choose your own limits
# 6) 📊 SHOW SYSTEM DETAILS - View detailed system information
#
# NON-INTERACTIVE MODE:
# ====================
# For automation, use: ./deploy-ollama-production.sh --non-interactive
# This will use the default balanced configuration (8GB RAM, 4 CPU cores)
#
# RESTART ON FAILURE:
# ===================
# - Systemd service: Restart=on-failure (auto-restart on failure)
# - Docker container: --restart unless-stopped (auto-restart)
# - Health checks: Every 5 minutes with automatic container restart
# - Multiple layers of protection ensure high availability

set -e

# Configuration
OLLAMA_VERSION="latest"
OLLAMA_PORT="80"
OLLAMA_HOST="0.0.0.0"
OLLAMA_DATA_DIR="/opt/ollama"
OLLAMA_MODELS_DIR="$OLLAMA_DATA_DIR/models"
OLLAMA_LOGS_DIR="/var/log/ollama"
OLLAMA_SERVICE_NAME="ollama"
OLLAMA_CONTAINER_NAME="ollama-production"

# Resource Configuration - Will be set by user interaction
OLLAMA_MEMORY_LIMIT=""
OLLAMA_CPU_LIMIT=""
OLLAMA_USE_FULL_RESOURCES=""

# Required models
LLM_MODEL="llama3:8b"
OPTIONAL_MODELS=("codellama:7b" "nomic-embed-text:latest")
EMBED_MODEL="nomic-embed-text:latest"

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

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Get system information
get_system_info() {
    local total_mem=$(free -g | awk 'NR==2 {print $2}')
    local total_cpu=$(nproc)
    local available_space=$(df -h / | awk 'NR==2 {print $4}')
    
    echo "System Information:"
    echo "  • Total RAM: ${total_mem}GB"
    echo "  • CPU Cores: ${total_cpu}"
    echo "  • Available Disk: ${available_space}"
    echo
}

# Interactive resource selection
select_resource_configuration() {
    local total_mem=$(free -g | awk 'NR==2 {print $2}')
    local total_cpu=$(nproc)
    
    log "Resource Configuration Selection"
    echo "================================="
    get_system_info
    
    echo "Choose your resource configuration:"
    echo
    echo "1) 🚀 FULL SYSTEM RESOURCES (Recommended for dedicated servers)"
    echo "   • Memory: All available RAM (${total_mem}GB)"
    echo "   • CPU: All available cores (${total_cpu})"
    echo "   • Best for: Maximum performance, dedicated servers"
    echo
    echo "2) ⚡ HIGH PERFORMANCE (16GB RAM, 8 CPU cores)"
    echo "   • Memory: 16GB"
    echo "   • CPU: 8 cores"
    echo "   • Best for: High-performance workloads"
    echo
    echo "3) 🎯 BALANCED (8GB RAM, 4 CPU cores)"
    echo "   • Memory: 8GB"
    echo "   • CPU: 4 cores"
    echo "   • Best for: Standard workloads, shared servers"
    echo
    echo "4) 💡 LIGHTWEIGHT (4GB RAM, 2 CPU cores)"
    echo "   • Memory: 4GB"
    echo "   • CPU: 2 cores"
    echo "   • Best for: Development, testing, limited resources"
    echo
    echo "5) 🔧 CUSTOM CONFIGURATION"
    echo "   • Choose your own memory and CPU limits"
    echo "   • Best for: Specific requirements"
    echo
    echo "6) 📊 SHOW SYSTEM DETAILS"
    echo "   • Display detailed system information"
    echo
    
    while true; do
        read -p "Enter your choice (1-6): " choice
        case $choice in
            1)
                log "Selected: FULL SYSTEM RESOURCES"
                OLLAMA_USE_FULL_RESOURCES="true"
                OLLAMA_MEMORY_LIMIT=""
                OLLAMA_CPU_LIMIT=""
                break
                ;;
            2)
                log "Selected: HIGH PERFORMANCE"
                OLLAMA_USE_FULL_RESOURCES="false"
                OLLAMA_MEMORY_LIMIT="16g"
                OLLAMA_CPU_LIMIT="8"
                break
                ;;
            3)
                log "Selected: BALANCED"
                OLLAMA_USE_FULL_RESOURCES="false"
                OLLAMA_MEMORY_LIMIT="8g"
                OLLAMA_CPU_LIMIT="4"
                break
                ;;
            4)
                log "Selected: LIGHTWEIGHT"
                OLLAMA_USE_FULL_RESOURCES="false"
                OLLAMA_MEMORY_LIMIT="4g"
                OLLAMA_CPU_LIMIT="2"
                break
                ;;
            5)
                select_custom_configuration
                break
                ;;
            6)
                show_detailed_system_info
                echo
                echo "Please select your resource configuration:"
                continue
                ;;
            *)
                echo "Invalid choice. Please enter a number between 1-6."
                ;;
        esac
    done
    
    # Display selected configuration
    echo
    log "Selected Configuration:"
    if [ "$OLLAMA_USE_FULL_RESOURCES" = "true" ]; then
        echo "  • Memory: Unlimited (all available RAM)"
        echo "  • CPU: Unlimited (all available cores)"
    else
        echo "  • Memory: $OLLAMA_MEMORY_LIMIT"
        echo "  • CPU: $OLLAMA_CPU_LIMIT cores"
    fi
    echo
}

# Custom configuration selection
select_custom_configuration() {
    local total_mem=$(free -g | awk 'NR==2 {print $2}')
    local total_cpu=$(nproc)
    
    log "Custom Resource Configuration"
    echo "============================="
    
    # Memory selection
    echo "Memory Configuration:"
    echo "Available options:"
    echo "  1) Unlimited (all available RAM: ${total_mem}GB)"
    echo "  2) 32GB"
    echo "  3) 16GB"
    echo "  4) 8GB"
    echo "  5) 4GB"
    echo "  6) Custom amount"
    echo
    
    while true; do
        read -p "Select memory configuration (1-6): " mem_choice
        case $mem_choice in
            1) OLLAMA_MEMORY_LIMIT="unlimited"; break ;;
            2) OLLAMA_MEMORY_LIMIT="32g"; break ;;
            3) OLLAMA_MEMORY_LIMIT="16g"; break ;;
            4) OLLAMA_MEMORY_LIMIT="8g"; break ;;
            5) OLLAMA_MEMORY_LIMIT="4g"; break ;;
            6)
                read -p "Enter custom memory amount (e.g., 12g, 24g): " custom_mem
                if [[ $custom_mem =~ ^[0-9]+g$ ]]; then
                    OLLAMA_MEMORY_LIMIT="$custom_mem"
                    break
                else
                    echo "Invalid format. Please use format like '12g', '24g'"
                fi
                ;;
            *) echo "Invalid choice. Please enter a number between 1-6." ;;
        esac
    done
    
    echo
    echo "CPU Configuration:"
    echo "Available options:"
    echo "  1) Unlimited (all available cores: ${total_cpu})"
    echo "  2) 16 cores"
    echo "  3) 8 cores"
    echo "  4) 4 cores"
    echo "  5) 2 cores"
    echo "  6) Custom amount"
    echo
    
    while true; do
        read -p "Select CPU configuration (1-6): " cpu_choice
        case $cpu_choice in
            1) OLLAMA_CPU_LIMIT="unlimited"; break ;;
            2) OLLAMA_CPU_LIMIT="16"; break ;;
            3) OLLAMA_CPU_LIMIT="8"; break ;;
            4) OLLAMA_CPU_LIMIT="4"; break ;;
            5) OLLAMA_CPU_LIMIT="2"; break ;;
            6)
                read -p "Enter custom CPU cores (e.g., 6, 12): " custom_cpu
                if [[ $custom_cpu =~ ^[0-9]+$ ]]; then
                    OLLAMA_CPU_LIMIT="$custom_cpu"
                    break
                else
                    echo "Invalid format. Please enter a number."
                fi
                ;;
            *) echo "Invalid choice. Please enter a number between 1-6." ;;
        esac
    done
    
    OLLAMA_USE_FULL_RESOURCES="false"
}

# Show detailed system information
show_detailed_system_info() {
    echo
    log "Detailed System Information"
    echo "==========================="
    
    # CPU Information
    echo "CPU Information:"
    echo "  • Total Cores: $(nproc)"
    echo "  • CPU Model: $(grep 'model name' /proc/cpuinfo | head -1 | cut -d: -f2 | xargs)"
    echo "  • Architecture: $(uname -m)"
    echo
    
    # Memory Information
    echo "Memory Information:"
    local total_mem=$(free -h | awk 'NR==2{print $2}')
    local used_mem=$(free -h | awk 'NR==2{print $3}')
    local free_mem=$(free -h | awk 'NR==2{print $4}')
    echo "  • Total RAM: $total_mem"
    echo "  • Used RAM: $used_mem"
    echo "  • Free RAM: $free_mem"
    echo
    
    # Disk Information
    echo "Disk Information:"
    df -h / | awk 'NR==2 {print "  • Total: " $2 ", Used: " $3 ", Available: " $4 ", Usage: " $5}'
    echo
    
    # Docker Information
    echo "Docker Information:"
    if command -v docker &> /dev/null; then
        echo "  • Version: $(docker --version)"
        echo "  • Status: $(systemctl is-active docker)"
    else
        echo "  • Docker: Not installed"
    fi
    echo
}

# Check system requirements
check_requirements() {
    log "Checking system requirements..."
    
    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        error "Docker is not running. Please start Docker first."
        exit 1
    fi
    
    # Check available disk space (need at least 20GB)
    available_space=$(df / | awk 'NR==2 {print $4}')
    if [ "$available_space" -lt 20971520 ]; then
        warn "Less than 20GB available space. Ollama models require significant space."
    fi
    
    # Check available memory (need at least 8GB)
    total_mem=$(free -g | awk 'NR==2 {print $2}')
    if [ "$total_mem" -lt 8 ]; then
        warn "Less than 8GB RAM available. Ollama may not perform optimally."
    fi
    
    log "System requirements check completed"
}

# Create necessary directories
create_directories() {
    log "Creating necessary directories..."
    
    mkdir -p "$OLLAMA_DATA_DIR"
    mkdir -p "$OLLAMA_MODELS_DIR"
    mkdir -p "$OLLAMA_LOGS_DIR"
    
    # Set proper permissions
    chown -R 1000:1000 "$OLLAMA_DATA_DIR"
    chown -R 1000:1000 "$OLLAMA_LOGS_DIR"
    chmod -R 755 "$OLLAMA_DATA_DIR"
    chmod -R 755 "$OLLAMA_LOGS_DIR"
    
    log "Directories created successfully"
}

# Stop and remove existing container
cleanup_existing() {
    log "Cleaning up existing Ollama container..."
    
    if docker ps -a --format "table {{.Names}}" | grep -q "$OLLAMA_CONTAINER_NAME"; then
        docker stop "$OLLAMA_CONTAINER_NAME" 2>/dev/null || true
        docker rm "$OLLAMA_CONTAINER_NAME" 2>/dev/null || true
        log "Existing container removed"
    fi
}

# Pull Ollama Docker image
pull_ollama_image() {
    log "Pulling Ollama Docker image..."
    
    docker pull ollama/ollama:$OLLAMA_VERSION
    
    if [ $? -eq 0 ]; then
        log "Ollama image pulled successfully"
    else
        error "Failed to pull Ollama image"
        exit 1
    fi
}

# Create and start Ollama container
start_ollama_container() {
    log "Starting Ollama container..."
    
    # Build Docker run command with resource configuration
    local docker_cmd="docker run -d"
    docker_cmd="$docker_cmd --name \"$OLLAMA_CONTAINER_NAME\""
    docker_cmd="$docker_cmd --restart unless-stopped"
    docker_cmd="$docker_cmd -p \"$OLLAMA_PORT:11434\""
    docker_cmd="$docker_cmd -v \"$OLLAMA_MODELS_DIR:/root/.ollama\""
    docker_cmd="$docker_cmd -v \"$OLLAMA_LOGS_DIR:/var/log/ollama\""
    docker_cmd="$docker_cmd -e OLLAMA_HOST=\"$OLLAMA_HOST\""
    docker_cmd="$docker_cmd -e OLLAMA_ORIGINS=\"*\""
    docker_cmd="$docker_cmd --ulimit nofile=65536:65536"
    
    # Add resource limits based on configuration
    if [ "$OLLAMA_USE_FULL_RESOURCES" = "true" ]; then
        info "Using full system resources (no limits)"
        # No memory or CPU limits - use all available resources
    else
        # Add memory limit
        if [ "$OLLAMA_MEMORY_LIMIT" != "unlimited" ]; then
            docker_cmd="$docker_cmd --memory=\"$OLLAMA_MEMORY_LIMIT\""
            info "Memory limit: $OLLAMA_MEMORY_LIMIT"
        else
            info "Memory: unlimited (using all available)"
        fi
        
        # Add CPU limit
        if [ "$OLLAMA_CPU_LIMIT" != "unlimited" ]; then
            docker_cmd="$docker_cmd --cpus=\"$OLLAMA_CPU_LIMIT\""
            info "CPU limit: $OLLAMA_CPU_LIMIT cores"
        else
            info "CPU: unlimited (using all available cores)"
        fi
    fi
    
    docker_cmd="$docker_cmd ollama/ollama:$OLLAMA_VERSION"
    
    # Log the command being executed
    log "Executing: $docker_cmd"
    
    # Execute the Docker command
    eval $docker_cmd
    
    if [ $? -eq 0 ]; then
        log "Ollama container started successfully"
    else
        error "Failed to start Ollama container"
        exit 1
    fi
}

# Wait for Ollama to be ready
wait_for_ollama() {
    log "Waiting for Ollama to be ready..."
    
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s --max-time 5 "http://localhost:$OLLAMA_PORT" > /dev/null 2>&1; then
            log "Ollama is ready!"
            return 0
        fi
        
        info "Attempt $attempt/$max_attempts - waiting 10 seconds..."
        sleep 10
        attempt=$((attempt + 1))
    done
    
    error "Ollama did not become ready within 5 minutes"
    return 1
}

# Pull required models
pull_models() {
    log "Pulling required models..."
    
    # Function to pull a model
    pull_model() {
        local model_name=$1
        local model_type=$2
        
        info "Pulling $model_type model: $model_name"
        
        # Check if model already exists
        if curl -s "http://localhost:$OLLAMA_PORT/api/tags" | jq -e --arg name "$model_name" '.models[] | select(.name == $name)' > /dev/null 2>&1; then
            info "Model $model_name already exists, skipping download"
            return 0
        fi
        
        # Pull the model
        response=$(curl -s -X POST "http://localhost:$OLLAMA_PORT/api/pull" \
            -H "Content-Type: application/json" \
            -d "{\"name\": \"$model_name\"}")
        
        if echo "$response" | jq -e '.status' > /dev/null 2>&1; then
            info "Successfully initiated pull for $model_name"
            info "This may take several minutes for large models..."
            return 0
        else
            error "Failed to pull $model_name"
            error "Response: $response"
            return 1
        fi
    }
    
    # Pull LLM model (llama3:8b)
    pull_model "$LLM_MODEL" "LLM"
    
    # Optionally pull other models
    if [ -z "$NON_INTERACTIVE" ]; then
        echo
        info "Optional: Pull additional models (codellama:7b, nomic-embed-text:latest)"
        for opt_model in "${OPTIONAL_MODELS[@]}"; do
            read -p "Do you want to pull $opt_model? (y/N): " yn
            case $yn in
                [Yy]*) pull_model "$opt_model" "optional";;
                *) info "Skipping $opt_model";;
            esac
        done
    fi
    
    log "Model pulling completed"
}

# Test models
test_models() {
    log "Testing models..."
    
    # Function to test a model
    test_model() {
        local model_name=$1
        local model_type=$2
        
        info "Testing $model_type model: $model_name"
        
        if [ "$model_type" = "embedding" ]; then
            # Test embedding model
            response=$(curl -s -X POST "http://localhost:$OLLAMA_PORT/api/embeddings" \
                -H "Content-Type: application/json" \
                -d "{\n                    \"model\": \"$model_name\",\n                    \"prompt\": \"Test embedding generation\"\n                }")
        else
            # Test LLM model
            response=$(curl -s -X POST "http://localhost:$OLLAMA_PORT/api/generate" \
                -H "Content-Type: application/json" \
                -d "{\n                    \"model\": \"$model_name\",\n                    \"prompt\": \"Hello, this is a test.\",\n                    \"stream\": false\n                }")
        fi
        
        if echo "$response" | jq -e '.embedding // .response' > /dev/null 2>&1; then
            info "$model_name is working correctly"
            return 0
        else
            error "$model_name test failed"
            error "Response: $response"
            return 1
        fi
    }
    
    # Test LLM model (llama3:8b)
    test_model "$LLM_MODEL" "LLM"
    
    # Optionally test other models
    for opt_model in "${OPTIONAL_MODELS[@]}"; do
        test_model "$opt_model" "optional"
    done
    
    log "Model testing completed"
}

# Create systemd service
create_systemd_service() {
    log "Creating systemd service..."
    
    cat > /etc/systemd/system/ollama.service << EOF
[Unit]
Description=Ollama AI Service
Documentation=https://ollama.ai
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/docker start $OLLAMA_CONTAINER_NAME
ExecStop=/usr/bin/docker stop $OLLAMA_CONTAINER_NAME
TimeoutStartSec=0
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    
    # Reload systemd and enable service
    systemctl daemon-reload
    systemctl enable ollama.service
    
    log "Systemd service created and enabled"
}

# Create health check script
create_health_check() {
    log "Creating health check script..."
    
    cat > /usr/local/bin/ollama-health-check.sh << 'EOF'
#!/bin/bash

OLLAMA_URL="http://localhost:80"
LOG_FILE="/var/log/ollama/health.log"

# Create log directory if it doesn't exist
mkdir -p "$(dirname "$LOG_FILE")"

# Function to log messages
log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

# Check if Ollama is responding
if curl -s --max-time 10 "$OLLAMA_URL" > /dev/null 2>&1; then
    log_message "Health check passed"
    exit 0
else
    log_message "Health check failed - Ollama not responding"
    
    # Try to restart the container
    if docker restart ollama-production > /dev/null 2>&1; then
        log_message "Container restarted successfully"
    else
        log_message "Failed to restart container"
    fi
    
    exit 1
fi
EOF
    
    chmod +x /usr/local/bin/ollama-health-check.sh
    
    # Create cron job for health check (every 5 minutes)
    (crontab -l 2>/dev/null; echo "*/5 * * * * /usr/local/bin/ollama-health-check.sh") | crontab -
    
    log "Health check script created and scheduled"
}

# Create monitoring script
create_monitoring() {
    log "Creating monitoring script..."
    
    cat > /usr/local/bin/ollama-monitor.sh << 'EOF'
#!/bin/bash

OLLAMA_URL="http://localhost:80"
LOG_FILE="/var/log/ollama/monitor.log"

# Create log directory if it doesn't exist
mkdir -p "$(dirname "$LOG_FILE")"

# Function to log messages
log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

# Get container stats
container_stats=$(docker stats ollama-production --no-stream --format "table {{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}")

# Get model information
models_info=$(curl -s "$OLLAMA_URL/api/tags" 2>/dev/null | jq -r '.models[] | "\(.name) (\(.size))"' 2>/dev/null || echo "Failed to get models")

# Log the information
log_message "=== Ollama Monitoring Report ==="
log_message "Container Stats:"
log_message "$container_stats"
log_message "Installed Models:"
log_message "$models_info"
log_message "================================"
EOF
    
    chmod +x /usr/local/bin/ollama-monitor.sh
    
    # Create cron job for monitoring (every hour)
    (crontab -l 2>/dev/null; echo "0 * * * * /usr/local/bin/ollama-monitor.sh") | crontab -
    
    log "Monitoring script created and scheduled"
}

# Create log rotation
setup_log_rotation() {
    log "Setting up log rotation..."
    
    cat > /etc/logrotate.d/ollama << EOF
$OLLAMA_LOGS_DIR/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 644 root root
    postrotate
        systemctl reload ollama.service > /dev/null 2>&1 || true
    endscript
}
EOF
    
    log "Log rotation configured"
}

# Display final status
show_status() {
    log "=== Ollama Production Deployment Complete ==="
    echo
    info "Service Information:"
    echo "  • Container Name: $OLLAMA_CONTAINER_NAME"
    echo "  • Service Name: $OLLAMA_SERVICE_NAME"
    echo "  • Port: $OLLAMA_PORT"
    echo "  • Data Directory: $OLLAMA_DATA_DIR"
    echo "  • Logs Directory: $OLLAMA_LOGS_DIR"
    echo
    info "Resource Configuration:"
    if [ "$OLLAMA_USE_FULL_RESOURCES" = "true" ]; then
        echo "  • Memory: Unlimited (full system resources)"
        echo "  • CPU: Unlimited (all available cores)"
    else
        echo "  • Memory: $OLLAMA_MEMORY_LIMIT"
        echo "  • CPU: $OLLAMA_CPU_LIMIT cores"
    fi
    echo "  • File Descriptors: 65536"
    echo
    info "Models Installed:"
    echo "  • Default LLM Model: $LLM_MODEL"
    echo "  • Optional Models: ${OPTIONAL_MODELS[*]}"
    echo
    info "Management Commands:"
    echo "  • Check status: systemctl status ollama"
    echo "  • Start service: systemctl start ollama"
    echo "  • Stop service: systemctl stop ollama"
    echo "  • View logs: journalctl -u ollama -f"
    echo "  • Health check: /usr/local/bin/ollama-health-check.sh"
    echo "  • Monitor: /usr/local/bin/ollama-monitor.sh"
    echo
    info "API Endpoints:"
    echo "  • Health: http://localhost:$OLLAMA_PORT"
    echo "  • Generate: http://localhost:$OLLAMA_PORT/api/generate"
    echo "  • Embeddings: http://localhost:$OLLAMA_PORT/api/embeddings"
    echo "  • Models: http://localhost:$OLLAMA_PORT/api/tags"
    echo
    info "Test Commands:"
    echo "  • Test LLM: curl -X POST http://localhost:$OLLAMA_PORT/api/generate -H 'Content-Type: application/json' -d '{\"model\":\"$LLM_MODEL\",\"prompt\":\"Hello\",\"stream\":false}'"
    echo "  • Test Embedding: curl -X POST http://localhost:$OLLAMA_PORT/api/embeddings -H 'Content-Type: application/json' -d '{\"model\":\"$EMBED_MODEL\",\"prompt\":\"Test\"}'"
    echo
    log "Ollama is now running in production mode!"
}

# Main deployment function
main() {
    log "Starting Ollama production deployment..."
    
    # Check for non-interactive mode
    if [ "$1" = "--non-interactive" ]; then
        log "Non-interactive mode detected"
        # Use default balanced configuration
        OLLAMA_USE_FULL_RESOURCES="false"
        OLLAMA_MEMORY_LIMIT="8g"
        OLLAMA_CPU_LIMIT="4"
        log "Using default configuration: 8GB RAM, 4 CPU cores"
    else
        check_root
        check_requirements
        select_resource_configuration
    fi
    
    create_directories
    cleanup_existing
    pull_ollama_image
    start_ollama_container
    wait_for_ollama
    pull_models
    test_models
    create_systemd_service
    create_health_check
    create_monitoring
    setup_log_rotation
    show_status
    
    log "Deployment completed successfully!"
}

# Run main function
main "$@" 