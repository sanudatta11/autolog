# Ollama Setup with Persistent Storage

This guide explains how to properly set up Ollama in Azure Container Apps with persistent model storage.

## 🏗️ Architecture

### Persistent Storage Solution
- **Azure File Share**: 50GB storage for Ollama models
- **Volume Mount**: Models stored in `/root/.ollama` directory
- **Persistence**: Models survive container restarts and updates

### Components
- **Storage Account**: `autolog-testollamastorage` (Standard LRS)
- **File Share**: `ollama-models` (50GB quota)
- **Container App**: `autolog-test-ollama` with volume mount

## 🚀 Deployment Steps

### 1. Deploy Infrastructure
```bash
cd terraform
terraform apply
```

This creates:
- ✅ Storage account for model persistence
- ✅ File share with 50GB capacity
- ✅ Ollama Container App with volume mount
- ✅ All networking and security configured

### 2. Verify Ollama is Running
```bash
# Check if Ollama is responding
curl https://autolog-test-ollama--spot.blackglacier-1f47edad.centralus.azurecontainerapps.io
```

### 3. Manage Models
Use the management script to handle models:

```bash
# Check current status
./scripts/manage-ollama-models.sh status

# Pull required models for AutoLog
./scripts/manage-ollama-models.sh setup

# List installed models
./scripts/manage-ollama-models.sh list

# Test a specific model
./scripts/manage-ollama-models.sh test llama2:13b

# Pull additional models
./scripts/manage-ollama-models.sh pull llama2:7b

# Remove a model
./scripts/manage-ollama-models.sh remove llama2:7b
```

## 📦 Required Models for AutoLog

### Core Models
1. **codellama:7b** (4.1 GB)
   - Primary CodeLlama model for log analysis
   - Text generation and interpretation
   - Pattern recognition in logs
   - Code-focused analysis capabilities

2. **nomic-embed-text:latest** (274 MB)
   - Text embedding model
   - Semantic search and clustering
   - Vector similarity operations

### Optional Models
- **llama2:7b** (4.1 GB) - Smaller, faster alternative
- **llama2:13b** (7.4 GB) - Larger, more capable model
- **mistral:7b** (4.1 GB) - High-performance alternative

## 🔧 Configuration

### Environment Variables
```bash
OLLAMA_HOST=0.0.0.0
OLLAMA_ORIGINS=*
OLLAMA_MODELS=/models
```

### Volume Mount
```yaml
volume_mounts:
  - name: ollama-models
    path: /root/.ollama
```

### Storage Configuration
- **Type**: Azure File Share
- **Size**: 50GB (expandable)
- **Replication**: LRS (Locally Redundant Storage)
- **Performance**: Standard tier

## 💰 Cost Breakdown

### Storage Costs
- **Storage Account**: ~$5-10/month
- **File Share**: Included in storage account
- **Data Transfer**: Minimal (models downloaded once)

### Total Impact
- **Previous**: ~$40-75/month
- **With Storage**: ~$45-85/month
- **Additional Cost**: ~$5-10/month

## 🧪 Testing

### Quick Test
```bash
# Test CodeLlama model
curl -X POST https://autolog-test-ollama--spot.blackglacier-1f47edad.centralus.azurecontainerapps.io/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "codellama:7b",
    "prompt": "Explain log analysis in one sentence.",
    "stream": false
  }'
```

### Embedding Test
```bash
# Test embedding model
curl -X POST https://autolog-test-ollama--spot.blackglacier-1f47edad.centralus.azurecontainerapps.io/api/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text",
    "prompt": "Error: Connection timeout to database server"
  }'
```

## 🔄 Model Persistence

### What Persists
- ✅ Downloaded models
- ✅ Model configurations
- ✅ Model metadata

### What Doesn't Persist
- ❌ Running model instances (restart on container restart)
- ❌ Temporary model files
- ❌ Model cache (regenerated on restart)

### Benefits
- **Faster Startup**: No need to re-download models
- **Cost Savings**: Avoid repeated downloads
- **Reliability**: Models survive container restarts
- **Scalability**: Easy to add/remove models

## 🛠️ Troubleshooting

### Common Issues

1. **Models Not Found After Restart**
   ```bash
   # Check if volume is mounted correctly
   ./scripts/manage-ollama-models.sh status
   ```

2. **Storage Full**
   ```bash
   # Remove unused models
   ./scripts/manage-ollama-models.sh remove <model_name>
   ```

3. **Download Failures**
   ```bash
   # Retry with different model
   ./scripts/manage-ollama-models.sh pull llama2:7b
   ```

### Logs
```bash
# Check Container App logs in Azure Portal
# Or use Azure CLI
az containerapp logs show --name autolog-test-ollama --resource-group autolog-rg-test
```

## 📈 Scaling Considerations

### Storage Scaling
- **Current**: 50GB file share
- **Expandable**: Up to 100TB per storage account
- **Cost**: ~$0.06/GB/month

### Performance Scaling
- **Current**: 1 CPU, 2GB RAM
- **Scaling**: Auto-scaling based on demand
- **Spot Instances**: Cost optimization

## 🎯 Best Practices

1. **Model Management**
   - Use `./manage-ollama-models.sh` for all operations
   - Keep only required models
   - Monitor storage usage

2. **Performance**
   - Use smaller models for testing
   - Load models on-demand
   - Monitor response times

3. **Cost Optimization**
   - Use spot instances
   - Monitor storage usage
   - Remove unused models

## 🔗 Integration with AutoLog

### Backend Configuration
```bash
OLLAMA_MODEL=codellama:7b
OLLAMA_EMBED_MODEL=nomic-embed-text:latest
```

### Log Parser Integration
- Models available for ML-enhanced parsing
- Semantic search capabilities
- Intelligent log classification

This setup provides a production-ready Ollama deployment with persistent model storage, ensuring your AutoLog platform has reliable access to AI models for intelligent log analysis. 