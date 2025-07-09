# AutoLog Phased Deployment Guide

This guide explains the phased deployment system for AutoLog, which uses Terraform targeting to deploy infrastructure in logical phases while preventing failures and ensuring proper resource dependencies.

## Overview

The deployment is split into 4 phases using a unified Terraform configuration:

1. **Phase 1**: Container Registry Infrastructure
2. **Phase 2**: Build and Push Docker Images
3. **Phase 3**: Main Infrastructure (Database, Container Apps)
4. **Phase 4**: Frontend Deployment (SWA)

## Architecture

Instead of separate Terraform files, we use a single `main.tf` configuration with Terraform targeting to deploy resources in phases:

- **Phase 1**: Targets `azurerm_resource_group.main` and `azurerm_container_registry.main`
- **Phase 2**: Builds and pushes Docker images (no Terraform resources)
- **Phase 3**: Targets database and container app resources
- **Phase 4**: Deploys frontend using SWA CLI (no Terraform resources)

## Prerequisites

Before starting the deployment, ensure you have:

- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installed and configured
- [Terraform](https://www.terraform.io/downloads) installed
- [Docker](https://docs.docker.com/get-docker/) installed and running
- [SWA CLI](https://docs.microsoft.com/en-us/azure/static-web-apps/cli/get-started) installed
- Azure subscription with appropriate permissions

## Quick Start

### Full Deployment

To deploy everything in one go:

```bash
./scripts/deploy-phased.sh
```

### Phase-by-Phase Deployment

To run specific phases:

```bash
# Run only Phase 1 (registry)
./scripts/deploy-phased.sh --phase 1

# Run only Phase 2 (build images)
./scripts/deploy-phased.sh --phase 2

# Run only Phase 3 (infrastructure)
./scripts/deploy-phased.sh --phase 3

# Run only Phase 4 (frontend)
./scripts/deploy-phased.sh --phase 4
```

### Skip Specific Phases

To skip certain phases:

```bash
# Skip Phase 2 (if images are already built)
./scripts/deploy-phased.sh --skip-phase 2

# Skip multiple phases
./scripts/deploy-phased.sh --skip-phase 1 --skip-phase 2
```

## Phase Details

### Phase 1: Container Registry Infrastructure

**Purpose**: Creates the Azure Container Registry (if using Azure) or prepares for Docker Hub.

**Terraform Targets**:
- `azurerm_resource_group.main`
- `azurerm_container_registry.main`

**Resources Created**:
- Resource Group
- Azure Container Registry (if `use_azure_registry = true`)

**Configuration**:
- Edit `terraform/terraform.tfvars` to set `use_azure_registry`
- Set `container_registry_url` for Docker Hub username

**Manual Execution**:
```bash
cd terraform
terraform apply -auto-approve -target="azurerm_resource_group.main" -target="azurerm_container_registry.main"
cd ..
```

### Phase 2: Build and Push Docker Images

**Purpose**: Builds and pushes backend and logparser images to the registry.

**Process**:
1. Authenticates with the registry (Azure ACR or Docker Hub)
2. Builds backend image from `backend/` directory
3. Builds logparser image from `logparser_service/` directory
4. Pushes images with environment tags

**Manual Execution**:
```bash
./scripts/phase2-build-images.sh
```

**Docker Hub Setup**:
If using Docker Hub, set environment variables:
```bash
export DOCKER_USERNAME="your-dockerhub-username"
export DOCKER_PASSWORD="your-dockerhub-password"
```

### Phase 3: Main Infrastructure

**Purpose**: Deploys the core infrastructure including database and container apps.

**Terraform Targets**:
- `azurerm_postgresql_flexible_server.main`
- `azurerm_postgresql_flexible_server_database.main`
- `azurerm_container_app_environment.main`
- `azurerm_container_app.backend`
- `azurerm_container_app.logparser`
- `azurerm_container_app.ollama`

**Resources Created**:
- PostgreSQL Flexible Server
- Container Apps Environment
- Backend Container App
- Logparser Container App
- Ollama Container App

**Configuration**:
- Database credentials in `terraform/terraform.tfvars`
- Container app scaling and resource limits
- Environment variables for services

**Manual Execution**:
```bash
cd terraform
terraform apply -auto-approve -target="azurerm_postgresql_flexible_server.main" -target="azurerm_postgresql_flexible_server_database.main" -target="azurerm_container_app_environment.main" -target="azurerm_container_app.backend" -target="azurerm_container_app.logparser" -target="azurerm_container_app.ollama"
cd ..
```

### Phase 4: Frontend Deployment

**Purpose**: Builds and deploys the React frontend using Azure Static Web Apps.

**Process**:
1. Installs frontend dependencies
2. Builds the React app with Vite
3. Creates SWA configuration with API routing
4. Deploys to Azure Static Web Apps

**Manual Execution**:
```bash
./scripts/phase4-deploy-frontend.sh
```

## Configuration

### Terraform Variables

Edit `terraform/terraform.tfvars`:

```hcl
# Environment Configuration
environment = "dev"
location = "centralus"
resource_group_name = "autolog-rg"

# Database Configuration
db_password = "your-secure-password"
jwt_secret = "your-jwt-secret-key"

# Registry Configuration
use_azure_registry = false  # Set to false for Docker Hub
container_registry_url = "docker.io/your-username"
```

### Environment Variables

For Docker Hub authentication:
```bash
export DOCKER_USERNAME="your-dockerhub-username"
export DOCKER_PASSWORD="your-dockerhub-password"
```

## File Structure

```
terraform/
├── main.tf                    # Unified Terraform configuration
├── variables.tf               # Variable definitions
├── terraform.tfvars           # Your configuration
├── terraform.tfvars.example   # Example configuration
└── .terraform/                # Terraform cache

scripts/
├── deploy-phased.sh           # Main deployment orchestrator
├── phase2-build-images.sh     # Phase 2: Build & push images
├── phase4-deploy-frontend.sh  # Phase 4: Deploy frontend
└── setup-dockerhub.sh         # Docker Hub setup helper
```

## Troubleshooting

### Common Issues

#### Phase 1: Terraform Targeting Errors

**Problem**: Terraform targeting fails with dependency errors
**Solution**: Ensure targets are specified in dependency order

```bash
# Run with proper target order
terraform apply -auto-approve -target="azurerm_resource_group.main" -target="azurerm_container_registry.main"
```

#### Phase 2: Docker Build Failures

**Problem**: Docker build fails due to missing dependencies
**Solution**: Ensure all dependencies are properly specified in Dockerfiles

```bash
# Check Dockerfile syntax
docker build --no-cache backend/
docker build --no-cache logparser_service/
```

#### Phase 3: Container App Failures

**Problem**: Container Apps fail to start due to missing images
**Solution**: Ensure Phase 2 completed successfully and images exist

```bash
# Check if images exist in registry
docker pull your-registry/autolog-backend:dev
docker pull your-registry/autolog-logparser:dev
```

#### Phase 4: SWA Deployment Failures

**Problem**: SWA CLI fails to deploy
**Solution**: Check SWA CLI installation and authentication

```bash
# Reinstall SWA CLI
npm install -g @azure/static-web-apps-cli

# Login to Azure
az login
```

### Debugging Commands

```bash
# Check Terraform state
cd terraform
terraform show

# Check specific resources
terraform state list

# Check Azure resources
az resource list --resource-group autolog-rg-dev

# Check Container Apps status
az containerapp show --name autolog-dev-backend --resource-group autolog-rg-dev

# Check SWA deployment
swa deploy ./frontend/dist --env production --dry-run
```

## Cost Optimization

The phased deployment system includes several cost optimizations:

- **Spot Instances**: Container Apps use spot instances (60-90% cost reduction)
- **Basic Database Tier**: Uses B_Standard_B1ms for development
- **Minimal Resources**: Container Apps use minimal CPU (0.5) and memory (1Gi)
- **No Monitoring**: Skips Log Analytics and Application Insights in dev

Estimated monthly cost: $40-75

## Security Considerations

- Database passwords and JWT secrets should be stored in Azure Key Vault in production
- Use managed identities for Container Apps in production
- Enable network security groups and private endpoints
- Implement proper RBAC for Azure resources

## Next Steps

After successful deployment:

1. **Configure Custom Domain**: Set up custom domain for SWA
2. **Set Up Monitoring**: Add Application Insights and Log Analytics
3. **Configure Backups**: Set up database backups
4. **CI/CD Pipeline**: Create GitHub Actions for automated deployments
5. **Security Hardening**: Implement production security measures

## Support

For issues and questions:

1. Check the troubleshooting section above
2. Review Azure Container Apps logs
3. Check Terraform state and outputs
4. Review the main README.md for additional information 