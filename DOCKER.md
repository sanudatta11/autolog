# Docker Setup for IncidentSage

This guide covers how to run IncidentSage using Docker for both development and production environments.

## 🚀 Quick Start (Development)

### Prerequisites
- Docker and Docker Compose installed
- Git

### 1. Clone and Setup
```bash
git clone <repository-url>
cd incident-sage
```

### 2. Start Development Environment
```bash
# Start all services with live reload
docker-compose up

# Or run in background
docker-compose up -d
```

### 3. Access the Application
- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080
- **Database Admin**: http://localhost:8081 (Adminer)
- **Database**: localhost:5432

## 🔧 Development Features

### Live Reload
- **Frontend**: Vite provides instant hot reload for React changes
- **Backend**: Normal Go build (rebuild container for changes)
- **Database**: PostgreSQL with persistent volume

### Development Tools
- **Adminer**: Web-based database management at http://localhost:8081
- **Volume Mounts**: Code changes are immediately reflected in containers
- **Hot Reload**: No need to restart containers for code changes

### Useful Commands
```bash
# View logs
docker-compose logs -f

# View specific service logs
docker-compose logs -f backend
docker-compose logs -f frontend

# Restart a service
docker-compose restart backend

# Rebuild backend after code changes
docker-compose build backend
docker-compose up -d backend

# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Rebuild containers
docker-compose build --no-cache

# Access container shell
docker-compose exec backend sh
docker-compose exec frontend sh
```

## 🏭 Production Deployment

### 1. Environment Setup
Create a `.env` file for production:
```bash
# Database
POSTGRES_USER=your_db_user
POSTGRES_PASSWORD=your_secure_password

# JWT
JWT_SECRET=your_super_secure_jwt_secret

# CORS
CORS_ORIGIN=https://yourdomain.com

# Optional
JWT_EXPIRY=24h
MAX_FILE_SIZE=10485760
```

### 2. Deploy
```bash
# Build and start production services
docker-compose -f docker-compose.prod.yml up -d

# Or with custom env file
docker-compose -f docker-compose.prod.yml --env-file .env up -d
```

### 3. Production URLs
- **Frontend**: http://localhost:3000 (or your domain)
- **Backend API**: http://localhost:8080
- **Database**: localhost:5432

## 📁 Docker Files Structure

```
├── docker-compose.yml              # Development setup
├── docker-compose.prod.yml         # Production setup
├── docker-compose.override.yml     # Development overrides
├── .dockerignore                   # Files to exclude from builds
├── backend/
│   ├── Dockerfile.dev             # Development backend
│   └── Dockerfile.prod            # Production backend
└── frontend/
    ├── Dockerfile.dev             # Development frontend
    ├── Dockerfile.prod            # Production frontend
    └── nginx.conf                 # Nginx configuration
```

## 🔍 Service Details

### PostgreSQL Database
- **Image**: postgres:15-alpine
- **Port**: 5432
- **Database**: incident_sage
- **Credentials**: postgres/password (dev) or from .env (prod)
- **Volume**: postgres_data (persistent)

### Go Backend
- **Development**: Normal Go build (rebuild for changes)
- **Production**: Multi-stage build with Alpine
- **Port**: 8080
- **Health Check**: /health endpoint

### React Frontend
- **Development**: Vite dev server with hot reload
- **Production**: Nginx serving built static files
- **Port**: 5173 (dev) / 3000 (prod)
- **API Proxy**: /api/* routes proxied to backend

### Adminer (Development Only)
- **Port**: 8081
- **Purpose**: Database management interface
- **Auto-connect**: Configured to connect to PostgreSQL

## 🛠️ Troubleshooting

### Common Issues

#### 1. Port Already in Use
```bash
# Check what's using the port
lsof -i :8080
lsof -i :5173

# Kill the process or change ports in docker-compose.yml
```

#### 2. Database Connection Issues
```bash
# Check if PostgreSQL is running
docker-compose ps postgres

# Check database logs
docker-compose logs postgres

# Reset database
docker-compose down -v
docker-compose up
```

#### 3. Build Failures
```bash
# Clean build
docker-compose build --no-cache

# Check Dockerfile syntax
docker build -f backend/Dockerfile.dev backend/
```

#### 4. Permission Issues
```bash
# Fix file permissions
sudo chown -R $USER:$USER .

# Or run with proper user mapping
docker-compose run --user $(id -u):$(id -g) backend
```

### Performance Optimization

#### Development
```bash
# Use Docker BuildKit for faster builds
export DOCKER_BUILDKIT=1

# Limit resource usage
docker-compose up --scale backend=1 --scale frontend=1
```

#### Production
```bash
# Use specific resource limits
docker-compose -f docker-compose.prod.yml up -d --scale backend=2
```

## 🔒 Security Considerations

### Development
- Default passwords (change for production)
- Exposed ports for debugging
- Development tools enabled

### Production
- Use strong passwords and secrets
- Configure proper CORS origins
- Enable HTTPS
- Use secrets management
- Regular security updates

## 📊 Monitoring

### Health Checks
```bash
# Check service health
docker-compose ps

# View resource usage
docker stats

# Monitor logs
docker-compose logs -f --tail=100
```

### Database Backup
```bash
# Create backup
docker-compose exec postgres pg_dump -U postgres incident_sage > backup.sql

# Restore backup
docker-compose exec -T postgres psql -U postgres incident_sage < backup.sql
```

## 🚀 Next Steps

1. **Customize Environment**: Update `.env` files for your environment
2. **Add SSL**: Configure HTTPS with Let's Encrypt or your certificate
3. **Set up CI/CD**: Automate builds and deployments
4. **Monitoring**: Add logging and monitoring solutions
5. **Scaling**: Configure load balancing and horizontal scaling

## 📚 Additional Resources

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Air Live Reload](https://github.com/cosmtrek/air)
- [Vite Development Server](https://vitejs.dev/guide/cli.html)
- [Nginx Configuration](https://nginx.org/en/docs/)
- [PostgreSQL Docker](https://hub.docker.com/_/postgres) 