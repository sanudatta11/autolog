import axios from 'axios'
// import { hashPassword } from '../utils/crypto'  // Remove hashing

// Get base URL from localStorage or fall back to environment variable or default
export const getApiUrl = (path) => {
  const storedBackendUrl = localStorage.getItem('backendUrl')
  const base = storedBackendUrl || import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
  // Remove trailing slash from base
  const cleanBase = base.replace(/\/$/, '')
  // If path already starts with /api/v1/, use it as is
  if (path.startsWith('/api/v1/')) {
    return cleanBase + path
  }
  // Otherwise, add the path
  return cleanBase + (path.startsWith('/') ? path : '/' + path)
}

const getBaseURL = () => {
  const storedBackendUrl = localStorage.getItem('backendUrl')
  if (storedBackendUrl) {
    return storedBackendUrl
  }
  return import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
}

const api = axios.create({
  baseURL: getBaseURL(),
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true,
})

// Request interceptor to add auth token and update base URL
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    // Update base URL for each request in case it changed
    const currentBaseURL = getBaseURL()
    if (config.baseURL !== currentBaseURL) {
      config.baseURL = currentBaseURL
    }
    return config
  },
  (error) => Promise.reject(error)
)

// Response interceptor to handle auth errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('backendUrl')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// Function to update base URL (can be called from other components)
export const updateApiBaseURL = (newBaseURL) => {
  localStorage.setItem('backendUrl', newBaseURL)
  api.defaults.baseURL = newBaseURL
}

// NOTE: Use only the exported 'api' instance or 'getApiUrl' for all API calls in the frontend.
// Parsing Rules API
export const parsingRulesAPI = {
  // Get all parsing rules for the current user
  getUserParsingRules: () => api.get('/parsing-rules'),
  
  // Get a specific parsing rule
  getParsingRule: (id) => api.get(`/parsing-rules/${id}`),
  
  // Create a new parsing rule
  createParsingRule: (rule) => api.post('/parsing-rules', rule),
  
  // Update an existing parsing rule
  updateParsingRule: (id, rule) => api.put(`/parsing-rules/${id}`, rule),
  
  // Delete a parsing rule
  deleteParsingRule: (id) => api.delete(`/parsing-rules/${id}`),
  
  // Test a parsing rule against sample logs
  testParsingRule: (rule, sampleLogs) => api.post('/parsing-rules/test', { rule, sampleLogs }),
  
  // Get active parsing rules
  getActiveParsingRules: () => api.get('/parsing-rules/active'),
}

// Admin User Management API
export const adminUsersAPI = {
  addAdminUser: (user) => api.post('/admin/users', user),
  deleteAdminUser: (id) => api.delete(`/admin/users/${id}`),
  updateUserRole: (id, role) => api.put(`/admin/users/${id}/role`, { role }),
}

// Manager User Management API
export const managerUsersAPI = {
  addManagerUser: (user) => api.post('/manager/users', user),
}

// Password Change API
export const passwordAPI = {
  changePassword: (currentPassword, newPassword) =>
    api.post('/auth/change-password', { currentPassword, newPassword }),
}

export default api 