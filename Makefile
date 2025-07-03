# IncidentSage Makefile
# Essential commands for development and deployment

.PHONY: help dev docker-dev docker-clean logs shell health status clean setup

# Default target
help: ## Show this help message
	@echo "🚀 IncidentSage - Essential Commands"
	@echo "===================================="
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development Commands
dev: ## Start development environment with Docker
	@echo "🐳 Starting Docker development environment..."
	@docker-compose up

docker-dev: ## Start development environment with Docker in background
	@echo "🐳 Starting Docker development environment in background..."
	@docker-compose up -d

docker-clean: ## Clean up Docker containers, images, and volumes
	@echo "🧹 Cleaning up Docker resources..."
	@docker-compose down -v --rmi all
	@docker system prune -f

# Logs and Monitoring
logs: ## Show Docker logs
	@docker-compose logs -f

shell: ## Access backend container shell
	@docker-compose exec backend sh

status: ## Show service status
	@echo "📊 Service Status:"
	@docker-compose ps

health: ## Check application health
	@echo "🏥 Health Check:"
	@echo "🔍 Checking backend health..."
	@curl -s -f http://localhost:8080/health | jq . 2>/dev/null || echo "Backend: ❌ (Not responding or unhealthy)"
	@echo "🔍 Checking frontend availability..."
	@curl -s -f http://localhost:5173 > /dev/null && echo "Frontend: ✅" || echo "Frontend: ❌ (Not responding)"
	@echo "🔍 Checking database connectivity..."
	@docker-compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1 && echo "Database: ✅" || echo "Database: ❌ (Not responding)"

# Utility Commands
clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf frontend/dist
	@rm -rf backend/bin
	@rm -rf node_modules
	@rm -rf frontend/node_modules

setup: ## Initial project setup
	@echo "🚀 Setting up IncidentSage project..."
	@chmod +x setup.sh
	@./setup.sh

# Development Tools
adminer: ## Open Adminer in browser
	@echo "🔧 Opening Adminer..."
	@open http://localhost:8081 || xdg-open http://localhost:8081 || echo "Please open http://localhost:8081 in your browser"

urls: ## Show all service URLs
	@echo "🌐 Frontend: http://localhost:5173"
	@echo "🔧 Backend: http://localhost:8080"
	@echo "🗄️ Adminer: http://localhost:8081" 