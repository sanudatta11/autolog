# Ollama Production Deployment Guide

This guide explains how to deploy and manage a production-grade Ollama instance for the AutoLog platform.

## 🚀 Quick Start

### 1. Deploy Ollama Production Instance

```bash
# Make the script executable (if not already)
chmod +x deploy-ollama-production.sh

# Run the deployment script (requires sudo)
sudo ./deploy-ollama-production.sh
```

The script will present you with interactive resource configuration options:

```
Choose your resource configuration:

1) 🚀 FULL SYSTEM RESOURCES (Recommended for dedicated servers)
   • Memory: All available RAM (32GB)
   • CPU: All available cores (16)
   • Best for: Maximum performance, dedicated servers

2) ⚡ HIGH PERFORMANCE (16GB RAM, 8 CPU cores)
   • Memory: 16GB
   • CPU: 8 cores
   • Best for: High-performance workloads

3) 🎯 BALANCED (8GB RAM, 4 CPU cores)
   • Memory: 8GB
   • CPU: 4 cores
   • Best for: Standard workloads, shared servers

4) 💡 LIGHTWEIGHT (4GB RAM, 2 CPU cores)
   • Memory: 4GB
   • CPU: 2 cores
   • Best for: Development, testing, limited resources

5) 🔧 CUSTOM CONFIGURATION
   • Choose your own memory and CPU limits
   • Best for: Specific requirements

6) 📊 SHOW SYSTEM DETAILS
   • Display detailed system information
```

### 2. Non-Interactive Deployment (for automation)

```bash
# For CI/CD pipelines or automation
sudo ./deploy-ollama-production.sh --non-interactive
```

This uses the default balanced configuration (8GB RAM, 4 CPU cores).

### 3. Verify Deployment

```bash
# Check status
./manage-ollama-production.sh status

# Test models
./manage-ollama-production.sh test
```

## 📋 Prerequisites

### System Requirements
- **OS**: Linux (Ubuntu 20.04+ recommended)
- **CPU**: 4+ cores (8+ recommended)
- **RAM**: 8GB+ (16GB+ recommended)
- **Storage**: 50GB+ available space
- **Docker**: Installed and running
- **Root Access**: Required for deployment

### Software Requirements
- Docker Engine 20.10+
- curl
- jq
- systemd
- cron

## 🏗️ Architecture

### Production Features
- **Docker Container**: Isolated Ollama instance
- **Systemd Service**: Auto-restart on failure
- **Persistent Storage**: Models survive restarts
- **Health Monitoring**: Automated health checks
- **Log Management**: Structured logging with rotation
- **Resource Limits**: CPU and memory constraints
- **Port 80**: All endpoints exposed on standard HTTP port

### Directory Structure
```
/opt/ollama/                    # Data directory
├── models/                     # Persistent model storage
└── ...

/var/log/ollama/               # Log directory
├── health.log                 # Health check logs
├── monitor.log                # Monitoring logs
└── ...

/etc/systemd/system/           # Systemd service
└── ollama.service

/usr/local/bin/               # Management scripts
├── ollama-health-check.sh    # Health check script
└── ollama-monitor.sh         # Monitoring script
```

## 🔧 Configuration

### Environment Variables
- `OLLAMA_HOST=0.0.0.0` - Bind to all interfaces
- `OLLAMA_ORIGINS=*` - Allow all origins
- `OLLAMA_PORT=80` - Expose on port 80

### Resource Configuration Options

The deployment script offers multiple resource configuration presets:

#### 🚀 Full System Resources
- **Memory**: All available RAM (unlimited)
- **CPU**: All available cores (unlimited)
- **Best for**: Dedicated servers, maximum performance

#### ⚡ High Performance
- **Memory**: 16GB limit
- **CPU**: 8 cores limit
- **Best for**: High-performance workloads

#### 🎯 Balanced (Default)
- **Memory**: 8GB limit
- **CPU**: 4 cores limit
- **Best for**: Standard workloads, shared servers

#### 💡 Lightweight
- **Memory**: 4GB limit
- **CPU**: 2 cores limit
- **Best for**: Development, testing, limited resources

#### 🔧 Custom Configuration
- **Memory**: User-defined (4GB to unlimited)
- **CPU**: User-defined (2 to unlimited cores)
- **Best for**: Specific requirements

### System Requirements
- **File Descriptors**: 65536 limit (all configurations)
- **Storage**: 50GB+ available space
- **Network**: Port 80 exposed

### Models
- **LLM Model**: `codellama:7b` (4.1GB)
- **Embedding Model**: `nomic-embed-text:latest` (274MB)

## 📊 Management Commands

### Status and Health
```bash
# Check overall status
./manage-ollama-production.sh status

# Show system information
./manage-ollama-production.sh system

# Test all models
./manage-ollama-production.sh test
```

