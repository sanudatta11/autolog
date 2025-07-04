# AutoLog Makefile
# Essential commands for development and deployment

.PHONY: dev docker-dev docker-clean logs shell health status clean setup setup-full setup-ollama ollama-status ollama-pull test-ai rebuild-backend rebuild-frontend rebuild-all restart-all dev-local stop-services run-backend run-frontend build-and-up down clean-db


	@echo "🚀 AutoLog - Essential Commands"
	@echo "===================================="
	@echo ""
	@echo "📋 Development Commands:"
	@echo "  dev              Start development environment with Docker"
	@echo "  dev-local        Start services in Docker, run backend/frontend locally"
	@echo "  run-backend      Run backend locally (requires dev-local first)"
	@echo "  run-frontend     Run frontend locally (requires dev-local first)"
	@echo "  stop-services    Stop Docker services (database, ollama, adminer)"
	@echo "  docker-dev       Start development environment with Docker in background"
	@echo "  rebuild-backend  Rebuild backend container"
	@echo "  rebuild-frontend Rebuild frontend container"
	@echo "  rebuild-all      Rebuild all application containers"
	@echo "  restart-all      Restart all services (after code changes)"
	@echo "  docker-clean     Clean up Docker containers, images, and volumes"
	@echo ""
	@echo "🤖 AI/LLM Commands:"
	@echo "  setup-ollama     Setup Ollama for local LLM functionality"
	@echo "  setup-full       Complete setup including Ollama LLM"
	@echo "  ollama-status    Check Ollama service status"
	@echo "  ollama-pull      Pull default LLM model (llama2)"
	@echo "  test-ai          Test AI functionality with sample log"
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
	@echo "  adminer          Open Adminer in browser"
	@echo "  urls             Show all service URLs"
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
	@echo " Starting services in Docker (database, ollama, adminer)..."
	@docker-compose up -d postgres ollama adminer

stop-services: ## Stop Docker services (database, ollama, adminer)
	@echo "🛑 Stopping Docker services..."
	@docker-compose stop postgres ollama adminer
	@echo "✅ Services stopped!"

run-backend: ## Run backend locally (requires dev-local first)
	@echo "🚀 Starting backend locally..."
	@echo "   Make sure you've run 'make dev-local' first!"
ifeq ($(OS),Windows_NT)
	cd backend && powershell -Command "go run cmd/server/main.go"
else
	cd backend && go run cmd/server/main.go
endif

run-frontend: ## Run frontend locally (requires dev-local first)
	@echo "🌐 Starting frontend locally..."
	@echo "   Make sure you've run 'make dev-local' first!"
	@cd frontend && npm run dev

docker-dev: ## Start development environment with Docker in background
	@echo "🐳 Starting Docker development environment in background..."
	@docker-compose up -d

# Rebuild Commands
rebuild-backend: ## Rebuild backend container
	@echo "🔨 Rebuilding backend container..."
	@docker-compose build backend
	@docker-compose up -d backend

rebuild-frontend: ## Rebuild frontend container
	@echo "🔨 Rebuilding frontend container..."
	@docker-compose build frontend
	@docker-compose up -d frontend

rebuild-all: ## Rebuild all application containers
	@echo "🔨 Rebuilding all application containers..."
	@docker-compose build backend frontend
	@docker-compose up -d backend frontend

restart-all: ## Restart all services (useful after code changes)
	@echo "🔄 Restarting all services..."
	@docker-compose restart backend frontend

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
	@curl -s -f http://localhost:3000 > /dev/null && echo "Frontend: ✅" || echo "Frontend: ❌ (Not responding)"
	@echo "🔍 Checking database connectivity..."
	@docker-compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1 && echo "Database: ✅" || echo "Database: ❌ (Not responding)"
	@echo "🔍 Checking Ollama LLM service..."
	@curl -s http://localhost:11434/api/tags > /dev/null 2>&1 && echo "Ollama: ✅" || echo "Ollama: ❌ (Not running - run 'make setup-ollama')"

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

ollama-pull: ## Pull default LLM model (llama2)
	@echo "📥 Pulling Llama2 model for AI analysis..."
	@if command -v ollama > /dev/null; then \
		ollama pull llama2; \
		echo "✅ Llama2 model ready for use"; \
	else \
		echo "❌ Ollama not found. Run 'make setup-ollama' first"; \
	fi

test-ai: ## Test AI functionality with sample log
	@echo "🧪 Testing AI-powered log analysis..."
	@if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then \
		echo "✅ Ollama is running"; \
		echo "📝 Testing with sample log analysis..."; \
		echo '{"timestamp": "2024-01-15T10:30:00Z", "level": "ERROR", "message": "Database connection failed"}' | \
		curl -s -X POST http://localhost:11434/api/generate \
			-H "Content-Type: application/json" \
			-d '{"model": "llama2", "prompt": "Analyze this log entry: {\"timestamp\": \"2024-01-15T10:30:00Z\", \"level\": \"ERROR\", \"message\": \"Database connection failed\"}. Provide a brief analysis in JSON format with severity and summary.", "stream": false}' | \
		jq -r '.response' 2>/dev/null || echo "   Test completed (response may be truncated)"; \
		echo "✅ AI test completed"; \
	else \
		echo "❌ Ollama not running. Run 'make setup-ollama' first"; \
	fi

# Development Tools
adminer: ## Open Adminer in browser
	@echo "🔧 Opening Adminer..."
	@open http://localhost:8081 || xdg-open http://localhost:8081 || echo "Please open http://localhost:8081 in your browser"

urls: ## Show all service URLs
	@echo "🌐 Frontend: http://localhost:3000"
	@echo "🔧 Backend: http://localhost:8080"
	@echo "🗄️ Adminer: http://localhost:8081"
	@echo "🤖 Ollama: http://localhost:11434" 

build-and-up: ## Build all images and start the full stack
	docker-compose build
	docker-compose up -d 

down:
	docker-compose down --volumes --remove-orphans

clean-db:
	docker volume rm postgres_data || true 