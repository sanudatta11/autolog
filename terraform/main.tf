terraform {
  required_version = ">= 1.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
  }
  
  # Using local backend for test environment (simpler, no Azure storage required)
  # Uncomment the azurerm backend below if you want to use Azure storage for state
  # backend "azurerm" {
  #   resource_group_name  = "terraform-state-rg"
  #   storage_account_name = "autologtfstate"
  #   container_name       = "tfstate"
  #   key                  = "autolog-${var.environment}.tfstate"
  # }
}

provider "azurerm" {
  features {
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
    key_vault {
      purge_soft_delete_on_destroy = true
    }
  }
}

# Variables are defined in variables.tf

# Local values
locals {
  name_prefix = "autolog-${var.environment}"
  tags = {
    Environment = var.environment
    Project     = "AutoLog"
    ManagedBy   = "Terraform"
    CostCenter  = "Engineering"
    Version     = "1.0.0"
  }
  
  # Environment-specific configurations
  is_production = var.environment == "prod"
  is_staging    = var.environment == "staging"
  is_dev        = var.environment == "dev"
  
  # Resource sizing based on environment
  db_sku = local.is_production ? "GP_Standard_D4s_v3" : local.is_staging ? "GP_Standard_D2s_v3" : "B_Standard_B1ms"
  db_storage = local.is_production ? 131072 : local.is_staging ? 65536 : 32768  # 128GB, 64GB, 32GB
  
  # Container sizing based on environment
  backend_cpu = local.is_production ? 2.0 : local.is_staging ? 1.0 : 0.5
  backend_memory = local.is_production ? "4Gi" : local.is_staging ? "2Gi" : "1Gi"
  
  logparser_cpu = local.is_production ? 2.0 : local.is_staging ? 1.0 : 0.5
  logparser_memory = local.is_production ? "4Gi" : local.is_staging ? "2Gi" : "1Gi"
  
  ollama_cpu = local.is_production ? 4.0 : local.is_staging ? 2.0 : 1.0
  ollama_memory = local.is_production ? "8Gi" : local.is_staging ? "4Gi" : "2Gi"
  
  # Log level based on environment
  log_level = local.is_production ? "info" : local.is_staging ? "info" : "debug"
  
  # Revision suffix for spot instances (if enabled)
  revision_suffix = var.use_spot_instances ? "spot" : null
}

# Resource Group
resource "azurerm_resource_group" "main" {
  name     = "${var.resource_group_name}-${var.environment}"
  location = var.location
  tags     = local.tags
}

# Container Registry (only when explicitly requested)
resource "azurerm_container_registry" "main" {
  count               = var.use_azure_registry ? 1 : 0
  name                = replace("${local.name_prefix}registry", "-", "")
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = local.is_production ? "Premium" : "Standard"
  admin_enabled       = true
  tags                = local.tags
}

# PostgreSQL Database
resource "azurerm_postgresql_flexible_server" "main" {
  name                   = "${local.name_prefix}-db"
  resource_group_name    = azurerm_resource_group.main.name
  location               = azurerm_resource_group.main.location
  version                = var.db_version
  administrator_login    = "postgres"
  administrator_password = var.db_password
  storage_mb             = local.db_storage
  sku_name               = local.db_sku
  
  # High availability for production only
  dynamic "high_availability" {
    for_each = local.is_production ? [1] : []
    content {
      mode = "ZoneRedundant"
    }
  }
  
  # Backup configuration
  backup_retention_days = var.enable_backup ? var.backup_retention_days : 7
  
  # Zone configuration
  zone = "1"
  
  tags = local.tags
}

# Database
resource "azurerm_postgresql_flexible_server_database" "main" {
  name      = "autolog"
  server_id = azurerm_postgresql_flexible_server.main.id
  collation = "en_US.utf8"
  charset   = "utf8"
}

# Log Analytics Workspace (when monitoring is enabled)
resource "azurerm_log_analytics_workspace" "main" {
  count               = var.enable_monitoring ? 1 : 0
  name                = "${local.name_prefix}-workspace"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                 = "PerGB2018"
  retention_in_days   = local.is_production ? 90 : 30
  tags                = local.tags
}

