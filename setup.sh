#!/bin/bash

# Fix PATH for WSL to find Go installation
export PATH="$PATH:/mnt/c/Program Files/Go/bin"

echo "🚀 Setting up IncidentSage Project"
echo "=================================="

echo "DEBUG: PATH is: $PATH"
echo "DEBUG: which go: $(which go)"
echo "DEBUG: go version output:"
go version

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ first."
    exit 1
fi

# Check if PostgreSQL is running
if ! pg_isready -q; then
    echo "⚠️  PostgreSQL might not be running. Please ensure PostgreSQL is started."
fi

echo "✅ Prerequisites check completed"

# Install frontend dependencies
echo "📦 Installing frontend dependencies..."
cd frontend
npm install
cd ..

# Install shared dependencies
echo "📦 Installing shared dependencies..."
cd shared
npm install
cd ..

# Install Go dependencies
echo "📦 Installing Go dependencies..."
cd backend
go mod tidy
cd ..

# Create environment files
echo "🔧 Creating environment files..."

# Backend .env
if [ ! -f backend/.env ]; then
    cat > backend/.env << EOF
# Database Configuration
DATABASE_URL=postgresql://postgres:password@localhost:5432/incident_sage

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
EOF
    echo "✅ Created backend/.env"
else
    echo "⚠️  backend/.env already exists"
fi

# Frontend .env
if [ ! -f frontend/.env ]; then
    cat > frontend/.env << EOF
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
EOF
    echo "✅ Created frontend/.env"
else
    echo "⚠️  frontend/.env already exists"
fi

echo ""
echo "🎉 Setup completed!"
echo ""
echo "Next steps:"
echo "1. Update the DATABASE_URL in backend/.env with your PostgreSQL credentials"
echo "2. Create the database: createdb incident_sage"
echo "3. Start the development servers: npm run dev"
echo ""
echo "The application will be available at:"
echo "  Frontend: http://localhost:5173"
echo "  Backend:  http://localhost:8080"
echo ""
echo "Happy coding! 🚀" 