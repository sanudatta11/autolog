# AutoLog Terraform Variables - High-End Deployment Configuration

# Environment Configuration
variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "prod"
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, staging, prod."
  }
}

variable "location" {
  description = "Azure region"
  type        = string
  default     = "eastus"
}

variable "resource_group_name" {
  description = "Resource group name"
  type        = string
  default     = "autolog-rg"
}

# Database Configuration
variable "db_password" {
  description = "PostgreSQL database password"
  type        = string
  sensitive   = true
}

variable "db_sku_name" {
  description = "Database SKU (B_Standard_B1ms, GP_Standard_D2s_v3, GP_Standard_D4s_v3, etc.)"
  type        = string
  default     = "GP_Standard_D2s_v3"
}

variable "db_storage_mb" {
  description = "Database storage in MB"
  type        = number
  default     = 32768
}

variable "db_version" {
  description = "PostgreSQL version"
  type        = string
  default     = "15"
}

# Security Configuration
variable "jwt_secret" {
  description = "JWT secret key"
  type        = string
  sensitive   = true
}

variable "enable_key_vault" {
  description = "Enable Azure Key Vault for secret management"
  type        = bool
  default     = true
}

variable "enable_monitoring" {
  description = "Enable monitoring (Log Analytics, Application Insights)"
  type        = bool
  default     = true
}

# Container Registry Configuration
variable "use_azure_registry" {
  description = "Whether to use Azure Container Registry (false = use Docker Hub)"
  type        = bool
  default     = true
}

variable "docker_hub_username" {
  description = "Docker Hub username for image repository"
  type        = string
  default     = "autolog"
}

variable "container_registry_url" {
  description = "Container registry URL (e.g., docker.io/username or acr.azurecr.io)"
  type        = string
  default     = "docker.io/autolog"
}

# Container App Configuration
variable "backend_cpu" {
  description = "Backend container CPU allocation"
  type        = number
  default     = 1.0
}

variable "backend_memory" {
  description = "Backend container memory allocation"
  type        = string
  default     = "2Gi"
}

variable "logparser_cpu" {
  description = "Log Parser container CPU allocation"
  type        = number
  default     = 1.0
}

variable "logparser_memory" {
  description = "Log Parser container memory allocation"
  type        = string
  default     = "2Gi"
}

variable "ollama_cpu" {
  description = "Ollama container CPU allocation"
  type        = number
  default     = 2.0
}

variable "ollama_memory" {
  description = "Ollama container memory allocation"
  type        = string
  default     = "4Gi"
}

# Application Configuration
variable "log_level" {
  description = "Application log level"
  type        = string
  default     = "info"
  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "Log level must be one of: debug, info, warn, error."
  }
}

variable "ollama_model" {
  description = "Primary Ollama model to use"
  type        = string
  default     = "llama2:13b"
}

variable "ollama_embed_model" {
  description = "Ollama embedding model to use"
  type        = string
  default     = "nomic-embed-text:latest"
}

# Port Configuration
variable "backend_port" {
  description = "Backend service port"
  type        = number
  default     = 8080
}

variable "logparser_port" {
  description = "Log Parser service port"
  type        = number
  default     = 5000
}

variable "ollama_port" {
  description = "Ollama service port"
  type        = number
  default     = 11434
}

# Scaling Configuration
variable "enable_auto_scaling" {
  description = "Enable auto-scaling for container apps"
  type        = bool
  default     = true
}

variable "min_replicas" {
  description = "Minimum number of replicas"
  type        = number
  default     = 1
}

variable "max_replicas" {
  description = "Maximum number of replicas"
  type        = number
  default     = 10
}

# Storage Configuration
variable "ollama_storage_gb" {
  description = "Ollama models storage in GB"
  type        = number
  default     = 100
}

# Network Configuration
variable "enable_private_networking" {
  description = "Enable private networking for container apps"
  type        = bool
  default     = false
}

# Cost Optimization
variable "use_spot_instances" {
  description = "Use spot instances for cost optimization (not recommended for production)"
  type        = bool
  default     = false
}

# Backup Configuration
variable "enable_backup" {
  description = "Enable database backup"
  type        = bool
  default     = true
}

variable "backup_retention_days" {
  description = "Database backup retention in days"
  type        = number
  default     = 30
} 