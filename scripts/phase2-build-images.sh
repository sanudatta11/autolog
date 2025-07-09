#!/bin/bash

# Phase 2: Build and Push Docker Images (ACR Only)
# This script builds and pushes backend and logparser images to Azure Container Registry (ACR)

set -e

# Ensure script is run from project root
if [ ! -f "terraform/terraform.tfvars" ]; then
    echo "❌ terraform/terraform.tfvars not found! Please run this script from the project root."
    exit 1
fi

cd terraform

# Load ACR credentials from Terraform outputs
REGISTRY_URL=$(terraform output -raw container_registry_login_server 2>/dev/null || true)
REGISTRY_USERNAME=$(terraform output -raw container_registry_username 2>/dev/null || true)
REGISTRY_PASSWORD=$(terraform output -raw container_registry_password 2>/dev/null || true)

# Fail if any ACR output is missing
if [ -z "$REGISTRY_URL" ] || [ -z "$REGISTRY_USERNAME" ] || [ -z "$REGISTRY_PASSWORD" ]; then
    echo "❌ One or more ACR outputs are missing. Ensure Phase 1 (infrastructure) is complete and outputs are available."
    echo "   REGISTRY_URL: $REGISTRY_URL"
    echo "   REGISTRY_USERNAME: $REGISTRY_USERNAME"
    echo "   REGISTRY_PASSWORD: [hidden]"
    exit 1
fi

# Get environment from Terraform
ENVIRONMENT=$(grep '^environment' terraform.tfvars | cut -d'"' -f2)

cd ..

echo "🚀 Phase 2: Building and Pushing Docker Images to Azure Container Registry (ACR)"
echo "================================================"
echo "📦 Using Azure Container Registry: $REGISTRY_URL"

# Login to Azure Container Registry
echo "🔐 Logging into Azure Container Registry..."
echo "$REGISTRY_PASSWORD" | docker login "$REGISTRY_URL" -u "$REGISTRY_USERNAME" --password-stdin

echo "🏗️  Building and pushing images for environment: $ENVIRONMENT"

# Build and push Backend image
echo "🔨 Building Backend image..."
cd backend
docker build -t "$REGISTRY_URL/autolog-backend:$ENVIRONMENT" .
docker build -t "$REGISTRY_URL/autolog-backend:latest" .

echo "📤 Pushing Backend image..."
docker push "$REGISTRY_URL/autolog-backend:$ENVIRONMENT"
docker push "$REGISTRY_URL/autolog-backend:latest"
cd ../terraform

# Build and push Logparser image
echo "🔨 Building Logparser image..."
cd ../logparser_service
docker build -t "$REGISTRY_URL/autolog-logparser:$ENVIRONMENT" .
docker build -t "$REGISTRY_URL/autolog-logparser:latest" .

echo "📤 Pushing Logparser image..."
docker push "$REGISTRY_URL/autolog-logparser:$ENVIRONMENT"
docker push "$REGISTRY_URL/autolog-logparser:latest"
cd ../terraform

echo "✅ Phase 2 Complete: Images built and pushed to Azure Container Registry successfully!"
echo ""
echo "📋 Summary:"
echo "   Backend: $REGISTRY_URL/autolog-backend:$ENVIRONMENT"
echo "   Logparser: $REGISTRY_URL/autolog-logparser:$ENVIRONMENT"
echo ""
echo "🔄 Next: Run Phase 3 to deploy infrastructure" 