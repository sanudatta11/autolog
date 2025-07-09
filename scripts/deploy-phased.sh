#!/bin/bash

# AutoLog Phased Deployment Script
# This script orchestrates the complete deployment in phases using Terraform targeting

set -e

echo "🚀 AutoLog Phased Deployment"
echo "============================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

# Check prerequisites
print_status "Checking prerequisites..."

# Check if Azure CLI is installed
if ! command -v az &> /dev/null; then
    print_error "Azure CLI is not installed. Please install it first."
    exit 1
fi

# Check if Terraform is installed
if ! command -v terraform &> /dev/null; then
    print_error "Terraform is not installed. Please install it first."
    exit 1
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed. Please install it first."
    exit 1
fi

# Check if SWA CLI is installed
if ! command -v swa &> /dev/null; then
    print_error "SWA CLI is not installed. Please install it first."
    exit 1
fi

print_success "All prerequisites are installed!"

# Check if user is logged into Azure
print_status "Checking Azure login..."
if ! az account show &> /dev/null; then
    print_error "Not logged into Azure. Please run 'az login' first."
    exit 1
fi

print_success "Logged into Azure!"

# Parse command line arguments
PHASE="all"
SKIP_PHASES=()

while [[ $# -gt 0 ]]; do
    case $1 in
        --phase)
            PHASE="$2"
            shift 2
            ;;
        --skip-phase)
            SKIP_PHASES+=("$2")
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --phase PHASE        Run specific phase (1, 2, 3, 4, 5, 6, or all)"
            echo "  --skip-phase PHASE   Skip specific phase (can be used multiple times)"
            echo "  --help               Show this help message"
            echo ""
            echo "Phases:"
            echo "  1 - Create container registry infrastructure"
            echo "  2 - Build and push Docker images"
            echo "  3 - Deploy Ollama container service"
            echo "  4 - Deploy custom applications (backend, logparser)"
            echo "  5 - Update to custom images"
            echo "  6 - Deploy frontend with SWA"
            echo ""
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Function to check if phase should be skipped
should_skip_phase() {
    local phase=$1
    for skip_phase in "${SKIP_PHASES[@]}"; do
        if [[ "$skip_phase" == "$phase" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to run a phase
run_phase() {
    local phase=$1
    local description=$2
    local script=$3
    local terraform_targets=$4
    
    if should_skip_phase "$phase"; then
        print_warning "Skipping Phase $phase: $description"
        return 0
    fi
    
    if [[ "$PHASE" != "all" && "$PHASE" != "$phase" ]]; then
        print_warning "Skipping Phase $phase: $description (not requested)"
        return 0
    fi
    
    echo ""
    print_status "Starting Phase $phase: $description"
    echo "================================================"
    
    if [[ -n "$script" ]]; then
        bash "$script"
    else
        cd terraform
        
        # Initialize Terraform if needed
        if [ ! -d ".terraform" ]; then
            print_status "Initializing Terraform..."
            terraform init
        fi
        
        # Apply with targets if specified
        if [[ -n "$terraform_targets" ]]; then
            print_status "Applying Terraform with targets: $terraform_targets"
            # Split targets by space and apply each as a separate -target flag
            target_args=""
            for target in $terraform_targets; do
                target_args="$target_args -target=$target"
            done
            terraform apply -auto-approve $target_args
        else
            print_status "Applying Terraform..."
            terraform apply -auto-approve
        fi
        
        cd ..
    fi
    
    print_success "Phase $phase completed: $description"
}

# Main deployment logic
echo ""
print_status "Starting phased deployment..."

# Phase 1: Registry Infrastructure
run_phase "1" "Create Container Registry Infrastructure" "" "azurerm_resource_group.main azurerm_container_registry.main"

# Phase 2: Build and Push Images
run_phase "2" "Build and Push Docker Images" "scripts/phase2-build-images.sh" ""

# Phase 3: Deploy Ollama Container Service
run_phase "3" "Deploy Ollama Container Service" "scripts/phase3-deploy-ollama.sh" ""

# Phase 4: Deploy Custom Applications
run_phase "4" "Deploy Custom Applications" "" "azurerm_postgresql_flexible_server.main azurerm_postgresql_flexible_server_database.main azurerm_container_app_environment.main azurerm_container_app.backend azurerm_container_app.logparser"

# Phase 5: Update to Custom Images
run_phase "5" "Update to Custom Images" "scripts/phase5-update-images.sh" ""

# Phase 6: Frontend Deployment
run_phase "6" "Deploy Frontend with SWA" "scripts/phase6-deploy-frontend.sh" ""

echo ""
print_success "🎉 Deployment completed successfully!"
echo ""
print_status "Next steps:"
echo "  1. Configure your SWA custom domain (if needed)"
echo "  2. Set up monitoring and logging"
echo "  3. Configure backup and disaster recovery"
echo "  4. Set up CI/CD pipelines for automated deployments"
echo ""
print_status "For more information, see the documentation in the docs/ folder." 