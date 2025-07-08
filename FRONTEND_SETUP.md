# Frontend Setup Guide for Azure Static Web Apps

## Overview
After running `terraform apply`, you need to connect your GitHub repository to the Azure Static Web App for automatic deployment.

## Prerequisites
- ✅ Terraform deployment completed successfully
- ✅ GitHub repository: `https://github.com/sanudatta11/autolog`
- ✅ Frontend code in `/frontend` directory
- ✅ Azure CLI installed and logged in

## Step-by-Step Setup

### 1. Get Your Static Web App URL
After Terraform deployment, get your frontend URL:
```bash
cd terraform
terraform output frontend_url
```

### 2. Connect GitHub Repository

#### Option A: Azure Portal (Recommended)
1. **Go to Azure Portal**
   - Navigate to your Static Web App resource: `autolog-test-frontend`
   - Click on "Deployment Center"

2. **Connect Repository**
   - Click "Configure" or "Connect"
   - Select "GitHub" as source
   - Authorize Azure to access your GitHub account
   - Select repository: `sanudatta11/autolog`
   - Select branch: `main`

3. **Configure Build Settings**
   - **App location**: `/frontend`
   - **Output location**: `dist`
   - **API location**: (leave empty)
   - Click "Save"

#### Option B: Azure CLI
```bash
# Get your Static Web App name
STATIC_WEB_APP_NAME="autolog-test-frontend"
RESOURCE_GROUP="autolog-rg-test"

# Connect to GitHub (this will open browser for auth)
az staticwebapp connect \
  --name $STATIC_WEB_APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --source https://github.com/sanudatta11/autolog \
  --branch main \
  --app-location "/frontend" \
  --output-location "dist"
```

### 3. Verify GitHub Actions Workflow
Azure will automatically create a GitHub Actions workflow in your repo:
- File: `.github/workflows/azure-static-web-apps-<hash>.yml`
- This workflow will build and deploy your frontend on every push to `main`

### 4. Test Deployment
1. **Make a small change** to your frontend code
2. **Push to GitHub**:
   ```bash
   git add .
   git commit -m "Test frontend deployment"
   git push origin main
   ```
3. **Check GitHub Actions**:
   - Go to your repo → Actions tab
   - Watch the workflow run
4. **Visit your frontend URL**:
   - Use the URL from `terraform output frontend_url`
   - Should see your updated frontend

## Build Configuration

### For React/Vite Frontend
Your frontend uses Vite, so the build configuration is:
- **App location**: `/frontend` (where your package.json is)
- **Output location**: `dist` (Vite's default build output)

### Custom Build Commands (if needed)
If you need custom build commands, you can modify the generated GitHub Actions workflow:
```yaml
- name: Build And Deploy
  uses: Azure/static-web-apps-deploy@v1
  with:
    azure_static_web_apps_api_token: ${{ secrets.AZURE_STATIC_WEB_APPS_API_TOKEN }}
    repo_token: ${{ secrets.GITHUB_TOKEN }}
    app_location: "/frontend"
    output_location: "dist"
    # Optional: Custom build command
    # app_build_command: "npm run build"
```

## Troubleshooting

### Common Issues

1. **Build Fails**
   - Check if `package.json` exists in `/frontend`
   - Verify `npm install` and `npm run build` work locally
   - Check GitHub Actions logs for specific errors

2. **404 Errors**
   - Ensure `output_location` is correct (`dist` for Vite)
   - Check if `index.html` exists in build output

3. **Authentication Issues**
   - Re-authorize Azure in GitHub
   - Check Azure Static Web App permissions

### Useful Commands
```bash
# Check Static Web App status
az staticwebapp show --name autolog-test-frontend --resource-group autolog-rg-test

# List deployments
az staticwebapp deployment list --name autolog-test-frontend --resource-group autolog-rg-test

# Get deployment logs
az staticwebapp deployment show --name autolog-test-frontend --resource-group autolog-rg-test --deployment-id <deployment-id>
```

## Next Steps
1. ✅ Deploy infrastructure with Terraform
2. 🔄 Connect GitHub repository (this guide)
3. ✅ Test deployment with a code push
4. ✅ Configure custom domain (optional)
5. ✅ Set up environment variables (if needed)

## Cost Optimization
- Static Web Apps are very cost-effective (~$5-10/month)
- Free tier includes 2GB storage and 100GB bandwidth
- Perfect for React/Vite applications

## Security Notes
- GitHub Actions workflow uses Azure's managed identity
- No secrets needed in your repository
- HTTPS enabled by default
- Custom domains supported 