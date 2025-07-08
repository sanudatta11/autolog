#!/bin/bash

set -e

# CONFIGURATION
tfvars_file="terraform/terraform.tfvars"
if [ ! -f "$tfvars_file" ]; then
  echo "❌ $tfvars_file not found! Please create and configure it."
  exit 1
fi

# Check Azure CLI login
if ! az account show > /dev/null 2>&1; then
  echo "❌ You are not logged into Azure CLI. Run 'az login' and try again."
  exit 1
fi

# Check Docker login
if ! docker info > /dev/null 2>&1; then
  echo "❌ Docker is not running or you are not logged in. Start Docker and try again."
  exit 1
fi

# Check if Azure Container Registry is enabled
use_acr=$(grep '^use_azure_registry' $tfvars_file | awk -F'"' '{print $2}')
if [ "$use_acr" != "true" ]; then
  echo "❌ use_azure_registry must be set to 'true' in $tfvars_file"
  exit 1
fi

enable_key_vault=$(grep '^enable_key_vault' $tfvars_file | awk -F'=' '{print $2}' | xargs)
if [ "$enable_key_vault" != "false" ]; then
  echo "⚠️  Warning: enable_key_vault is not set to false. If you see Key Vault permission errors, set enable_key_vault = false in $tfvars_file."
fi

# If Key Vault is disabled, check for required secrets in environment
if [ "$enable_key_vault" = "false" ]; then
  if [ -z "$DB_PASSWORD" ] || [ -z "$JWT_SECRET" ]; then
    echo "⚠️  DB_PASSWORD and/or JWT_SECRET environment variables are not set."
    echo "    These are required for deployment when Key Vault is disabled."
    echo "    Set them with: export DB_PASSWORD=...; export JWT_SECRET=..."
    exit 1
  fi
fi

# Get Azure Container Registry details from Terraform output
echo "🔍 Getting Azure Container Registry details..."
cd terraform
ACR_LOGIN_SERVER=$(terraform output -raw container_registry_url 2>/dev/null || echo "")
cd ..

if [ -z "$ACR_LOGIN_SERVER" ]; then
  echo "❌ Could not get ACR login server from Terraform output"
  echo "   Make sure to run 'terraform apply' first to create the ACR"
  exit 1
fi

ACR_REPO="$ACR_LOGIN_SERVER"
BACKEND_IMAGE="autolog-backend:latest"
LOGPARSER_IMAGE="autolog-logparser:latest"
FRONTEND_IMAGE="autolog-frontend:latest" # Only if you use a frontend container

# 1. Login to Azure Container Registry
echo "🔐 Logging into Azure Container Registry..."
az acr login --name $(echo $ACR_LOGIN_SERVER | cut -d'.' -f1)

# Function to check if an image exists in ACR
acr_image_exists() {
  local image_name=$1
  local tag=$2
  az acr repository show-manifests --name $(echo $ACR_LOGIN_SERVER | cut -d'.' -f1) --repository $image_name --query "[?tags[?@=='$tag']]" | grep "$tag" > /dev/null 2>&1
}

# 2. Build and push backend if missing
if ! acr_image_exists "autolog-backend" "latest"; then
  if [ -d ./backend ]; then
    echo "🔨 Building backend image..."
    docker build -t $ACR_REPO/$BACKEND_IMAGE ./backend
    echo "📤 Pushing backend image to ACR..."
    docker push $ACR_REPO/$BACKEND_IMAGE
  else
    echo "⚠️  ./backend directory not found, skipping backend image."
  fi
else
  echo "✅ Backend image already exists in ACR."
fi

# 3. Build and push logparser if missing
if ! acr_image_exists "autolog-logparser" "latest"; then
  if [ -d ./logparser_service ]; then
    echo "🔨 Building logparser image..."
    docker build -t $ACR_REPO/$LOGPARSER_IMAGE ./logparser_service
    echo "📤 Pushing logparser image to ACR..."
    docker push $ACR_REPO/$LOGPARSER_IMAGE
  else
    echo "⚠️  ./logparser_service directory not found, skipping logparser image."
  fi
else
  echo "✅ Logparser image already exists in ACR."
fi

# 4. Frontend is deployed via Azure Static Web Apps (not Docker)
echo "ℹ️  Frontend will be deployed via Azure Static Web Apps from GitHub repo"
echo "   - Repository: https://github.com/sanudatta11/autolog"
echo "   - Branch: main"
echo "   - App location: /frontend"
echo "   - Output location: dist"
echo "   - See FRONTEND_SETUP.md for connection instructions after deployment"

# 5. Terraform deploy
cd terraform

echo "🚀 Running terraform init..."
terraform init

echo "📝 Running terraform plan..."
terraform plan -out=tfplan

echo "🚀 Running terraform apply..."
terraform apply -auto-approve tfplan

echo "✅ Deployment complete!"
echo "\n--- Deployment Outputs ---"
terraform output

# 6. Ollama model setup (after Terraform deployment)
cd ..
echo ""
echo "🤖 Setting up Ollama models..."
if [ -f ./manage-ollama-models.sh ]; then
  # Get Ollama URL from Terraform output
  OLLAMA_URL=$(cd terraform && terraform output -raw ollama_url)
  echo "🔗 Ollama URL: $OLLAMA_URL"
  
  # Update the manage-ollama-models.sh script with the correct URL
  sed -i "s|OLLAMA_URL=.*|OLLAMA_URL=\"$OLLAMA_URL\"|" ./manage-ollama-models.sh
  
  echo "⏳ Waiting for Ollama container to be ready..."
  bash ./manage-ollama-models.sh wait
  
  echo "📥 Pulling required models..."
  bash ./manage-ollama-models.sh setup
else
  echo "⚠️  manage-ollama-models.sh not found, skipping Ollama model setup."
fi

echo ""
echo "🎉 Complete deployment finished!"
echo "📋 Next steps:"
echo "   1. Follow FRONTEND_SETUP.md to connect your GitHub repo"
echo "   2. Test your application at the URLs above"
echo "   3. Use 'bash manage-ollama-models.sh status' to check Ollama" 