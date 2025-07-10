#!/bin/bash

# Ollama Model Management Script for Azure Container Apps
# This script manages models in the persistent storage

set -e

# Default URL (will be updated by build-and-deploy.sh)
OLLAMA_URL="https://autolog-dev-ollama--944jqms.delightfulsand-3ecb87ce.centralus.azurecontainerapps.io"

echo "🤖 Ollama Model Management for Azure Container Apps"
echo "=================================================="
echo "Ollama URL: $OLLAMA_URL"
echo ""

# Function to check if Ollama is responding
check_ollama() {
    echo "🔍 Checking Ollama availability..."
    if curl -s --max-time 10 "$OLLAMA_URL" > /dev/null; then
        echo "✅ Ollama is responding"
        return 0
    else
        echo "❌ Ollama is not responding at $OLLAMA_URL"
        echo "   This might be normal if the container is still starting up."
        echo "   Wait a few minutes and try again."
        return 1
    fi
}

# Function to list models
list_models() {
    echo "📋 Current models:"
    response=$(curl -s "$OLLAMA_URL/api/tags")
    if echo "$response" | jq -e '.models' > /dev/null; then
        echo "$response" | jq -r '.models[] | "• \(.name) (\(.size))"'
    else
        echo "No models found"
    fi
}

# Function to check if a model exists
model_exists() {
    local model_name=$1
    response=$(curl -s "$OLLAMA_URL/api/tags")
    if echo "$response" | jq -e --arg name "$model_name" '.models[] | select(.name == $name)' > /dev/null; then
        return 0  # Model exists
    else
        return 1  # Model doesn't exist
    fi
}

# Function to pull a model
pull_model() {
    local model_name=$1
    echo "📥 Checking model: $model_name"
    
    if model_exists "$model_name"; then
        echo "✅ Model $model_name already exists, skipping download"
        return 0
    fi
    
    echo "📥 Pulling model: $model_name"
    response=$(curl -s -X POST "$OLLAMA_URL/api/pull" \
      -H "Content-Type: application/json" \
      -d "{\"name\": \"$model_name\"}")
    
    if echo "$response" | jq -e '.status' > /dev/null; then
        echo "✅ Successfully initiated pull for $model_name"
        echo "⏳ This may take several minutes for large models..."
        return 0
    else
        echo "❌ Failed to pull $model_name"
        echo "Response: $response"
        return 1
    fi
}

# Function to remove a model
remove_model() {
    local model_name=$1
    echo "🗑️ Removing model: $model_name"
    
    response=$(curl -s -X DELETE "$OLLAMA_URL/api/delete" \
      -H "Content-Type: application/json" \
      -d "{\"name\": \"$model_name\"}")
    
    if echo "$response" | jq -e '.status' > /dev/null; then
        echo "✅ Successfully removed $model_name"
        return 0
    else
        echo "❌ Failed to remove $model_name"
        echo "Response: $response"
        return 1
    fi
}

# Function to test a model
test_model() {
    local model_name=$1
    echo "🧪 Testing model: $model_name"
    
    response=$(curl -s -X POST "$OLLAMA_URL/api/generate" \
      -H "Content-Type: application/json" \
      -d "{
        \"model\": \"$model_name\",
        \"prompt\": \"Hello, this is a test.\",
        \"stream\": false
      }")
    
    if echo "$response" | jq -e '.response' > /dev/null; then
        echo "✅ $model_name is working"
        echo "📝 Response: $(echo "$response" | jq -r '.response')"
        return 0
    else
        echo "❌ $model_name test failed"
        echo "Response: $response"
        return 1
    fi
}

# Main script logic
case "${1:-help}" in
    "list")
        if check_ollama; then
            list_models
        fi
        ;;
    "pull")
        if [ -z "$2" ]; then
            echo "Usage: $0 pull <model_name>"
            echo "Example: $0 pull llama3:8b"
            exit 1
        fi
        if check_ollama; then
            pull_model "$2"
        fi
        ;;
    "remove")
        if [ -z "$2" ]; then
            echo "Usage: $0 remove <model_name>"
            echo "Example: $0 remove llama3:8b"
            exit 1
        fi
        if check_ollama; then
            remove_model "$2"
        fi
        ;;
    "test")
        if [ -z "$2" ]; then
            echo "Usage: $0 test <model_name>"
            echo "Example: $0 test llama3:8b"
            exit 1
        fi
        if check_ollama; then
            test_model "$2"
        fi
        ;;
    "setup")
        echo "🚀 Setting up required models..."
        if check_ollama; then
            echo ""
            echo "📋 Checking current models..."
            list_models
            echo ""
            echo "📥 Setting up LLaMA model..."
            pull_model "llama3:8b"
            echo ""
            echo "📥 Setting up text embedding model..."
            pull_model "nomic-embed-text:latest"
            echo ""
            echo "📋 Final model list:"
            list_models
            echo ""
            echo "✅ Model setup complete!"
        else
            echo "⚠️  Skipping model setup - Ollama not available"
            echo "   Run this command again in a few minutes:"
            echo "   bash manage-ollama-models.sh setup"
        fi
        ;;
    "wait")
        echo "⏳ Waiting for Ollama to be ready..."
        max_attempts=30
        attempt=1
        
        while [ $attempt -le $max_attempts ]; do
            if curl -s --max-time 5 "$OLLAMA_URL" > /dev/null; then
                echo "✅ Ollama is ready!"
                return 0
            fi
            echo "   Attempt $attempt/$max_attempts - waiting 10 seconds..."
            sleep 10
            attempt=$((attempt + 1))
        done
        
        echo "❌ Ollama did not become ready within 5 minutes"
        return 1
        ;;
    "status")
        if check_ollama; then
            echo ""
            list_models
            echo ""
            echo "🔗 Ollama API: $OLLAMA_URL"
            echo "📊 Health: $(curl -s "$OLLAMA_URL" | jq -r '.status // "Unknown"')"
        fi
        ;;
    *)
        echo "Usage: $0 {list|pull|remove|test|setup|wait|status}"
        echo ""
        echo "Commands:"
        echo "  list                    - List all installed models"
        echo "  pull <model_name>       - Pull a model (e.g., llama3:8b)"
        echo "  remove <model_name>     - Remove a model"
        echo "  test <model_name>       - Test a model with a simple prompt"
        echo "  setup                   - Pull required models for AutoLog"
        echo "  wait                    - Wait for Ollama to be ready"
        echo "  status                  - Show Ollama status and models"
        echo ""
        echo "Examples:"
        echo "  $0 setup                # Pull required models"
        echo "  $0 list                 # List current models"
        echo "  $0 test llama3:8b      # Test LLaMA model"
        echo "  $0 pull llama3:8b       # Pull default LLaMA model"
        ;;
esac 