### Service Control
```bash
# Start Ollama
./manage-ollama-production.sh start

# Stop Ollama
./manage-ollama-production.sh stop

# Restart Ollama
./manage-ollama-production.sh restart
```

### Logs and Monitoring
```bash
# Follow container logs
./manage-ollama-production.sh logs container

# Follow health check logs
./manage-ollama-production.sh logs health

# Follow monitoring logs
./manage-ollama-production.sh logs monitor

# Follow systemd service logs
./manage-ollama-production.sh logs service

# Show available log types
./manage-ollama-production.sh logs all
```

### Model Management
```bash
# Pull a new model
./manage-ollama-production.sh pull llama2:7b

# Remove a model
./manage-ollama-production.sh remove llama2:7b

# List installed models (via status command)
./manage-ollama-production.sh status
```

### Cleanup
```bash
# Remove Ollama completely (preserves model data)
./manage-ollama-production.sh cleanup
```

## 🔗 API Endpoints

### Base URL
```
http://localhost:80
```

### Available Endpoints
- **Health Check**: `GET /`
- **Generate Text**: `POST /api/generate`
- **Embeddings**: `POST /api/embeddings`
- **List Models**: `GET /api/tags`
- **Pull Model**: `POST /api/pull`
- **Delete Model**: `DELETE /api/delete`

### Example API Calls

#### Test LLM Generation
```bash
curl -X POST http://localhost:80/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2:13b",
    "prompt": "Explain log analysis in one sentence.",
    "stream": false
  }'
```

#### Test Embedding Generation
```bash
curl -X POST http://localhost:80/api/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text:latest",
    "prompt": "Error: Connection timeout to database server"
  }'
```

#### List Installed Models
```bash
curl http://localhost:80/api/tags
```

## 🔍 Monitoring and Logs

### Health Checks
- **Frequency**: Every 5 minutes
- **Action**: Restart container if unhealthy
- **Log File**: `/var/log/ollama/health.log`

### Resource Monitoring
- **Frequency**: Every hour
- **Metrics**: CPU, Memory, Network, Models
- **Log File**: `/var/log/ollama/monitor.log`

### Log Rotation
- **Frequency**: Daily
- **Retention**: 7 days
- **Compression**: Enabled
- **Location**: `/etc/logrotate.d/ollama`

## 🛠️ Troubleshooting

### Common Issues

#### 1. Container Not Starting
```bash
# Check Docker logs
docker logs ollama-production

# Check systemd service
systemctl status ollama

# Check system resources
./manage-ollama-production.sh system

# Check resource configuration
./manage-ollama-production.sh status
```

#### 2. Resource Configuration Issues
```bash
# If you selected too many resources for your system
# Check available resources
./manage-ollama-production.sh system

# Restart with different configuration
sudo ./deploy-ollama-production.sh

# Or use non-interactive mode with default settings
sudo ./deploy-ollama-production.sh --non-interactive
```

#### 3. Models Not Loading
```bash
# Check available disk space
df -h /opt/ollama

# Check model status
./manage-ollama-production.sh status

# Re-pull models if needed
./manage-ollama-production.sh pull llama2:13b
./manage-ollama-production.sh pull nomic-embed-text:latest
```

#### 4. API Not Responding
```bash
# Check if container is running
docker ps | grep ollama-production

# Check health
./manage-ollama-production.sh status

# Restart if needed
./manage-ollama-production.sh restart
```

#### 5. High Resource Usage
```bash
# Check resource usage
./manage-ollama-production.sh status

# Monitor in real-time
docker stats ollama-production

# Consider reducing model size
./manage-ollama-production.sh pull llama2:7b

# Or redeploy with different resource configuration
sudo ./deploy-ollama-production.sh
```

### Performance Optimization

#### Resource Configuration Optimization
- **For maximum performance**: Use "Full System Resources" option
- **For shared servers**: Use "Balanced" or "Lightweight" options
- **For development**: Use "Lightweight" option
- **For production**: Use "High Performance" or "Full System Resources"

#### Memory Optimization
- Use smaller models for testing (`llama2:7b` instead of `llama2:13b`)
- Monitor memory usage with `docker stats`
- Redeploy with different memory configuration if needed

#### CPU Optimization
- Monitor CPU usage with `docker stats`
- Redeploy with different CPU configuration if needed
- Use spot instances for cost optimization

#### Storage Optimization
- Monitor disk usage: `df -h /opt/ollama`
- Remove unused models: `./manage-ollama-production.sh remove <model>`
- Use log rotation to manage log files

## 🔒 Security Considerations

### Network Security
- Ollama is exposed on port 80 (HTTP)
- Consider using HTTPS in production
- Implement firewall rules to restrict access
- Use reverse proxy for additional security

### Container Security
- Container runs with limited privileges
- Resource limits prevent resource exhaustion
- Health checks ensure service availability
- Logs are rotated to prevent disk filling

