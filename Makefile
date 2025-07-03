# IncidentSage Makefile
# Provides convenient commands for development, building, testing, and deployment

.PHONY: help install dev build test clean docker-dev docker-clean logs shell backend-shell frontend-shell db-shell lint format migrate seed setup

# Default target
help: ## Show this help message
	@echo "🚀 IncidentSage - Available Commands"
	@echo "====================================="
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development Commands
install: ## Install all dependencies (Node.js and Go)
	@echo "📦 Installing dependencies..."
	@npm install
	@cd frontend && npm install
	@cd shared && npm install
	@cd backend && go mod tidy
	@echo "✅ Dependencies installed successfully"

dev: ## Start development servers (local)
	@echo "🚀 Starting development servers..."
	@npm run dev

docker-dev: ## Start development environment with Docker
	@echo "🐳 Starting Docker development environment..."
	@docker-compose up

docker-dev-detached: ## Start development environment with Docker in background
	@echo "🐳 Starting Docker development environment in background..."
	@docker-compose up -d

# Building Commands
build: ## Build both frontend and backend
	@echo "🔨 Building application..."
	@npm run build

build-frontend: ## Build frontend only
	@echo "🔨 Building frontend..."
	@cd frontend && npm run build

build-backend: ## Build backend only
	@echo "🔨 Building backend..."
	@cd backend && go build -o bin/server cmd/server/main.go

docker-build: ## Build Docker images
	@echo "🐳 Building Docker images..."
	@docker-compose build



# Testing Commands
test: ## Run all tests
	@echo "🧪 Running tests..."
	@npm run test

test-frontend: ## Run frontend tests
	@echo "🧪 Running frontend tests..."
	@cd frontend && npm run test

test-backend: ## Run backend tests
	@echo "🧪 Running backend tests..."
	@cd backend && go test ./...

test-watch: ## Run tests in watch mode
	@echo "🧪 Running tests in watch mode..."
	@cd frontend && npm run test -- --watch

# Database Commands
migrate: ## Run database migrations
	@echo "🗄️ Running database migrations..."
	@cd backend && go run cmd/migrate/main.go 2>/dev/null || echo "⚠️  Migration command not found. Make sure the backend is built and dependencies are installed."

migrate-docker: ## Run database migrations in Docker
	@echo "🗄️ Running database migrations in Docker..."
	@docker-compose exec backend go run cmd/migrate/main.go 2>/dev/null || echo "⚠️  Migration command not found in Docker. Make sure containers are running."

seed: ## Seed database with sample data
	@echo "🌱 Seeding database..."
	@cd backend && go run cmd/seed/main.go 2>/dev/null || echo "⚠️  Seed command not found. Make sure the backend is built and dependencies are installed."

db-reset: ## Reset database (drop and recreate)
	@echo "🔄 Resetting database..."
	@docker-compose down -v
	@docker-compose up -d postgres
	@sleep 5
	@make migrate-docker

# Docker Commands
docker-clean: ## Clean up Docker containers, images, and volumes
	@echo "🧹 Cleaning up Docker resources..."
	@docker-compose down -v --rmi all
	@docker system prune -f

docker-logs: ## Show Docker logs
	@docker-compose logs -f

docker-logs-backend: ## Show backend logs
	@docker-compose logs -f backend

docker-logs-frontend: ## Show frontend logs
	@docker-compose logs -f frontend

docker-logs-db: ## Show database logs
	@docker-compose logs -f postgres

# Shell Access Commands
shell: ## Access backend container shell
	@docker-compose exec backend sh

backend-shell: ## Access backend container shell
	@docker-compose exec backend sh

frontend-shell: ## Access frontend container shell
	@docker-compose exec frontend sh

db-shell: ## Access database shell
	@docker-compose exec postgres psql -U postgres -d incident_sage

# Code Quality Commands
lint: ## Run linting
	@echo "🔍 Running linting..."
	@npm run lint

lint-fix: ## Run linting with auto-fix
	@echo "🔍 Running linting with auto-fix..."
	@cd frontend && npm run lint -- --fix

format: ## Format Go code
	@echo "🎨 Formatting Go code..."
	@cd backend && go fmt ./...

format-check: ## Check Go code formatting
	@echo "🎨 Checking Go code formatting..."
	@cd backend && test -z "$(shell gofmt -l .)"



# Utility Commands
clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf frontend/dist
	@rm -rf backend/bin
	@rm -rf shared/dist
	@rm -rf node_modules
	@rm -rf frontend/node_modules
	@rm -rf shared/node_modules

setup: ## Initial project setup
	@echo "🚀 Setting up IncidentSage project..."
	@chmod +x setup.sh
	@./setup.sh

status: ## Show service status
	@echo "📊 Service Status:"
	@docker-compose ps

health: ## Check application health
	@echo "🏥 Health Check:"
	@curl -f http://localhost:8080/health || echo "Backend: ❌"
	@curl -f http://localhost:5173 || echo "Frontend: ❌"

# Backup and Restore Commands
backup: ## Create database backup
	@echo "💾 Creating database backup..."
	@docker-compose exec postgres pg_dump -U postgres incident_sage > backup_$(shell date +%Y%m%d_%H%M%S).sql

restore: ## Restore database from backup (usage: make restore BACKUP_FILE=backup.sql)
	@echo "📥 Restoring database from $(BACKUP_FILE)..."
	@docker-compose exec -T postgres psql -U postgres incident_sage < $(BACKUP_FILE)

# Development Tools
adminer: ## Open Adminer in browser
	@echo "🔧 Opening Adminer..."
	@open http://localhost:8081 || xdg-open http://localhost:8081 || echo "Please open http://localhost:8081 in your browser"

frontend-url: ## Show frontend URL
	@echo "🌐 Frontend: http://localhost:5173"

backend-url: ## Show backend URL
	@echo "🔧 Backend: http://localhost:8080"

adminer-url: ## Show Adminer URL
	@echo "🗄️ Adminer: http://localhost:8081"

# Quick Development Workflow
quick-start: ## Quick start development environment
	@echo "⚡ Quick starting development environment..."
	@make install
	@make docker-dev-detached
	@sleep 10
	@make health
	@make frontend-url
	@make backend-url
	@make adminer-url

# Environment Management
env-dev: ## Create development environment file
	@echo "🔧 Creating development environment file..."
	@cp backend/env.example backend/.env
	@cp frontend/.env.example frontend/.env 2>/dev/null || echo "Frontend .env.example not found"



# Monitoring and Debugging
monitor: ## Monitor resource usage
	@echo "📊 Monitoring resource usage..."
	@docker stats

logs-all: ## Show all logs
	@echo "📋 All logs:"
	@docker-compose logs

debug: ## Show debug information
	@echo "🐛 Debug Information:"
	@echo "Docker version:"
	@docker --version
	@echo "Docker Compose version:"
	@docker-compose --version
	@echo "Node version:"
	@node --version
	@echo "Go version:"
	@go version
	@echo "Service status:"
	@make status 