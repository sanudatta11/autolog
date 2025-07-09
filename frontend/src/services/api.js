import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'https://backend.autolog.tech',
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

export default api 