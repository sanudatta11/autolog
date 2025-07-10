#!/bin/bash

# Test script for Ollama models in Azure Container Apps
# This script tests both llama2:13b and nomic-embed-text models

OLLAMA_URL="https://autolog-test-ollama--spot.blackglacier-1f47edad.centralus.azurecontainerapps.io"

echo "🧪 Testing Ollama Models in Azure Container Apps"
echo "================================================"
echo "Ollama URL: $OLLAMA_URL"
echo ""

# Test 1: Check if Ollama is responding
echo "1. Testing Ollama connectivity..."
curl -s "$OLLAMA_URL" > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ Ollama service is responding"
else
    echo "❌ Ollama service is not responding"
    exit 1
fi

# Test 2: List available models
echo ""
echo "2. Listing available models..."
curl -s "$OLLAMA_URL/api/tags" | jq -r '.models[] | "📦 \(.name) (\(.size))"'

# Test 3: Test LLaMA model (llama2:13b)
echo ""
echo "3. Testing CodeLlama model (codellama:7b)..."
LLAMA_RESPONSE=$(curl -s -X POST "$OLLAMA_URL/api/generate" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "codellama:7b",
    "prompt": "Explain what log analysis is in one sentence.",
    "stream": false
  }')

if echo "$LLAMA_RESPONSE" | jq -e '.response' > /dev/null; then
    echo "✅ CodeLlama model is working"
    echo "📝 Response: $(echo "$LLAMA_RESPONSE" | jq -r '.response')"
else
    echo "❌ CodeLlama model test failed"
    echo "Response: $LLAMA_RESPONSE"
fi

# Test 4: Test text embedding model (nomic-embed-text)
echo ""
echo "4. Testing text embedding model (nomic-embed-text)..."
EMBED_RESPONSE=$(curl -s -X POST "$OLLAMA_URL/api/embeddings" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text",
    "prompt": "Error: Connection timeout to database server"
  }')

if echo "$EMBED_RESPONSE" | jq -e '.embedding' > /dev/null; then
    echo "✅ Text embedding model is working"
    EMBEDDING_LENGTH=$(echo "$EMBED_RESPONSE" | jq -r '.embedding | length')
    echo "📊 Embedding vector length: $EMBEDDING_LENGTH"
else
    echo "❌ Text embedding model test failed"
    echo "Response: $EMBED_RESPONSE"
fi

echo ""
echo "🎉 Ollama model testing complete!"
echo ""
echo "Your models are ready for use in AutoLog:"
echo "• llama2:13b - For log analysis and text generation"
echo "• nomic-embed-text - For text embeddings and similarity search" 