# AutoLog

An advanced log analysis platform using LLM for Root Cause Analysis (RCA) and multi-source log integration. Designed to streamline log analysis, provide intelligent insights, and generate comprehensive RCA reports from various log sources including CloudWatch, Splunk, and more.

## 🚀 Features

- **Multi-Source Log Integration**: Connect to CloudWatch, Splunk, and other log sources
- **LLM-Powered Analysis**: Advanced log analysis using Large Language Models
- **Enhanced ML Log Parsing**: Intelligent log parsing using 14+ ML algorithms (Drain, Spell, IPLoM, LogCluster, etc.)
- **Universal Log Support**: Parse JSON, structured, unstructured, and mixed log formats
- **Root Cause Analysis (RCA)**: Automated generation of comprehensive RCA reports
- **Real-time Log Processing**: Live log ingestion and analysis
- **Intelligent Insights**: AI-driven pattern recognition and anomaly detection
- **Role-based Access Control**: Secure access management with different user roles
- **Dashboard & Analytics**: Comprehensive reporting and log analytics
- **Modern UI/UX**: Intuitive interface built with React

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
├── prd/              # Product Requirements Document
└── docs/             # Project documentation
```

## 🛠️ Setup Instructions

### Prerequisites

- Go 1.24+
- Node.js 18+ 
- Python 3.10+
- PostgreSQL 14+
- npm or yarn

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd autolog
   ```

2. **Install frontend dependencies**
   ```bash
   npm run install:all
   ```

3. **Install logparser microservice dependencies**
   ```bash
   cd logparser_service
   pip install -r requirements.txt
   cd ..
   ```

4. **Environment Setup**
   ```bash
   # Copy environment files
   cp backend/.env.example backend/.env
   cp frontend/.env.example frontend/.env
   ```

5. **Start Development Environment**
   ```bash
   # Using Docker (recommended)
   make docker-dev
   
   # Or locally
   make dev
   ```

### Development

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080
- **Logparser Microservice**: http://localhost:8001
- **Database**: PostgreSQL on localhost:5432

## 🔐 Default Login Credentials

The application comes with pre-configured users for testing:

- **Admin**: `admin@autolog.com` / `admin123`
- **Manager**: `manager@autolog.com` / `manager123`
- **Responder**: `responder@autolog.com` / `responder123`
- **Viewer**: `viewer@autolog.com` / `viewer123`

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

### **Features:**
- **Intelligent Algorithm Selection**: Automatically chooses the best ML algorithm based on log characteristics
- **Field Extraction**: Extracts timestamps, log levels, IP addresses, HTTP fields, and more
- **High Performance**: Processes 7,000+ entries per second
- **Robust Fallback**: Multiple fallback mechanisms for edge cases
- **Comprehensive Testing**: Full test suite with real-world scenarios

### **Testing the Logparser:**

```bash
# Run all tests
cd logparser_service
python3 test_all.py

# Test specific components
python3 test_all.py --ml          # Test ML parser functionality
python3 test_all.py --microservice # Test microservice API
python3 test_all.py --real-world  # Test real-world scenarios
python3 test_all.py --performance # Test performance with large datasets

# Test microservice health
curl http://localhost:8001/health
```

## 🏥 Health Checks & Startup Order

The application uses Docker's native health checks to ensure proper startup order:

1. **Database** starts first with PostgreSQL health checks
2. **Logparser Microservice** starts with health endpoint
3. **Backend** waits for database and logparser to be healthy, then starts
4. **Frontend** waits for backend to be healthy before starting

### Health Endpoints

**Backend Health**: `http://localhost:8080/health`
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0.0",
  "services": {
    "database": {
      "status": "ok",
      "error": null
    }
  }
}
```

**Logparser Health**: `http://localhost:8001/health`
```json
{
  "status": "healthy",
  "service": "logparser"
}
```

### Health Check Commands

```bash
# Check all services
make health

# Check specific services
make health-backend
make health-frontend
make health-db

# Test health endpoint with detailed output
make health-test
make health-test-docker
```

## 🌱 Initial Data Configuration

The application automatically seeds the database with initial users when starting in development mode. You can customize this data by editing the JSON files in `backend/data/`:

- `backend/data/initial-users.json` - Initial user accounts

See `backend/data/README.md` for detailed configuration options.

## 📚 Available Scripts

- `npm run dev` - Start both frontend and backend in development mode
- `npm run build` - Build both frontend and backend for production
- `npm run test` - Run tests for both frontend and backend
- `npm run lint` - Run linting for frontend

## 🔧 Configuration

### Environment Variables

#### Backend (.env)
```
DATABASE_URL="postgresql://username:password@localhost:5432/autolog"
JWT_SECRET="your-jwt-secret"
PORT=8080
ENV=development
LOGPARSER_URL="http://localhost:8001"
```

#### Frontend (.env)
```
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

For support and questions, please open an issue in the GitHub repository or contact the development team. 