# Application Insights (when monitoring is enabled)
resource "azurerm_application_insights" "main" {
  count               = var.enable_monitoring ? 1 : 0
  name                = "${local.name_prefix}-appinsights"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  application_type    = "web"
  workspace_id        = var.enable_monitoring ? azurerm_log_analytics_workspace.main[0].id : null
  tags                = local.tags
}

# Key Vault (when enabled)
resource "azurerm_key_vault" "main" {
  count                       = var.enable_key_vault ? 1 : 0
  name                        = "${local.name_prefix}-kv"
  location                    = azurerm_resource_group.main.location
  resource_group_name         = azurerm_resource_group.main.name
  enabled_for_disk_encryption = true
  tenant_id                   = data.azurerm_client_config.current.tenant_id
  soft_delete_retention_days  = local.is_production ? 90 : 7
  purge_protection_enabled    = local.is_production
  sku_name                    = "standard"
  tags                        = local.tags
}

# Key Vault access policy (when enabled)
resource "azurerm_key_vault_access_policy" "main" {
  count        = var.enable_key_vault ? 1 : 0
  key_vault_id = azurerm_key_vault.main[0].id
  tenant_id    = data.azurerm_client_config.current.tenant_id
  object_id    = data.azurerm_client_config.current.object_id
  
  key_permissions = [
    "Get", "List", "Create", "Delete", "Update", "Import", "Backup", "Restore", "Recover"
  ]
  secret_permissions = [
    "Get", "List", "Set", "Delete", "Backup", "Restore", "Recover"
  ]
  certificate_permissions = [
    "Get", "List", "Create", "Delete", "Update", "Import", "Backup", "Restore", "Recover"
  ]
}

# Store secrets in Key Vault (when enabled)
resource "azurerm_key_vault_secret" "db_password" {
  count        = var.enable_key_vault ? 1 : 0
  name         = "database-password"
  value        = var.db_password
  key_vault_id = azurerm_key_vault.main[0].id
}

resource "azurerm_key_vault_secret" "jwt_secret" {
  count        = var.enable_key_vault ? 1 : 0
  name         = "jwt-secret"
  value        = var.jwt_secret
  key_vault_id = azurerm_key_vault.main[0].id
}

# Container Apps Environment
resource "azurerm_container_app_environment" "main" {
  name                       = "${local.name_prefix}-env"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  tags                       = local.tags
  
  # Log Analytics integration (when monitoring is enabled)
  log_analytics_workspace_id = var.enable_monitoring ? azurerm_log_analytics_workspace.main[0].id : null
}

# Container App Environment Storage for Ollama
resource "azurerm_container_app_environment_storage" "ollama_storage" {
  name                         = "ollama-models"
  container_app_environment_id = azurerm_container_app_environment.main.id
  account_name                 = azurerm_storage_account.ollama.name
  access_key                   = azurerm_storage_account.ollama.primary_access_key
  access_mode                  = "ReadWrite"
  share_name                   = azurerm_storage_share.ollama_models.name
}

