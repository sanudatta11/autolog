#!/bin/bash

# Suppress warnings and errors for cleaner output
exec 2>/dev/null

echo "🎉 AutoLog Azure Deployment Complete!"
echo ""
echo "📋 Service URLs:"
echo "   Frontend: https://$(terraform output -raw frontend_url 2>/dev/null || echo 'N/A')"
echo "   Backend: https://$(terraform output -raw backend_url 2>/dev/null || echo 'N/A')"
echo "   Log Parser: https://$(terraform output -raw logparser_url 2>/dev/null || echo 'N/A')"
echo "   Ollama: https://$(terraform output -raw ollama_url 2>/dev/null || echo 'N/A')"
echo ""
echo "🗄️  Infrastructure:"
echo "   Resource Group: $(terraform output -raw resource_group_name 2>/dev/null || echo 'N/A')"
echo "   Database: $(terraform output -raw database_fqdn 2>/dev/null || echo 'N/A')"
echo "   Container Registry: $(terraform output -raw container_registry_url 2>/dev/null || echo 'N/A')"
echo ""
echo "💰 Estimated Monthly Cost: ~$45-85"