# AutoLog Ollama Setup Script for Windows PowerShell
# This script helps set up Ollama for local LLM functionality

Write-Host "Setting up Ollama for AutoLog..."
Write-Host "========================================"

# Check if Ollama is already installed
$ollamaInstalled = $false
try {
    $ollamaVersion = ollama --version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Ollama is already installed: $ollamaVersion"
        $ollamaInstalled = $true
    }
} catch {
    Write-Host "Ollama not found, will install via Docker..."
}

if (-not $ollamaInstalled) {
    # For Windows, we'll use Docker since Ollama doesn't have a native Windows installer
    Write-Host "Using Docker to run Ollama..."
    # Check if Docker is running
    $dockerRunning = $true
    try {
        docker ps > $null 2>&1
        if ($LASTEXITCODE -ne 0) {
            $dockerRunning = $false
        }
    } catch {
        $dockerRunning = $false
    }
    if (-not $dockerRunning) {
        Write-Host "Docker is not running. Please start Docker Desktop first."
        exit 1
    }
    # Check if Ollama container is already running
    $ollamaContainer = docker ps --filter "name=ollama" --format "{{.Names}}" 2>$null
    if ($ollamaContainer -eq "ollama") {
        Write-Host "Ollama container is already running"
    } else {
        # Stop any existing Ollama container
        docker stop ollama 2>$null
        docker rm ollama 2>$null
        # Start Ollama container
        Write-Host "Starting Ollama container..."
        docker run -d --name ollama -v ollama:/root/.ollama -p 11434:11434 ollama/ollama
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Failed to start Ollama container"
            exit 1
        }
    }
}

# Wait for Ollama to start
Write-Host "Waiting for Ollama to start..."
Start-Sleep -Seconds 10

# Check if Ollama is running
$ollamaRunning = $true
try {
    $response = Invoke-RestMethod -Uri "http://localhost:11434/api/tags" -Method Get -TimeoutSec 5
    Write-Host "Ollama is running"
} catch {
    $ollamaRunning = $false
}
if (-not $ollamaRunning) {
    Write-Host "Failed to connect to Ollama"
    Write-Host "Try running: docker logs ollama"
    exit 1
}

# Download the default model
Write-Host "Downloading Llama2 model (this may take a while)..."
if ($ollamaInstalled) {
    ollama pull llama2
} else {
    docker exec ollama ollama pull llama2
}
if ($LASTEXITCODE -eq 0) {
    Write-Host "Llama2 model downloaded successfully"
} else {
    Write-Host "Failed to download Llama2 model"
    exit 1
}

# Test the model
Write-Host "Testing the model..."
if ($ollamaInstalled) {
    $testResult = ollama run llama2 "Hello, this is a test" 2>$null
} else {
    $testResult = docker exec ollama ollama run llama2 "Hello, this is a test" 2>$null
}
if ($LASTEXITCODE -eq 0) {
    Write-Host "Model test successful"
} else {
    Write-Host "Model test failed"
    exit 1
}

Write-Host ""
Write-Host "Ollama setup completed successfully!"
Write-Host ""
Write-Host "Next steps:"
Write-Host "1. Start AutoLog: make dev"
Write-Host "2. Upload a log file in the web interface"
Write-Host "3. Click 'Analyze' to get AI-powered incident analysis"
Write-Host ""
Write-Host "Configuration:"
Write-Host "- Ollama URL: http://localhost:11434"
Write-Host "- Default Model: llama2"
if (-not $ollamaInstalled) {
    Write-Host "- Container Name: ollama"
}
Write-Host ""
Write-Host "Tips:"
Write-Host "- The first analysis may take longer as the model loads"
Write-Host "- You can use other models like 'mistral' or 'codellama'"
Write-Host "- For better performance, consider using a GPU-enabled model"
if (-not $ollamaInstalled) {
    Write-Host "- To stop Ollama: docker stop ollama"
    Write-Host "- To start Ollama: docker start ollama"
}
Write-Host ""

# Start everything again
docker-compose up -d