#!/bin/bash

# AutoLog Ollama Setup Script
# This script helps set up Ollama for local LLM functionality

echo "🚀 Setting up Ollama for AutoLog..."
echo "========================================"

# Check if Ollama is already installed
if command -v ollama &> /dev/null; then
    echo "✅ Ollama is already installed"
else
    echo "📥 Installing Ollama..."
    
    # Detect OS and install Ollama
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        curl -fsSL https://ollama.ai/install.sh | sh
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        if command -v brew &> /dev/null; then
            brew install ollama
        else
            echo "❌ Homebrew not found. Please install Homebrew first: https://brew.sh"
            exit 1
        fi
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]]; then
        # Windows
        echo "❌ Ollama is not yet available for Windows. Please use WSL2 or Docker."
        echo "   For Docker: docker run -d -v ollama:/root/.ollama -p 11434:11434 --name ollama ollama/ollama"
        exit 1
    else
        echo "❌ Unsupported operating system: $OSTYPE"
        exit 1
    fi
fi

# Start Ollama service
echo "🔧 Starting Ollama service..."
ollama serve &
OLLAMA_PID=$!

# Wait for Ollama to start
echo "⏳ Waiting for Ollama to start..."
sleep 5

# Check if Ollama is running
if curl -s http://localhost:11434/api/tags > /dev/null; then
    echo "✅ Ollama is running"
else
    echo "❌ Failed to start Ollama"
    exit 1
fi

# Download the default model
echo "📥 Downloading Llama2 model (this may take a while)..."
ollama pull llama2

# Test the model
echo "🧪 Testing the model..."
if ollama run llama2 "Hello, this is a test" > /dev/null 2>&1; then
    echo "✅ Model test successful"
else
    echo "❌ Model test failed"
    exit 1
fi

echo ""
echo "🎉 Ollama setup completed successfully!"
echo ""
echo "📋 Next steps:"
echo "1. Start AutoLog: make dev"
echo "2. Upload a log file in the web interface"
echo "3. Click 'Analyze' to get AI-powered log analysis and RCA"
echo ""
echo "🔧 Configuration:"
echo "- Ollama URL: http://localhost:11434"
echo "- Default Model: llama2"
echo "- You can change the model in backend/env.example"
echo ""
echo "💡 Tips:"
echo "- The first analysis may take longer as the model loads"
echo "- You can use other models like 'mistral' or 'codellama'"
echo "- For better performance, consider using a GPU-enabled model"
echo ""

# Clean up
if [ ! -z "$OLLAMA_PID" ]; then
    kill $OLLAMA_PID 2>/dev/null
fi 