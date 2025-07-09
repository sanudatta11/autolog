# Temporary configuration to update logparser image
resource "azurerm_container_app" "logparser" {
  name                         = "autolog-prod-logparser"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"

  identity {
    type = "SystemAssigned"
  }
  template {
    container {
      name   = "logparser"
      image  = "autologdevregistry.azurecr.io/autolog-logparser:prod"
      cpu    = 1.0
      memory = "2Gi"

      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
      env {
        name  = "PORT"
        value = "5000"
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
