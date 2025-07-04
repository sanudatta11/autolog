# AutoLog

A modern incident management and response platform designed to streamline incident handling, improve team collaboration, and provide comprehensive incident lifecycle management.

## 🚀 Features

- **Incident Management**: Create, track, and manage incidents with full lifecycle support
- **Real-time Collaboration**: Live updates and team communication during incidents
- **Role-based Access Control**: Secure access management with different user roles
- **Dashboard & Analytics**: Comprehensive reporting and incident metrics
- **Integration Ready**: API-first design for easy third-party integrations
- **Modern UI/UX**: Intuitive interface built with React

## 🏗️ Architecture

- **Frontend**: React 18 + JavaScript + Vite (with live reload)
- **Backend**: Go + Gin + GORM (normal build)
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
├── shared/            # Shared types and utilities
├── prd/              # Product Requirements Document
└── docs/             # Project documentation
```

## 🛠️ Setup Instructions

### Prerequisites

- Go 1.24+
- Node.js 18+ 
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

3. **Environment Setup**
   ```bash
   # Copy environment files
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

### Development

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080
- **Database**: PostgreSQL on localhost:5432
- **Adminer**: http://localhost:8081 (Database management)

## 🔐 Default Login Credentials

The application comes with pre-configured users for testing:

- **Admin**: `admin@autolog.com` / `admin123`
- **Manager**: `manager@autolog.com` / `manager123`
- **Responder**: `responder@autolog.com` / `responder123`
- **Viewer**: `viewer@autolog.com` / `viewer123`

## 🏥 Health Checks & Startup Order

The application uses Docker's native health checks to ensure proper startup order:

1. **Database** starts first with PostgreSQL health checks
2. **Backend** waits for database to be healthy, then starts with health endpoint
3. **Frontend** waits for backend to be healthy before starting

### Health Endpoint

The backend provides a detailed health endpoint at `http://localhost:8080/health`:

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

**Note**: Docker automatically manages the startup order using health checks. If the backend is unhealthy, the frontend container will not start, preventing the `ERR_EMPTY_RESPONSE` error.

## 🌱 Initial Data Configuration

The application automatically seeds the database with initial users and sample incidents when starting in development mode. You can customize this data by editing the JSON files in `backend/data/`:

- `backend/data/initial-users.json` - Initial user accounts
- `backend/data/initial-incidents.json` - Sample incidents

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