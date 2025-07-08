# AutoLog

An advanced log analysis platform using LLM for Root Cause Analysis (RCA) and multi-source log integration. Designed to streamline log analysis, provide intelligent insights, and generate comprehensive RCA reports from various log sources including CloudWatch, Splunk, and more.

## 🚀 Features

- **Proactive AI-Driven Monitoring**: Autogenerates alerts before failures - no custom monitors needed
- **LLM-Powered Root Cause Analysis**: Automated, developer-friendly RCA with instant preliminary findings
- **Code-Aware Fix Suggestions**: (Optional) Auto-generates PRs for common issues before a dev even logs in
- **Multi-Source Log Integration**: Connect to CloudWatch, Splunk, and more
- **Intelligent Insights**: AI-driven anomaly detection and pattern recognition
- **Enhanced ML Log Parsing**: 14+ ML algorithms for all log types (JSON, structured, unstructured, mixed)
- **Real-time Log Processing**: Live log ingestion and analysis
- **Role-based Access Control**: Secure, multi-user support
- **Dashboard & Analytics**: Comprehensive reporting and log analytics
- **Modern UI/UX**: Intuitive React interface

## 🏗️ Architecture

- **Frontend**: React 18 + JavaScript + Vite (with live reload)
- **Backend**: Go + Gin + GORM (normal build)
- **Logparser Microservice**: Python + FastAPI + ML algorithms (port 8001)
- **Database**: PostgreSQL
- **Real-time**: WebSocket support
- **Authentication**: JWT-based authentication
- **Styling**: Tailwind CSS

## 📁 Project Structure

```
autolog/
├── frontend/          # React frontend application
├── backend/           # Go backend API
│   ├── cmd/          # Application entry points
│   ├── internal/     # Private application code
│   ├── pkg/          # Public libraries
│   └── migrations/   # Database migrations
├── logparser_service/ # Python ML logparser microservice
│   ├── enhanced_ml_parser.py  # Enhanced ML parser with 14 algorithms
│   ├── main.py               # FastAPI microservice
│   ├── test_all.py           # Comprehensive test suite
│   └── requirements.txt      # Python dependencies
├── shared/            # Shared types and utilities
├── terraform/         # Azure deployment configuration
├── prd/              # Product Requirements Document
└── docs/             # Project documentation
```

## 🛠️ Quick Start

### Prerequisites

- Go 1.24+
- Node.js 18+ 
- Python 3.10+
- PostgreSQL 14+
- npm or yarn

### Local Development

1. **Clone and setup**
   ```bash
   git clone <repository-url>
   cd autolog
   npm run install:all
   ```

2. **Install Python dependencies**
   ```bash
   cd logparser_service
   pip install -r requirements.txt
   cd ..
   ```

3. **Environment Setup**
   ```bash
   cp backend/.env.example backend/.env
   cp frontend/.env.example frontend/.env
   ```

4. **Start Development Environment**
   ```bash
   # Using Docker (recommended)
   make docker-dev
   
   # Or locally
   make dev
   ```

### Access Points

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080
- **Logparser Microservice**: http://localhost:8001
- **Database**: PostgreSQL on localhost:5432

### Default Login Credentials

- **Admin**: `admin@autolog.com` / `admin123`
- **Manager**: `manager@autolog.com` / `manager123`
- **Responder**: `responder@autolog.com` / `responder123`
- **Viewer**: `viewer@autolog.com` / `viewer123`

## 🚀 Deployment

### Azure Static Web Apps (SWA) Deployment

For quick frontend deployment using Azure Static Web Apps:

#### Prerequisites
- Azure CLI installed and authenticated
- Azure Static Web Apps CLI installed: `npm install -g @azure/static-web-apps-cli`

#### Quick SWA Deployment

1. **Install SWA CLI globally**
   ```bash
   npm install -g @azure/static-web-apps-cli
   ```

2. **Navigate to frontend directory**
   ```bash
   cd frontend
   ```

3. **Login to Azure (if not already logged in)**
   ```bash
   az login
   ```

4. **Deploy to Azure Static Web Apps**
   ```bash
   # Build and deploy
   swa build
   
   # Or deploy directly (builds automatically)
   swa deploy
   ```

5. **For development with SWA**
   ```bash
   # Start local development server with SWA
   swa start
   ```

#### SWA Configuration

The project includes a `swa-cli.config.json` file configured for:
- **App Location**: `frontend` directory
- **Output Location**: `dist` (Vite build output)
- **Build Command**: `npm run build`
- **Dev Server**: `npm run dev` on port 5173

#### Troubleshooting SWA Issues

If you encounter shell execution errors (common in WSL2):
```bash
# Create a build script wrapper
echo '#!/bin/bash
cd "$(dirname "$0")"
npm run build' > frontend/build.sh

chmod +x frontend/build.sh

# Update swa-cli.config.json to use the script
# "appBuildCommand": "./build.sh"
```

### Full Azure Infrastructure Deployment

For complete infrastructure deployment (backend, database, etc.), see [DEPLOYMENT.md](./DEPLOYMENT.md).

**Quick Terraform deployment:**
```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
./deploy.sh deploy
```

**Estimated costs:**
- **SWA Frontend Only**: $0-20/month (free tier available)
- **Full Infrastructure**: $40-75/month for test environment

## 🧠 Enhanced ML Log Parser

The logparser microservice provides intelligent log parsing using 14+ machine learning algorithms:

### **Available ML Algorithms:**
- **Drain**: Hierarchical clustering for structured logs
- **Spell**: Spell-based log parsing for duplicate detection
- **IPLoM**: Iterative partitioning for log mining
- **LogCluster**: Clustering-based log parsing
- **LenMa**: Length-based log parsing
- **LFA**: Log format analyzer
- **LKE**: Log key extraction
- **LogMine**: Multi-level log parsing
- **LogSig**: Signature-based log parsing
- **Logram**: N-gram based log parsing
- **SLCT**: Simple log clustering toolkit
- **ULP**: Unsupervised log parsing
- **Brain**: Neural network-based parsing
- **AEL**: Automated event log parsing

### **Supported Log Types:**
- **JSON Logs**: Structured JSON with automatic field extraction
- **Structured Logs**: Syslog, application logs, system logs
- **Web Server Logs**: Apache, Nginx access logs
- **Container Logs**: Docker, Kubernetes logs
- **Security Logs**: Authentication, authorization events
- **Mixed Content**: Hybrid JSON and unstructured logs
- **Unstructured Logs**: Free-form text with intelligent parsing

### **Testing the Logparser:**

```bash
cd logparser_service
python3 test_all.py
```

## 🤖 Proactive AI-Driven Monitoring

AutoLog isn't just a log analyzer—it's your tireless, ever-vigilant AI SRE sidekick:

- **Autogenerated Alerts Before Disaster Strikes**: Constantly watches your logs, dynamically detects emerging error patterns
- **Zero Custom Monitors Needed**: Adapts to new log formats and error types automatically
- **Developer-First RCA Pages**: Every alert links to a rich, developer-friendly RCA page
- **Preliminary RCA, Instantly**: As soon as an anomaly is detected, AutoLog runs preliminary RCA
- **Code-Aware Fix Suggestions**: Can suggest or draft Pull Requests to fix common issues

## 📚 Documentation

- **[DEPLOYMENT.md](./DEPLOYMENT.md)** - Complete Azure deployment guide with cost optimization
- **[Product Requirements](./prd/)** - Detailed product specifications and requirements

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details. 