# Container App - Backend
resource "azurerm_container_app" "backend" {
  name                         = "${local.name_prefix}-backend"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = var.enable_auto_scaling ? "Multiple" : "Single"
  tags                         = local.tags
  
  depends_on = [
    azurerm_container_app_environment.main,
    azurerm_postgresql_flexible_server.main,
    azurerm_postgresql_flexible_server_database.main,
    azurerm_container_registry.main
  ]

  # Container registry configuration
  dynamic "secret" {
    for_each = var.use_azure_registry ? [1] : []
    content {
      name  = "acr-password"
      value = azurerm_container_registry.main[0].admin_password
    }
  }
  
  dynamic "registry" {
    for_each = var.use_azure_registry ? [1] : []
    content {
      server                = azurerm_container_registry.main[0].login_server
      username              = azurerm_container_registry.main[0].admin_username
      password_secret_name  = "acr-password"
    }
  }

  template {
    container {
      name   = "backend"
      image  = var.use_azure_registry ? "${azurerm_container_registry.main[0].login_server}/autolog-backend:latest" : "${var.container_registry_url}/autolog-backend:latest"
      cpu    = local.backend_cpu
      memory = local.backend_memory

      env {
        name  = "DATABASE_URL"
        value = "postgres://postgres:${var.db_password}@${azurerm_postgresql_flexible_server.main.fqdn}:5432/autolog?sslmode=require"
      }
      env {
        name  = "JWT_SECRET"
        value = var.jwt_secret
      }
      env {
        name  = "CORS_ORIGIN"
        value = "https://${local.name_prefix}-frontend.azurestaticapps.net"
      }
      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
      env {
        name  = "LOG_LEVEL"
        value = local.log_level
      }
      env {
        name  = "OLLAMA_URL"
        value = "https://${azurerm_container_app.ollama.latest_revision_fqdn}"
      }
      env {
        name  = "OLLAMA_MODEL"
        value = var.ollama_model
      }
      env {
        name  = "OLLAMA_EMBED_MODEL"
        value = var.ollama_embed_model
      }
    }

  }

  # Auto-scaling and revision management handled by Azure Container Apps

  ingress {
    external_enabled = true
    target_port     = var.backend_port
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
}

# Container App - Log Parser
resource "azurerm_container_app" "logparser" {
  name                         = "${local.name_prefix}-logparser"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = var.enable_auto_scaling ? "Multiple" : "Single"
  tags                         = local.tags
  
  depends_on = [
    azurerm_container_app_environment.main,
    azurerm_postgresql_flexible_server.main,
    azurerm_postgresql_flexible_server_database.main,
    azurerm_container_registry.main
  ]

  # Container registry configuration
  dynamic "secret" {
    for_each = var.use_azure_registry ? [1] : []
    content {
      name  = "acr-password"
      value = azurerm_container_registry.main[0].admin_password
    }
  }
  
  dynamic "registry" {
    for_each = var.use_azure_registry ? [1] : []
    content {
      server                = azurerm_container_registry.main[0].login_server
      username              = azurerm_container_registry.main[0].admin_username
      password_secret_name  = "acr-password"
    }
  }

  template {
    container {
      name   = "logparser"
      image  = var.use_azure_registry ? "${azurerm_container_registry.main[0].login_server}/autolog-logparser:latest" : "${var.container_registry_url}/autolog-logparser:latest"
      cpu    = local.logparser_cpu
      memory = local.logparser_memory

      env {
        name  = "DATABASE_URL"
        value = "postgres://postgres:${var.db_password}@${azurerm_postgresql_flexible_server.main.fqdn}:5432/autolog?sslmode=require"
      }
      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
      env {
        name  = "LOG_LEVEL"
        value = local.log_level
      }
      env {
        name  = "OLLAMA_URL"
        value = "https://${azurerm_container_app.ollama.latest_revision_fqdn}"
      }
      env {
        name  = "OLLAMA_MODEL"
        value = var.ollama_model
      }
      env {
        name  = "OLLAMA_EMBED_MODEL"
        value = var.ollama_embed_model
      }
    }

  }

  # Auto-scaling and revision management handled by Azure Container Apps

  ingress {
    external_enabled = true
    target_port     = var.logparser_port
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
}

# Storage Account for Ollama models (persistent storage)
resource "azurerm_storage_account" "ollama" {
  name                     = "autolog${var.environment}ollama"
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = local.is_production ? "ZRS" : "LRS"
  tags                     = local.tags
}

# File Share for Ollama models
resource "azurerm_storage_share" "ollama_models" {
  name                 = "ollama-models"
  storage_account_name = azurerm_storage_account.ollama.name
  quota                = var.ollama_storage_gb * 1024  # Convert GB to MB
}

# Container App - Ollama
resource "azurerm_container_app" "ollama" {
  name                         = "${local.name_prefix}-ollama"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = var.enable_auto_scaling ? "Multiple" : "Single"
  tags                         = local.tags
  
  depends_on = [
    azurerm_container_app_environment.main,
    azurerm_container_app_environment_storage.ollama_storage
  ]

  template {
    container {
      name   = "ollama"
      image  = "ollama/ollama:latest"
      cpu    = local.ollama_cpu
      memory = local.ollama_memory

      env {
        name  = "OLLAMA_HOST"
        value = "0.0.0.0"
      }
      env {
        name  = "OLLAMA_ORIGINS"
        value = "*"
      }
      env {
        name  = "OLLAMA_MODELS"
        value = "/models"
      }

      # Mount the persistent storage volume
      volume_mounts {
        name = "ollama-models"
        path = "/root/.ollama"
      }
    }

    # Mount persistent storage for models
    volume {
      name         = "ollama-models"
      storage_type = "AzureFile"
      storage_name = azurerm_storage_share.ollama_models.name
    }
  }

  # Auto-scaling and revision management handled by Azure Container Apps

  ingress {
    external_enabled = true
    target_port     = var.ollama_port
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
}

# Static Web App for Frontend
resource "azurerm_static_web_app" "frontend" {
  name                = "${local.name_prefix}-frontend"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  tags                = local.tags
  
  depends_on = [
    azurerm_resource_group.main
  ]
}

# Cost monitoring (commented out for test environments)
# resource "azurerm_monitor_action_group" "cost_alert" {
#   name                = "${local.name_prefix}-cost-alerts"
#   resource_group_name = azurerm_resource_group.main.name
#   short_name          = "cost-alert"
#   tags                = local.tags
#   email_receiver {
#     name                    = "admin"
#     email_address          = "admin@example.com"
#     use_common_alert_schema = true
#   }
# }

# resource "azurerm_monitor_metric_alert" "cost_alert" {
#   name                = "${local.name_prefix}-cost-alert"
#   resource_group_name = azurerm_resource_group.main.name
#   scopes               = [azurerm_resource_group.main.id]
#   tags                 = local.tags
#   criteria {
#     metric_namespace = "Microsoft.Resources/subscriptions/resourceGroups"
#     metric_name      = "Cost"
#     aggregation      = "Total"
#     operator         = "GreaterThan"
#     threshold        = 50
#     dimension {
#       name     = "ResourceGroupName"
#       operator = "Include"
#       values   = [azurerm_resource_group.main.name]
#     }
#   }
#   action {
#     action_group_id = azurerm_monitor_action_group.cost_alert.id
#   }
#   frequency = "PT1H"
#   window_size = "PT1H"
# }

# Data sources
data "azurerm_client_config" "current" {}

# Outputs
output "resource_group_name" {
  description = "Name of the resource group"
  value       = azurerm_resource_group.main.name
}

output "container_registry_url" {
  description = "Container registry URL"
  value       = var.use_azure_registry ? azurerm_container_registry.main[0].login_server : var.container_registry_url
}

output "database_fqdn" {
  description = "Database FQDN"
  value       = azurerm_postgresql_flexible_server.main.fqdn
}

output "frontend_url" {
  description = "Frontend URL"
  value       = azurerm_static_web_app.frontend.default_host_name
}

output "backend_url" {
  description = "Backend URL"
  value       = azurerm_container_app.backend.latest_revision_fqdn
}

output "logparser_url" {
  description = "Log Parser URL"
  value       = azurerm_container_app.logparser.latest_revision_fqdn
}

output "ollama_url" {
  description = "Ollama URL"
  value       = azurerm_container_app.ollama.latest_revision_fqdn
}

output "key_vault_name" {
  description = "Key Vault name"
  value       = var.enable_key_vault ? azurerm_key_vault.main[0].name : "Key Vault not enabled"
}

output "application_insights_key" {
  description = "Application Insights instrumentation key"
  value       = var.enable_monitoring ? azurerm_application_insights.main[0].instrumentation_key : "Monitoring not enabled"
  sensitive   = true
}

output "estimated_monthly_cost" {
  description = "Estimated monthly cost breakdown"
  value = {
    database = local.is_production ? "~$200-400 (Production tier)" : local.is_staging ? "~$100-200 (Staging tier)" : "~$15-25 (Basic tier)"
    container_apps = var.use_spot_instances ? "~$20-40 (Spot instances)" : local.is_production ? "~$200-500 (Production instances)" : local.is_staging ? "~$100-250 (Staging instances)" : "~$40-80 (Standard instances)"
    static_web_app = "~$5-10"
    container_registry = var.use_azure_registry ? (local.is_production ? "~$20-50 (Premium ACR)" : "~$5-10 (Standard ACR)") : "Free (Docker Hub)"
    storage_account = "~$5-20 (for Ollama models)"
    monitoring = var.enable_monitoring ? "~$10-30" : "Free (disabled)"
    key_vault = var.enable_key_vault ? "~$5-10" : "Free (not enabled)"
    total = local.is_production ? "~$440-1010/month" : local.is_staging ? "~$220-510/month" : "~$65-125/month"
  }
} 