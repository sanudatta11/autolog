# AutoLog Setup Script for Windows PowerShell
# This script sets up the AutoLog project with all dependencies

Write-Host "🚀 Setting up AutoLog Project" -ForegroundColor Green
Write-Host "==================================" -ForegroundColor Green

# Check if Go is installed
try {
    $goVersion = go version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Go is installed: $goVersion" -ForegroundColor Green
    } else {
        throw "Go not found"
    }
} catch {
    Write-Host "❌ Go is not installed. Please install Go 1.21+ first." -ForegroundColor Red
    Write-Host "Download from: https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Check if Node.js is installed
try {
    $nodeVersion = node --version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Node.js is installed: $nodeVersion" -ForegroundColor Green
    } else {
        throw "Node.js not found"
    }
} catch {
    Write-Host "❌ Node.js is not installed. Please install Node.js 18+ first." -ForegroundColor Red
    Write-Host "Download from: https://nodejs.org/" -ForegroundColor Yellow
    exit 1
}

# Check if Docker is installed
try {
    $dockerVersion = docker --version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Docker is installed: $dockerVersion" -ForegroundColor Green
    } else {
        throw "Docker not found"
    }
} catch {
    Write-Host "❌ Docker is not installed. Please install Docker Desktop first." -ForegroundColor Red
    Write-Host "Download from: https://www.docker.com/products/docker-desktop/" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Prerequisites check completed" -ForegroundColor Green

# Install frontend dependencies
Write-Host "📦 Installing frontend dependencies..." -ForegroundColor Blue
Set-Location frontend
npm install
Set-Location ..

# Install shared dependencies
Write-Host "📦 Installing shared dependencies..." -ForegroundColor Blue
Set-Location shared
npm install
Set-Location ..

# Install Go dependencies
Write-Host "📦 Installing Go dependencies..." -ForegroundColor Blue
Set-Location backend
go mod tidy
Set-Location ..

# Create environment files
Write-Host "🔧 Creating environment files..." -ForegroundColor Blue

# Backend .env
if (-not (Test-Path "backend\.env")) {
    @"
# Database Configuration
DATABASE_URL=postgresql://postgres:password@localhost:5432/autolog

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production
JWT_EXPIRY=24h

# Server Configuration
PORT=8080
ENV=development

# CORS Configuration
CORS_ORIGIN=http://localhost:5173

# File Upload Configuration
MAX_FILE_SIZE=10485760
UPLOAD_DIR=./uploads
"@ | Out-File -FilePath "backend\.env" -Encoding UTF8
    Write-Host "✅ Created backend\.env" -ForegroundColor Green
} else {
    Write-Host "⚠️  backend\.env already exists" -ForegroundColor Yellow
}

# Frontend .env
if (-not (Test-Path "frontend\.env")) {
    @"
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
"@ | Out-File -FilePath "frontend\.env" -Encoding UTF8
    Write-Host "✅ Created frontend\.env" -ForegroundColor Green
} else {
    Write-Host "⚠️  frontend\.env already exists" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "🎉 Setup completed!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "1. Update the DATABASE_URL in backend\.env with your PostgreSQL credentials" -ForegroundColor White
Write-Host "2. Start the development servers: make dev" -ForegroundColor White
Write-Host ""
Write-Host "The application will be available at:" -ForegroundColor Cyan
Write-Host "  Frontend: http://localhost:5173" -ForegroundColor White
Write-Host "  Backend:  http://localhost:8080" -ForegroundColor White
Write-Host ""
Write-Host "Happy coding! 🚀" -ForegroundColor Green 