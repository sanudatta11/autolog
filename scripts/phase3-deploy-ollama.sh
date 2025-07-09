#!/bin/bash

# AutoLog Phase 3: Deploy Ollama Container Service
# This script builds a custom Ollama image and deploys it as a container service

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

# Source terraform outputs to get registry info
cd terraform
if [ ! -f "terraform.tfstate" ]; then
    print_error "Terraform state not found. Please run Phase 1 first."
    exit 1
fi

# Get registry information from Terraform outputs
REGISTRY_URL=$(terraform output -raw container_registry_login_server 2>/dev/null || echo "")
REGISTRY_USERNAME=$(terraform output -raw container_registry_username 2>/dev/null || echo "")
REGISTRY_PASSWORD=$(terraform output -raw container_registry_password 2>/dev/null || echo "")

if [ -z "$REGISTRY_URL" ]; then
    print_error "Container registry information not found. Please run Phase 1 first."
    exit 1
fi

cd ..

print_success "Registry URL: $REGISTRY_URL"

# Load variables from terraform.tfvars
ENVIRONMENT=$(grep 'environment' terraform/terraform.tfvars | cut -d'"' -f2 || echo "dev")
OLLAMA_MODEL=$(grep 'ollama_model' terraform/terraform.tfvars | cut -d'"' -f2 || echo "llama2:13b")
OLLAMA_EMBED_MODEL=$(grep 'ollama_embed_model' terraform/terraform.tfvars | cut -d'"' -f2 || echo "nomic-embed-text:latest")

print_status "Environment: $ENVIRONMENT"
print_status "Ollama Model: $OLLAMA_MODEL"
print_status "Ollama Embed Model: $OLLAMA_EMBED_MODEL"

# Create temporary directory for Ollama build
TEMP_DIR=$(mktemp -d)
print_status "Created temporary directory: $TEMP_DIR"

# Create Dockerfile for custom Ollama image
print_status "Creating custom Ollama Dockerfile..."

cat > "$TEMP_DIR/Dockerfile" << EOF
# Use official Ollama image as base
FROM ollama/ollama:latest

# Set environment variables
ENV OLLAMA_HOST=0.0.0.0
ENV OLLAMA_ORIGINS=*

# Create startup script
RUN echo '#!/bin/bash' > /startup.sh && \\
    echo 'ollama serve &' >> /startup.sh && \\
    echo 'sleep 10' >> /startup.sh && \\
    echo 'ollama pull $OLLAMA_MODEL' >> /startup.sh && \\
    echo 'ollama pull $OLLAMA_EMBED_MODEL' >> /startup.sh && \\
    echo 'wait' >> /startup.sh && \\
    chmod +x /startup.sh

# Expose port
EXPOSE 11434

# Use custom startup script
CMD ["/startup.sh"]
EOF

print_success "Created Dockerfile"

# Build custom Ollama image
print_status "Building custom Ollama image..."

IMAGE_NAME="autolog-ollama"
IMAGE_TAG="latest"
FULL_IMAGE_NAME="$REGISTRY_URL/$IMAGE_NAME:$IMAGE_TAG"

cd "$TEMP_DIR"
docker build -t "$FULL_IMAGE_NAME" .

if [ $? -ne 0 ]; then
    print_error "Failed to build Ollama image"
    exit 1
fi

print_success "Built Ollama image: $FULL_IMAGE_NAME"

# Login to container registry
print_status "Logging into container registry..."

echo "$REGISTRY_PASSWORD" | docker login "$REGISTRY_URL" -u "$REGISTRY_USERNAME" --password-stdin

if [ $? -ne 0 ]; then
    print_error "Failed to login to container registry"
    exit 1
fi

print_success "Logged into container registry"

# Push image to registry
print_status "Pushing Ollama image to registry..."

docker push "$FULL_IMAGE_NAME"

if [ $? -ne 0 ]; then
    print_error "Failed to push Ollama image"
    exit 1
fi

print_success "Pushed Ollama image to registry"

# Clean up temporary directory
cd ..
rm -rf "$TEMP_DIR"
print_status "Cleaned up temporary directory"

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
cat > "ollama.tf" << EOF
# Ollama Container App - Custom Image
resource "azurerm_container_app" "ollama" {
  name                         = "autolog-${ENVIRONMENT}-ollama"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"

  identity {
    type = "SystemAssigned"
  }
  
  template {
    container {
      name   = "ollama"
      image  = "$FULL_IMAGE_NAME"
      cpu    = var.ollama_cpu
      memory = var.ollama_memory

      env {
        name  = "OLLAMA_HOST"
        value = "0.0.0.0"
      }
      
      env {
        name  = "OLLAMA_ORIGINS"
        value = "*"
      }
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

# Role assignment for ACR access
resource "azurerm_role_assignment" "acr_pull_ollama" {
  scope                = azurerm_container_registry.main.id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_container_app.ollama.identity[0].principal_id
  depends_on           = [azurerm_container_app.ollama]
}
EOF

# Apply the Ollama configuration
terraform apply -auto-approve -target=azurerm_container_app.ollama -target=azurerm_role_assignment.acr_pull_ollama

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
echo "" 