# AutoLog Makefile
# Essential commands for development and deployment

.PHONY: dev dev-local stop-services run-backend run-frontend logs shell health status clean setup setup-full setup-ollama ollama-status ollama-pull migrate urls docker-clean rebuild-logparser

help: ## Show this help message
	@echo "🚀 AutoLog - Essential Commands"
	@echo "===================================="
	@echo ""
	@echo "📋 Development Commands:"
	@echo "  dev              Start development environment with Docker"
	@echo "  dev-local        Start services in Docker (database, ollama, logparser)"
	@echo "  run-backend      Run backend locally (requires dev-local first)"
	@echo "  run-frontend     Run frontend locally (requires dev-local first)"
	@echo "  stop-services    Stop Docker services (database, ollama, logparser)"
	@echo "  rebuild-logparser Rebuild logparser microservice container"
	@echo ""
	@echo "🤖 AI/LLM Commands:"
	@echo "  setup-ollama     Setup Ollama for local LLM functionality"
	@echo "  setup-full       Complete setup including Ollama LLM"
	@echo "  ollama-status    Check Ollama service status"
	@echo "  ollama-pull      Pull default LLM model (llama3:8b)"
	@echo ""
	@echo "📊 Monitoring Commands:"
	@echo "  logs             Show Docker logs"
	@echo "  shell            Access backend container shell"
	@echo "  status           Show service status"
	@echo "  health           Check application health"
	@echo ""
	@echo "🔧 Utility Commands:"
	@echo "  clean            Clean build artifacts"
	@echo "  migrate          Run database migrations"
	@echo "  setup            Initial project setup"
	@echo "  urls             Show all service URLs"
	@echo "  docker-clean     Clean up Docker containers, images, and volumes"
	@echo ""
	@echo "💡 Quick Start:"
	@echo "  1. make setup-full    # Complete setup with AI"
	@echo "  2. make dev           # Start the application"
	@echo "  3. make health        # Check all services"

# Development Commands
dev: ## Start development environment with Docker
	@echo "🐳 Starting Docker development environment..."
	@docker-compose up

dev-local: ## Start services in Docker, run backend/frontend locally
	@echo "🐳 Starting services in Docker (database, ollama, logparser)..."
	@docker-compose up -d postgres ollama logparser-service

stop-services: ## Stop and delete Docker services (database, ollama, logparser)
	@echo "🛑 Stopping and deleting Docker services..."
	@docker-compose down postgres ollama logparser-service
	@echo "✅ Services stopped and deleted!"

run-backend: ## Run backend locally (requires dev-local first)
	@echo "🚀 Starting backend locally..."
	@echo "   Make sure you've run 'make dev-local' first!"
ifeq ($(OS),Windows_NT)
	cd backend && powershell -Command "$$env:LOGPARSER_URL='http://localhost:8000'; go run cmd/server/main.go"
else
	cd backend && LOGPARSER_URL=http://localhost:8000 go run cmd/server/main.go
endif

run-frontend: ## Run frontend locally (requires dev-local first)
	@echo "🌐 Starting frontend locally..."
	@echo "   Make sure you've run 'make dev-local' first!"
	@cd frontend && npm run dev

rebuild-logparser: ## Rebuild logparser microservice container
	@echo "🔨 Rebuilding logparser microservice container..."
	@docker-compose build logparser-service
	@docker-compose up -d logparser-service
	@echo "✅ Logparser microservice rebuilt and restarted!"

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
	@echo "🔍 Checking Ollama LLM service..."
	@curl -s http://localhost:11434/api/tags > /dev/null 2>&1 && echo "Ollama: ✅" || echo "Ollama: ❌ (Not running - run 'make setup-ollama')"
	@echo "🔍 Checking Logparser microservice..."
	@curl -s -f http://localhost:8000/docs > /dev/null 2>&1 && echo "Logparser: ✅" || echo "Logparser: ❌ (Not responding)"

# Utility Commands
clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf frontend/dist
	@rm -rf backend/bin
	@rm -rf node_modules
	@rm -rf frontend/node_modules

migrate: ## Run database migrations
	@echo "🗄️ Running database migrations..."
	@echo "   Make sure you've run 'make dev-local' first!"
ifeq ($(OS),Windows_NT)
	cd backend && powershell -Command "go run cmd/migrate/main.go"
else
	cd backend && go run cmd/migrate/main.go
endif

setup: ## Initial project setup
	@echo "🚀 Setting up AutoLog project..."
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy Bypass -File setup.ps1
else
	- chmod +x setup.sh
	@bash ./setup.sh
endif

setup-full: ## Complete setup including Ollama LLM
	@echo "🚀 Setting up AutoLog with AI capabilities..."
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy Bypass -File setup.ps1
	@echo ""
	@echo "🤖 Setting up Ollama for AI-powered log analysis..."
	@powershell -ExecutionPolicy Bypass -File setup-ollama.ps1
else
	- chmod +x setup.sh
	@bash ./setup.sh
	@echo ""
	@echo "🤖 Setting up Ollama for AI-powered log analysis..."
	- chmod +x setup-ollama.sh
	@bash ./setup-ollama.sh
endif
	@echo ""
	@echo "🎉 Complete setup finished!"
	@echo "💡 Run 'make dev' to start the application"

# Ollama LLM Setup Commands
setup-ollama: ## Setup Ollama for local LLM functionality
	@echo "🤖 Setting up Ollama for AI-powered log analysis..."
	- chmod +x setup-ollama.sh
	@bash ./setup-ollama.sh

ollama-status: ## Check Ollama service status
	@echo "🔍 Checking Ollama status..."
	@if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then \
		echo "✅ Ollama is running"; \
		echo "📋 Available models:"; \
		curl -s http://localhost:11434/api/tags | jq -r '.models[].name' 2>/dev/null || echo "   No models found"; \
	else \
		echo "❌ Ollama is not running"; \
		echo "💡 Run 'make setup-ollama' to install and start Ollama"; \
	fi

ollama-pull: ## Pull default LLM model (llama3:8b)
	@echo "📥 Pulling Llama3:8b model for AI analysis..."
	@if command -v ollama > /dev/null; then \
		ollama pull llama3:8b; \
		echo "✅ Llama3:8b model ready for use"; \
	else \
		echo "❌ Ollama not found. Run 'make setup-ollama' first"; \
	fi

# Development Tools
urls: ## Show all service URLs
	@echo "🌐 Frontend: http://localhost:5173"
	@echo "🔧 Backend: http://localhost:8080"
	@echo "🤖 Ollama: http://localhost:11434"
	@echo "📝 Logparser: http://localhost:8000"

docker-clean: ## Clean up Docker containers, images, and volumes
	@echo "🧹 Cleaning up Docker resources..."
	@docker-compose down -v --rmi all
	@docker system prune -f 

deploy:
	@echo "🚀 Deploying AutoLog to Azure..."
	@cd terraform && \
		./deploy.sh deploy || \
		(echo "❌ Deployment failed. Check the error messages above." && exit 1)