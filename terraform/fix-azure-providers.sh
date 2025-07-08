#!/bin/bash

# AutoLog Azure Provider Registration Script
# This script registers required Azure providers for Container Apps and Static Web Apps

set -e

echo "🔧 Registering Azure providers for AutoLog deployment..."

# Get current subscription
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
echo "📋 Using subscription: $SUBSCRIPTION_ID"

# Register Microsoft.App provider (Container Apps)
echo "📦 Registering Microsoft.App provider for Container Apps..."
az provider register --namespace Microsoft.App --wait

# Register Microsoft.Web provider (Static Web Apps)
echo "🌐 Registering Microsoft.Web provider for Static Web Apps..."
az provider register --namespace Microsoft.Web --wait

# Register Microsoft.DBforPostgreSQL provider (PostgreSQL)
echo "🗄️ Registering Microsoft.DBforPostgreSQL provider for PostgreSQL..."
az provider register --namespace Microsoft.DBforPostgreSQL --wait

# Check registration status
echo "✅ Checking provider registration status..."
az provider show --namespace Microsoft.App --query registrationState -o tsv
az provider show --namespace Microsoft.Web --query registrationState -o tsv
az provider show --namespace Microsoft.DBforPostgreSQL --query registrationState -o tsv

echo "🎉 Provider registration complete!"
echo ""
echo "Next steps:"
echo "1. Run: terraform init"
echo "2. Run: terraform plan"
echo "3. Run: terraform apply"
echo ""
echo "Note: Provider registration can take up to 10 minutes to complete."
echo "If you get provider errors, wait a few minutes and try again." 