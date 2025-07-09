#!/bin/bash

# AutoLog Phase 3: Deploy Ollama Container Service
# This script deploys Ollama using the public image as a container service

set -e

echo "🚀 AutoLog Phase 3: Deploy Ollama Container Service"
echo "=================================================="
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

# Load environment variables
print_status "Loading environment configuration..."

# Check if Terraform state exists
cd terraform
if [ ! -f "terraform.tfstate" ]; then
    print_error "Terraform state not found. Please run Phase 1 first."
    exit 1
fi

cd ..

# Load variables from terraform.tfvars
ENVIRONMENT=$(grep 'environment' terraform/terraform.tfvars | cut -d'"' -f2 || echo "dev")
OLLAMA_MODEL=$(grep 'ollama_model' terraform/terraform.tfvars | cut -d'"' -f2 || echo "llama2:13b")
OLLAMA_EMBED_MODEL=$(grep 'ollama_embed_model' terraform/terraform.tfvars | cut -d'"' -f2 || echo "nomic-embed-text:latest")

print_status "Environment: $ENVIRONMENT"
print_status "Ollama Model: $OLLAMA_MODEL"
print_status "Ollama Embed Model: $OLLAMA_EMBED_MODEL"

print_status "Using public Ollama image: ollama/ollama:latest"

# Deploy Ollama using Terraform
print_status "Deploying Ollama container service..."

cd terraform

# Initialize Terraform if needed
if [ ! -d ".terraform" ]; then
    print_status "Initializing Terraform..."
    terraform init
fi

# Apply Terraform to deploy Ollama
print_status "Applying Terraform for Ollama deployment..."

# Create a temporary terraform configuration for Ollama
cat > "ollama.tf" << 'EOF'
# Ollama Container App - Public Image
resource "azurerm_container_app" "ollama" {
  name                         = "autolog-${var.environment}-ollama"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"

  identity {
    type = "SystemAssigned"
  }
  
  template {
    container {
      name   = "ollama"
      image  = "ollama/ollama:latest"
      cpu    = var.ollama_cpu
      memory = var.ollama_memory

      env {
        name  = "OLLAMA_HOST"
        value = "0.0.0.0"
      }
      
      # Add startup command to download models
      command = ["/bin/sh", "-c"]
      args = [
        "ollama serve & sleep 10 && ollama pull ${var.ollama_model} && ollama pull ${var.ollama_embed_model} && wait"
      ]
    }
  }

  ingress {
    external_enabled = true
    target_port     = 11434
    transport       = "http"
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
}
EOF

# Apply the Ollama configuration
terraform apply -auto-approve -target=azurerm_container_app.ollama

if [ $? -ne 0 ]; then
    print_error "Failed to deploy Ollama container app"
    exit 1
fi

# Get Ollama URL
OLLAMA_URL=$(terraform output -raw ollama_url 2>/dev/null || echo "")

if [ -z "$OLLAMA_URL" ]; then
    print_warning "Could not retrieve Ollama URL from Terraform output"
else
    print_success "Ollama deployed successfully!"
    print_status "Ollama URL: $OLLAMA_URL"
fi

# Clean up temporary file
rm -f ollama.tf

cd ..

print_success "🎉 Phase 3 completed: Ollama container service deployed!"
echo ""
print_status "Next steps:"
echo "  - Run Phase 4 to deploy custom applications"
echo "  - Ollama models are being downloaded in the background"
echo "  - You can check model status at: $OLLAMA_URL/api/tags"
echo "  - Models being downloaded: $OLLAMA_MODEL, $OLLAMA_EMBED_MODEL"
echo "" 