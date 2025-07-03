# IncidentSage

A modern incident management and response platform designed to streamline incident handling, improve team collaboration, and provide comprehensive incident lifecycle management.

## 🚀 Features

- **Incident Management**: Create, track, and manage incidents with full lifecycle support
- **Real-time Collaboration**: Live updates and team communication during incidents
- **Role-based Access Control**: Secure access management with different user roles
- **Dashboard & Analytics**: Comprehensive reporting and incident metrics
- **Integration Ready**: API-first design for easy third-party integrations
- **Modern UI/UX**: Intuitive interface built with React

## 🏗️ Architecture

- **Frontend**: React 18 + JavaScript + Vite
- **Backend**: Go + Gin + GORM
- **Database**: PostgreSQL
- **Real-time**: WebSocket support
- **Authentication**: JWT-based authentication
- **Styling**: Tailwind CSS

## 📁 Project Structure

```
incident-sage/
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

- Go 1.21+
- Node.js 18+ 
- PostgreSQL 14+
- npm or yarn

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd incident-sage
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

4. **Database Setup**
   ```bash
   cd backend
   go run cmd/migrate/main.go
   ```

5. **Start Development Servers**
   ```bash
   npm run dev
   ```

### Development

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080
- **Database**: PostgreSQL on localhost:5432

## 📚 Available Scripts

- `npm run dev` - Start both frontend and backend in development mode
- `npm run build` - Build both frontend and backend for production
- `npm run test` - Run tests for both frontend and backend
- `npm run lint` - Run linting for frontend

## 🔧 Configuration

### Environment Variables

#### Backend (.env)
```
DATABASE_URL="postgresql://username:password@localhost:5432/incident_sage"
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