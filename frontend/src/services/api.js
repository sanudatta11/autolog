import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true,
})

// Request interceptor to add auth token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor to handle auth errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// Parsing Rules API
export const parsingRulesAPI = {
  // Get all parsing rules for the current user
  getUserParsingRules: () => api.get('/api/v1/parsing-rules'),
  
  // Get a specific parsing rule
  getParsingRule: (id) => api.get(`/api/v1/parsing-rules/${id}`),
  
  // Create a new parsing rule
  createParsingRule: (rule) => api.post('/api/v1/parsing-rules', rule),
  
  // Update an existing parsing rule
  updateParsingRule: (id, rule) => api.put(`/api/v1/parsing-rules/${id}`, rule),
  
  // Delete a parsing rule
  deleteParsingRule: (id) => api.delete(`/api/v1/parsing-rules/${id}`),
  
  // Test a parsing rule against sample logs
  testParsingRule: (rule, sampleLogs) => api.post('/api/v1/parsing-rules/test', { rule, sampleLogs }),
  
  // Get active parsing rules
  getActiveParsingRules: () => api.get('/api/v1/parsing-rules/active'),
}

export default api 