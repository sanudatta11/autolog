#!/bin/bash

# Phase 5: Deploy Frontend with SWA
# This script deploys the frontend using Azure Static Web Apps CLI

set -e

echo "🚀 Phase 5: Deploying Frontend with SWA"
echo "========================================"

# Check if Terraform state exists
if [ ! -f "terraform/terraform.tfstate" ]; then
    echo "❌ Error: Terraform state file not found. Run previous phases first."
    exit 1
fi

# Get backend URL from Terraform outputs
cd terraform
BACKEND_URL=$(terraform output -raw backend_url 2>/dev/null || echo "")
cd ..

if [ -z "$BACKEND_URL" ]; then
    echo "❌ Error: Backend URL not found in Terraform outputs. Run Phase 3 first."
    exit 1
fi

echo "🔗 Backend URL: $BACKEND_URL"

# Build the frontend
echo "🔨 Building frontend..."
cd frontend

# Check if node_modules exists, if not install dependencies
if [ ! -d "node_modules" ]; then
    echo "📦 Installing frontend dependencies..."
    npm install
fi

# Build the frontend
echo "🏗️  Building frontend with Vite..."
npm run build

# Create SWA configuration with backend URL
echo "⚙️  Creating SWA configuration..."
cat > swa-cli.config.json << EOF
{
  "routes": [
    {
      "route": "/api/*",
      "rewrite": "$BACKEND_URL/api/\$1"
    }
  ],
  "navigationFallback": {
    "rewrite": "/index.html"
  },
  "platformErrorOverrides": [
    {
      "errorType": "NotFound",
      "serve": "/index.html"
    }
  ]
}
EOF

# Deploy to SWA
echo "🚀 Deploying to Azure Static Web Apps..."
swa deploy ./dist --env production

cd ..

echo "✅ Phase 5 Complete: Frontend deployed successfully!"
echo ""
echo "🌐 Your application is now live!"
echo "   Frontend: https://your-app-name.azurestaticapps.net"
echo "   Backend: $BACKEND_URL"
echo ""
echo "🎉 All phases complete! AutoLog is deployed and ready to use." 