### Model Security
- Models are stored in persistent volume
- Access to models is controlled by container permissions
- No sensitive data should be stored in models

## 📈 Scaling Considerations

### Vertical Scaling
- **Redeploy with higher resource configuration**: Use interactive menu to select more resources
- **Use "Full System Resources"**: For maximum performance on dedicated servers
- **Use larger models**: For better performance (requires more resources)
- **Monitor and adjust**: Use `./manage-ollama-production.sh status` to monitor resource usage

### Horizontal Scaling
- Deploy multiple Ollama instances
- Use load balancer for distribution
- Implement model caching strategies

### Cost Optimization
- **Use appropriate resource configuration**: Choose "Lightweight" for development, "Balanced" for shared servers
- **Use spot instances for compute**: Deploy on cost-effective cloud instances
- **Monitor resource usage**: Use `./manage-ollama-production.sh status` regularly
- **Remove unused models**: Free up storage space
- **Use smaller models for development**: `llama2:7b` instead of `llama2:13b`

## 🔄 Integration with AutoLog

### Backend Configuration
Update your AutoLog backend configuration:

```bash
# Environment variables
OLLAMA_URL=http://localhost:80
OLLAMA_MODEL=codellama:7b
OLLAMA_EMBED_MODEL=nomic-embed-text:latest
```

### Health Check Integration
The AutoLog backend can use the health endpoint:
```bash
curl http://localhost:80
```

### Model Verification
AutoLog can verify model availability:
```bash
curl http://localhost:80/api/tags
```

## 📝 Maintenance

### Regular Maintenance Tasks

#### Daily
- Check health logs: `tail -f /var/log/ollama/health.log`
- Monitor resource usage: `./manage-ollama-production.sh status`

#### Weekly
- Review monitoring logs: `tail -f /var/log/ollama/monitor.log`
- Check disk usage: `df -h /opt/ollama`
- Update models if needed

#### Monthly
- Review and clean up old logs
- Check for model updates
- Review performance metrics

### Backup Strategy
- Models are stored in `/opt/ollama/models/`
- Backup this directory for model persistence
- Logs are in `/var/log/ollama/`
- Systemd service file is in `/etc/systemd/system/ollama.service`

## 🆘 Support

### Getting Help
1. Check the status: `./manage-ollama-production.sh status`
2. Review logs: `./manage-ollama-production.sh logs all`
3. Check system resources: `./manage-ollama-production.sh system`
4. Restart if needed: `./manage-ollama-production.sh restart`
5. Redeploy with different configuration: `sudo ./deploy-ollama-production.sh`

### Resource Configuration Help
- **Too much resource usage**: Redeploy with "Lightweight" or "Balanced" configuration
- **Not enough performance**: Redeploy with "High Performance" or "Full System Resources"
- **Custom requirements**: Use "Custom Configuration" option during deployment
- **Automation needs**: Use `--non-interactive` flag for CI/CD pipelines

### Useful Commands
```bash
# Full system check
./manage-ollama-production.sh status && ./manage-ollama-production.sh system

# Quick health check
curl -s http://localhost:80 > /dev/null && echo "Healthy" || echo "Unhealthy"

# Resource monitoring
watch -n 5 'docker stats ollama-production --no-stream'

# Log monitoring
tail -f /var/log/ollama/health.log /var/log/ollama/monitor.log

# Redeploy with different configuration
sudo ./deploy-ollama-production.sh

# Non-interactive deployment (for automation)
sudo ./deploy-ollama-production.sh --non-interactive
```

### Resource Configuration Examples
```bash
# For development/testing
sudo ./deploy-ollama-production.sh  # Choose "Lightweight" option

# For production with dedicated server
sudo ./deploy-ollama-production.sh  # Choose "Full System Resources" option

# For shared server
sudo ./deploy-ollama-production.sh  # Choose "Balanced" option

# For CI/CD automation
sudo ./deploy-ollama-production.sh --non-interactive
```

## 🎯 Key Features Summary

### ✅ Interactive Resource Selection
- **6 preset configurations** from lightweight to full system resources
- **Custom configuration** option for specific requirements
- **System information display** to help with decision making
- **Non-interactive mode** for automation and CI/CD

### ✅ Production-Grade Reliability
- **Multiple restart layers**: Systemd service, Docker container, health checks
- **Comprehensive monitoring**: Health checks every 5 minutes, resource monitoring hourly
- **Persistent storage**: Models survive restarts and updates
- **Log management**: Structured logging with rotation

### ✅ Flexible Deployment Options
- **Interactive deployment**: User-friendly menu system
- **Automated deployment**: Non-interactive mode for CI/CD
- **Resource optimization**: Choose configuration based on your needs
- **Easy reconfiguration**: Redeploy anytime with different settings

This production deployment provides a robust, scalable, and maintainable Ollama instance for your AutoLog platform with comprehensive monitoring, management capabilities, and flexible resource configuration options. 