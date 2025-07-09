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
