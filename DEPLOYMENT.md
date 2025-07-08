# AutoLog Azure Deployment Guide

## 🚀 Quick Start (Test Environment Only)

Choose your deployment method:

### Option 1: Terraform (Recommended - 60-70% Cost Savings)
```bash
# Use the automated deployment script (easiest)
cd terraform
./deploy.sh deploy

# Or deploy manually:
cd terraform

# Copy and configure variables for test environment
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars for maximum cost savings:
# environment = "test"  # Fixed to test environment
# use_azure_registry = false  # Use Docker Hub (FREE)
# container_registry_url = "docker.io/your-username"

# Deploy with cost optimizations
terraform init
terraform plan
terraform apply
```

### Option 2: GitHub Actions (CI/CD - Automated Cost Optimization)
1. Set up GitHub secrets (see below)
2. Configure Docker Hub credentials for free registry
3. Push to main branch or use manual workflow trigger
4. Automatic cost-optimized deployment

**Estimated Cost**: $40-75/month for test environment

## 📋 Prerequisites

### Required Tools
- **Azure CLI**: [Install Guide](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
- **Docker**: [Install Guide](https://docs.docker.com/get-docker/)
- **Terraform** (for Option 1): [Install Guide](https://developer.hashicorp.com/terraform/downloads)

### Azure Requirements
- Azure subscription with billing enabled
- Contributor access to create resources
- ~$40-75/month budget for test environment

### Docker Hub Requirements (for free registry)
- Docker Hub account: [Create Account](https://hub.docker.com/signup)
- Docker Hub access token: [Create Token](https://hub.docker.com/settings/security)
- GitHub secrets configured (see below)

## 🏗️ Architecture Overview

AutoLog deploys as a multi-service application:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │    Backend      │    │  Log Parser     │
│   (React)       │◄──►│   (Go API)      │◄──►│   (Python)      │
│   Port 80       │    │   Port 8080     │    │   Port 8000     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │   PostgreSQL    │
                       │   Database      │
                       └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │     Ollama      │
                       │   (Local LLM)   │
                       │   Port 11434    │
                       └─────────────────┘
```

## 💰 Cost Breakdown

### Monthly Estimates (Cost Optimized)

#### Test Environment ($40-75/month) - 60-70% Savings
| Component | Service | Cost | Optimization |
|-----------|---------|------|--------------|
| Frontend | Azure Static Web App | $5-10 | Standard pricing |
| Backend | Azure Container App (Spot) | $10-20 | 60-90% cost reduction |
| Log Parser | Azure Container App (Spot) | $5-10 | 60-90% cost reduction |
| Ollama | Azure Container App (Spot) | $10-20 | 60-90% cost reduction |
| PostgreSQL | Azure Database (Basic) | $15-25 | Basic tier vs Standard |
| Container Registry | Docker Hub | **FREE** | Free alternative to ACR |
| Monitoring | Disabled | **FREE** | No Log Analytics/App Insights |
| Key Vault | Not Created | **FREE** | Removed for test environments |
| **Total** | | **$40-75** | **60-70% savings** |

#### Production Environment ($167-335/month)
| Component | Service | Cost |
|-----------|---------|------|
| Frontend | Azure Static Web App | $5-10 |
| Backend | Azure Container App | $30-60 |
| Log Parser | Azure Container App | $20-40 |
| Ollama | Azure Container App | $30-60 |
| PostgreSQL | Azure Database (Standard) | $50-100 |
| Container Registry | Azure Container Registry | $5-10 |
| Monitoring | Log Analytics + App Insights | $10-20 |
| Key Vault | Azure Key Vault | $2-5 |
| **Total** | | **$165-330** |

### Cost Optimization Features
- ✅ **Free Container Registry**: Docker Hub instead of Azure ACR ($5-10/month saved)
- ✅ **Spot Instances**: 60-90% cost reduction on container apps
- ✅ **Basic Database Tier**: $15-25/month vs $50-100/month
- ✅ **Disabled Monitoring**: No Log Analytics/App Insights costs for test
- ✅ **Right-sized Resources**: 50% CPU/memory reduction for test
- ✅ **Automatic Optimization**: Environment-based cost configuration

## 🔧 Deployment Options

### Option 1: Terraform Infrastructure as Code (Recommended)

**Best for**: All environments (test, staging, production)

**Pros**:
- ✅ Infrastructure as code with version control
- ✅ Reproducible deployments
- ✅ Cost tracking and tagging
- ✅ Auto-scaling with Container Apps
- ✅ Spot pricing for cost optimization
- ✅ Built-in security with Key Vault

**Cons**:
- ❌ Requires Terraform knowledge
- ❌ More complex initial setup

**Deployment**:
```bash
# Navigate to terraform directory
cd terraform

# Configure variables
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values

# Deploy
./deploy.sh deploy

# Or step by step:
terraform init
terraform plan
terraform apply
```

### Option 2: GitHub Actions CI/CD

**Best for**: Automated deployments, team collaboration

**Pros**:
- ✅ Automated deployments
- ✅ Built-in testing
- ✅ Environment management
- ✅ Rollback capabilities
- ✅ Terraform state management

**Cons**:
- ❌ Requires GitHub setup
- ❌ More complex configuration

**Setup**:
1. Fork/clone the repository
2. Set up GitHub secrets:
   - `AZURE_CREDENTIALS`: Service principal credentials
   - `DB_PASSWORD`: Database password
   - `JWT_SECRET`: JWT secret key
   - `AZURE_SUBSCRIPTION_ID`: Azure subscription ID
   - `DOCKER_HUB_USERNAME`: Your Docker Hub username
   - `DOCKER_HUB_TOKEN`: Your Docker Hub access token

3. Push to main branch or trigger manual workflow

## 🔐 Security Configuration

### Required Secrets
- **Database Password**: Strong PostgreSQL password
- **JWT Secret**: Secure random string for JWT tokens
- **Azure Credentials**: Service principal for deployment

### Security Best Practices
1. **Use Azure Key Vault** for secret management
2. **Enable SSL/TLS** for all connections
3. **Configure network security groups**
4. **Use managed identities** where possible
5. **Enable Azure Security Center**

### Environment Variables
```bash
# Backend Configuration
DATABASE_URL=postgres://postgres:password@server:5432/autolog?sslmode=require
JWT_SECRET=your-secure-jwt-secret
CORS_ORIGIN=https://your-frontend-url
ENV=production
```

## 🚀 Step-by-Step Deployment

### 1. Azure Login and Setup

```bash
# Login to Azure
az login

# Set subscription (if you have multiple)
az account set --subscription "your-subscription-id"

# Create service principal for Terraform
az ad sp create-for-rbac --name "autolog-terraform" --role contributor \
    --scopes /subscriptions/your-subscription-id \
    --sdk-auth
```

### 2. Configure Terraform Variables

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:
```hcl
# Environment Configuration (Test Only)
environment = "test"
location = "westus2"  # Use westus2 for better availability
resource_group_name = "autolog-rg"

# Secrets (set these via environment variables or Azure Key Vault)
db_password = "your-secure-password"
jwt_secret = "your-jwt-secret-key"

# Container Registry Configuration
use_azure_registry = false  # Use Docker Hub (FREE)

# Docker Hub Configuration
docker_hub_username = "your-dockerhub-username"
container_registry_url = "docker.io/your-dockerhub-username"
```

### 3. Register Azure Providers

```bash
# Run the provider registration script
./fix-azure-providers.sh
```

### 4. Deploy Infrastructure

```bash
# Initialize Terraform
terraform init

# Plan deployment
terraform plan

# Apply deployment
terraform apply
```

### 5. Build and Push Docker Images

```bash
# Build images
docker build -t your-dockerhub-username/autolog-backend:latest ../backend
docker build -t your-dockerhub-username/autolog-frontend:latest ../frontend
docker build -t your-dockerhub-username/autolog-logparser:latest ../logparser_service

# Push to Docker Hub
docker push your-dockerhub-username/autolog-backend:latest
docker push your-dockerhub-username/autolog-frontend:latest
docker push your-dockerhub-username/autolog-logparser:latest
```

### 6. Deploy Applications

```bash
# Deploy all services
./deploy.sh deploy
```

## 🔧 Troubleshooting

### Common Issues

#### 1. Provider Registration Errors
```
MissingSubscriptionRegistration: The subscription is not registered to use namespace 'Microsoft.App'
```

**Solution**: Run the provider registration script:
```bash
./fix-azure-providers.sh
```

#### 2. Location Restrictions
```
LocationIsOfferRestricted: Subscriptions are restricted from provisioning in location 'eastus'
```

**Solution**: Use a different region like `westus2`, `centralus`, or `eastus2`:
```hcl
location = "westus2"
```

#### 3. Resource Type Not Available
```
LocationNotAvailableForResourceType: The provided location is not available for resource type
```

**Solution**: Check available regions for the resource type and use a supported region.

#### 4. Docker Hub Authentication
```
Error: unauthorized: authentication required
```

**Solution**: Login to Docker Hub:
```bash
docker login
```

### Debug Commands

```bash
# Check Azure provider registration
az provider show --namespace Microsoft.App
az provider show --namespace Microsoft.Web
az provider show --namespace Microsoft.DBforPostgreSQL

# Check Terraform state
terraform show
terraform state list

# Check Azure resources
az resource list --resource-group autolog-rg-test

# Check container app logs
az containerapp logs show --name autolog-test-backend --resource-group autolog-rg-test
```

## 📊 Monitoring and Maintenance

### Health Checks
- **Frontend**: `https://your-frontend-url.azurestaticapps.net`
- **Backend**: `https://your-backend-url.azurecontainerapps.io/health`
- **Log Parser**: `https://your-logparser-url.azurecontainerapps.io/health`

### Logs and Debugging
```bash
# View container app logs
az containerapp logs show --name autolog-test-backend --resource-group autolog-rg-test

# View database logs
az postgres flexible-server logs list --resource-group autolog-rg-test --server-name autolog-test-db
```

### Scaling
```bash
# Scale container apps
az containerapp revision set-mode --name autolog-test-backend --resource-group autolog-rg-test --mode single

# Scale database (if needed)
az postgres flexible-server update --resource-group autolog-rg-test --name autolog-test-db --sku-name Standard_B2s
```

## 🧹 Cleanup

### Destroy Infrastructure
```bash
# Destroy all resources
terraform destroy

# Or use the cleanup script
./deploy.sh destroy
```

### Manual Cleanup
```bash
# Delete resource group
az group delete --name autolog-rg-test --yes

# Delete container registry (if using Azure ACR)
az acr delete --name autolog-testregistry --resource-group autolog-rg-test
```

## 📚 Additional Resources

- [Azure Container Apps Documentation](https://docs.microsoft.com/en-us/azure/container-apps/)
- [Azure Static Web Apps Documentation](https://docs.microsoft.com/en-us/azure/static-web-apps/)
- [Azure PostgreSQL Documentation](https://docs.microsoft.com/en-us/azure/postgresql/)
- [Terraform Azure Provider Documentation](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs)
- [Docker Hub Documentation](https://docs.docker.com/docker-hub/)

## 🆘 Support

For deployment issues:
1. Check the troubleshooting section above
2. Review Azure resource logs
3. Check Terraform state and plan output
4. Open an issue in the GitHub repository with detailed error information 