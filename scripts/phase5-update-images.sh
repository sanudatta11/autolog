#!/bin/bash

# Phase 4: Update Backend and Logparser to Use Custom Images
# This script updates the container apps to use the custom images built in phase 2

set -e

echo "🚀 Phase 4: Updating Backend and Logparser Images"
echo "=================================================="

# Check if Terraform state exists
if [ ! -f "terraform/terraform.tfstate" ]; then
    echo "❌ Error: Terraform state file not found. Run previous phases first."
    exit 1
fi

# Get ACR details from Terraform outputs
cd terraform
ACR_LOGIN_SERVER=$(terraform output -raw container_registry_login_server 2>/dev/null || echo "")
ENVIRONMENT=$(terraform output -raw environment 2>/dev/null || echo "prod")
cd ..

if [ -z "$ACR_LOGIN_SERVER" ]; then
    echo "❌ Error: ACR login server not found in Terraform outputs. Run Phase 1 first."
    exit 1
fi

echo "🔗 ACR Login Server: $ACR_LOGIN_SERVER"
echo "🌍 Environment: $ENVIRONMENT"

# Clean up any leftover temporary files first
cd terraform
echo "🧹 Cleaning up any leftover temporary files..."
rm -f update-*.tf main.tf.custom main.tf.backup main.tf.original

# Create a simple approach using sed to update the main.tf file
echo " Updating Terraform configuration..."

# Backup the original file
cp main.tf main.tf.backup

# Update backend image and port
echo "🔄 Updating backend configuration..."
sed -i "s|image  = \"nginx:alpine\"|image  = \"${ACR_LOGIN_SERVER}/autolog-backend:${ENVIRONMENT}\"|g" main.tf
sed -i "s|target_port     = 80|target_port     = 8080|g" main.tf

# Update logparser image and port
echo " Updating logparser configuration..."
sed -i "s|image  = \"nginx:alpine\"|image  = \"${ACR_LOGIN_SERVER}/autolog-logparser:${ENVIRONMENT}\"|g" main.tf
sed -i "s|target_port     = 80|target_port     = 5000|g" main.tf

# Update the logparser URL in backend environment variables
echo " Updating logparser URL in backend configuration..."
# Get the current logparser URL (it should be available from the nginx version)
LOGPARSER_URL=$(terraform output -raw logparser_url 2>/dev/null || echo "")
if [ -n "$LOGPARSER_URL" ]; then
    # Replace the placeholder logparser URL with the actual one
    sed -i "s|value = \"http://localhost:5000\"|value = \"${LOGPARSER_URL}\"|g" main.tf
fi

# Apply the changes
echo "📦 Applying image updates..."
terraform apply -auto-approve

# Restore the original configuration
echo " Restoring original Terraform configuration..."
mv main.tf.backup main.tf

cd ..

echo "✅ Phase 4 Complete: Backend and Logparser updated to use custom images!"
echo ""
echo "🔗 Updated services:"
echo "   Backend: ${ACR_LOGIN_SERVER}/autolog-backend:${ENVIRONMENT} (port 8080)"
echo "   Logparser: ${ACR_LOGIN_SERVER}/autolog-logparser:${ENVIRONMENT} (port 5000)"
if [ -n "$LOGPARSER_URL" ]; then
    echo "   Logparser URL: ${LOGPARSER_URL}"
fi
echo ""
echo "📋 Next: Run Phase 5 to deploy the frontend" 