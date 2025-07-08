#!/bin/bash

# AutoLog Docker Hub Setup Script
# This script helps you set up Docker Hub credentials for cost-optimized deployment

set -e

echo "🐳 AutoLog Docker Hub Setup"
echo "=========================="
echo ""
echo "This script will help you set up Docker Hub credentials for free container registry usage."
echo "This saves $5-10/month compared to Azure Container Registry."
echo ""

# Check if Docker Hub credentials are already set
if [ -n "$DOCKER_HUB_USERNAME" ] && [ -n "$DOCKER_HUB_TOKEN" ]; then
    echo "✅ Docker Hub credentials already configured:"
    echo "   Username: $DOCKER_HUB_USERNAME"
    echo "   Token: ${DOCKER_HUB_TOKEN:0:8}..."
    echo ""
    echo "You can proceed with deployment using:"
    echo "   terraform apply -var='use_azure_registry=false'"
    exit 0
fi

echo "📋 Prerequisites:"
echo "1. Docker Hub account (create at https://hub.docker.com/signup)"
echo "2. Docker Hub access token (create at https://hub.docker.com/settings/security)"
echo ""

# Prompt for Docker Hub username
read -p "Enter your Docker Hub username: " DOCKER_HUB_USERNAME

if [ -z "$DOCKER_HUB_USERNAME" ]; then
    echo "❌ Username cannot be empty"
    exit 1
fi

# Prompt for Docker Hub token
read -s -p "Enter your Docker Hub access token: " DOCKER_HUB_TOKEN
echo ""

if [ -z "$DOCKER_HUB_TOKEN" ]; then
    echo "❌ Token cannot be empty"
    exit 1
fi

# Test Docker Hub credentials
echo ""
echo "🔐 Testing Docker Hub credentials..."

# Try to login to Docker Hub
if echo "$DOCKER_HUB_TOKEN" | docker login -u "$DOCKER_HUB_USERNAME" --password-stdin > /dev/null 2>&1; then
    echo "✅ Docker Hub credentials are valid!"
else
    echo "❌ Failed to authenticate with Docker Hub"
    echo "Please check your username and token"
    exit 1
fi

# Create .env file for local development
echo ""
echo "📝 Creating .env file for local development..."
cat > .env << EOF
# Docker Hub Configuration
DOCKER_HUB_USERNAME=$DOCKER_HUB_USERNAME
DOCKER_HUB_TOKEN=$DOCKER_HUB_TOKEN

# Terraform Configuration
TF_VAR_use_azure_registry=false
TF_VAR_container_registry_url=docker.io/$DOCKER_HUB_USERNAME
EOF

echo "✅ Created .env file with Docker Hub credentials"

# Create terraform.tfvars if it doesn't exist
if [ ! -f "terraform/terraform.tfvars" ]; then
    echo ""
    echo "📝 Creating terraform.tfvars with cost-optimized settings..."
    cat > terraform/terraform.tfvars << EOF
# AutoLog Terraform Variables - Cost Optimized Configuration
environment = "test"
location = "eastus"
resource_group_name = "autolog-rg"

# Use Docker Hub instead of Azure Container Registry (FREE)
use_azure_registry = false
container_registry_url = "docker.io/$DOCKER_HUB_USERNAME"

# Set these values before deploying
db_password = "your-secure-db-password"
jwt_secret = "your-jwt-secret-key"
EOF
    echo "✅ Created terraform/terraform.tfvars"
    echo "⚠️  Please edit terraform/terraform.tfvars to set db_password and jwt_secret"
fi

echo ""
echo "🎉 Docker Hub setup complete!"
echo ""
echo "📋 Next steps:"
echo "1. Edit terraform/terraform.tfvars to set secure passwords"
echo "2. Deploy with cost optimizations:"
echo "   cd terraform"
echo "   terraform init"
echo "   terraform plan"
echo "   terraform apply"
echo ""
echo "💰 Cost savings:"
echo "   - Container Registry: FREE (Docker Hub) vs $5-10/month (Azure ACR)"
echo "   - Total test environment: $40-75/month vs $165-330/month"
echo "   - Savings: 60-70% compared to production configuration"
echo ""
echo "🔐 For GitHub Actions, add these secrets:"
echo "   DOCKER_HUB_USERNAME: $DOCKER_HUB_USERNAME"
echo "   DOCKER_HUB_TOKEN: $DOCKER_HUB_TOKEN"
echo ""
echo "📚 For more information, see COST_OPTIMIZATION.md" 