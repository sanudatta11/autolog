import React, { createContext, useContext, useState, useEffect } from 'react'
import api from '../services/api'
// import { hashPassword } from '../utils/crypto'  // Remove hashing

const AuthContext = createContext()

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)
  const [token, setToken] = useState(localStorage.getItem('token'))

  useEffect(() => {
    const token = localStorage.getItem('token')
    const backendUrl = localStorage.getItem('backendUrl')
    
    if (token && backendUrl) {
      // Update API base URL with stored backend URL
      api.defaults.baseURL = backendUrl
      api.defaults.headers.common['Authorization'] = `Bearer ${token}`
      setToken(token)
      fetchCurrentUser(token)
    } else {
      setLoading(false)
    }
  }, [])

  const fetchCurrentUser = async (token) => {
    try {
      const response = await api.get('/users/me')
      setUser(response.data)
    } catch (error) {
      // Only remove token if error is 401 (unauthorized)
      if (error.response && error.response.status === 401) {
        localStorage.removeItem('token')
        delete api.defaults.headers.common['Authorization']
        setUser(null)
        setToken(null)
      }
      // For other errors (e.g., network), keep the token and show loading as false
    } finally {
      setLoading(false)
    }
  }

  const login = async (email, password, backendUrl) => {
    try {
      // Update API base URL with the provided backend URL
      api.defaults.baseURL = backendUrl
      
      // Send raw password
      const response = await api.post('/auth/login', { email, password })
      const { token, user } = response.data
      if (token) {
        localStorage.setItem('token', token)
        api.defaults.headers.common['Authorization'] = `Bearer ${token}`
        setToken(token)
      }
      setUser(user)
      return { success: true }
    } catch (error) {
      return { 
        success: false, 
        error: error.response?.data?.message || 'Login failed' 
      }
    }
  }

  const logout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('backendUrl')
    delete api.defaults.headers.common['Authorization']
    setUser(null)
    setToken(null)
  }

  const getToken = () => {
    return localStorage.getItem('token')
  }

  const value = {
    user,
    login,
    logout,
    loading,
    token
  }

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
} 