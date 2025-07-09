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

# Update backend container app to use custom image
echo "🔄 Updating backend container app..."
cd terraform

# Create a temporary Terraform configuration to update the backend image
cat > update-backend.tf << EOF
# Temporary configuration to update backend image
resource "azurerm_container_app" "backend" {
  name                         = "autolog-${ENVIRONMENT}-backend"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"

  identity {
    type = "SystemAssigned"
  }
  template {
    container {
      name   = "backend"
      image  = "${ACR_LOGIN_SERVER}/autolog-backend:${ENVIRONMENT}"
      cpu    = 1.0
      memory = "2Gi"

      env {
        name  = "DB_HOST"
        value = azurerm_postgresql_flexible_server.main.fqdn
      }
      env {
        name  = "DB_PORT"
        value = "5432"
      }
      env {
        name  = "DB_NAME"
        value = azurerm_postgresql_flexible_server_database.main.name
      }
      env {
        name  = "DB_USER"
        value = azurerm_postgresql_flexible_server.main.administrator_login
      }
      env {
        name  = "DB_PASSWORD"
        value = azurerm_postgresql_flexible_server.main.administrator_password
      }
      env {
        name  = "JWT_SECRET"
        value = var.jwt_secret
      }
      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
    }
  }

  ingress {
    external_enabled = true
    target_port     = 8080
    transport       = "http"
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
}
EOF

# Apply the backend update
echo "📦 Applying backend image update..."
terraform apply -auto-approve -target=azurerm_container_app.backend

# Create a temporary Terraform configuration to update the logparser image
cat > update-logparser.tf << EOF
# Temporary configuration to update logparser image
resource "azurerm_container_app" "logparser" {
  name                         = "autolog-${ENVIRONMENT}-logparser"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"

  identity {
    type = "SystemAssigned"
  }
  template {
    container {
      name   = "logparser"
      image  = "${ACR_LOGIN_SERVER}/autolog-logparser:${ENVIRONMENT}"
      cpu    = 1.0
      memory = "2Gi"

      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
    }
  }

  ingress {
    external_enabled = true
    target_port     = 5000
    transport       = "http"
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
}
EOF

# Apply the logparser update
echo "�� Applying logparser image update..."
terraform apply -auto-approve -target=azurerm_container_app.logparser

# Clean up temporary files
rm -f update-backend.tf update-logparser.tf

cd ..

echo "✅ Phase 4 Complete: Backend and Logparser updated to use custom images!"
echo ""
echo "🔗 Updated services:"
echo "   Backend: ${ACR_LOGIN_SERVER}/autolog-backend:${ENVIRONMENT}"
echo "   Logparser: ${ACR_LOGIN_SERVER}/autolog-logparser:${ENVIRONMENT}"
echo ""
echo "📋 Next: Run Phase 5 to deploy the frontend" 