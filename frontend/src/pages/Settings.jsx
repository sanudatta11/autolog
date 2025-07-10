import React, { useState, useEffect } from 'react'
import api from '../services/api'
import { useAuth } from '../contexts/AuthContext'

function Settings() {
  const [llmEndpoint, setLlmEndpoint] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [lastCheck, setLastCheck] = useState(null)
  const { user } = useAuth()

  useEffect(() => {
    fetchLLMEndpoint()
  }, [])

  const fetchLLMEndpoint = async () => {
    try {
      setLoading(true)
      const response = await api.get('/settings/llm-endpoint')
      setLlmEndpoint(response.data.llm_endpoint || '')
      setLastCheck(response.data.lastLLMStatusCheck || null)
    } catch (error) {
      console.error('Error fetching LLM endpoint:', error)
      // Don't show error for initial load, just set empty
    } finally {
      setLoading(false)
    }
  }

  const handleSaveLLMEndpoint = async (e) => {
    e.preventDefault()
    setSaving(true)
    setError('')
    setSuccess('')

    try {
      await api.post('/settings/llm-endpoint', {
        llm_endpoint: llmEndpoint
      })
      setSuccess('LLM endpoint updated successfully!')
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to update LLM endpoint')
    } finally {
      setSaving(false)
    }
  }

  const validateURL = (url) => {
    if (!url) return true // Allow empty
    try {
      new URL(url)
      return true
    } catch {
      return false
    }
  }

  const isURLValid = validateURL(llmEndpoint)

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading settings...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
        <p className="text-gray-600 mt-1">Configure your application settings</p>
      </div>

      <div className="card">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">LLM Configuration</h2>
          <p className="text-sm text-gray-600 mt-1">
            Configure your LLM endpoint for RCA analysis and log processing
          </p>
        </div>
        <div className="p-6">
          <form onSubmit={handleSaveLLMEndpoint} className="space-y-4">
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
                {error}
              </div>
            )}
            {success && (
              <div className="bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded">
                {success}
              </div>
            )}
            
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                LLM Endpoint URL
              </label>
              <input
                type="url"
                className={`input w-full ${!isURLValid && llmEndpoint ? 'border-red-300 focus:border-red-500 focus:ring-red-500' : ''}`}
                placeholder="https://your-llm-server.com"
                value={llmEndpoint}
                onChange={(e) => setLlmEndpoint(e.target.value)}
              />
              {!isURLValid && llmEndpoint && (
                <p className="text-red-600 text-sm mt-1">
                  Please enter a valid URL
                </p>
              )}
              <p className="text-gray-500 text-sm mt-1">
                Enter the URL of your LLM service (e.g., Ollama server). Leave empty to use default.
              </p>
              {lastCheck && (
                <p className="text-xs text-gray-600 mt-1">
                  Last successful status check: {new Date(lastCheck).toLocaleString()}
                </p>
              )}
            </div>

            <div className="flex items-center justify-between pt-4">
              <div className="text-sm text-gray-600">
                <p>• This endpoint will be used for all LLM operations</p>
                <p>• Make sure your LLM service is accessible from this application</p>
                <p>• The endpoint should support Ollama-compatible API</p>
              </div>
              <button
                type="submit"
                className="bg-primary-600 text-white px-6 py-2 rounded hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={saving || (!isURLValid && llmEndpoint)}
              >
                {saving ? 'Saving...' : 'Save Settings'}
              </button>
            </div>
          </form>
        </div>
      </div>

      <div className="card">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">User Information</h2>
        </div>
        <div className="p-6">
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700">Name</label>
              <p className="text-gray-900">{user?.firstName} {user?.lastName}</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Email</label>
              <p className="text-gray-900">{user?.email}</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Role</label>
              <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getRoleColor(user?.role)}`}>
                {user?.role}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function getRoleColor(role) {
  switch (role) {
    case 'ADMIN': return 'bg-red-100 text-red-800'
    case 'MANAGER': return 'bg-blue-100 text-blue-800'
    case 'RESPONDER': return 'bg-green-100 text-green-800'
    case 'VIEWER': return 'bg-gray-100 text-gray-800'
    default: return 'bg-gray-100 text-gray-800'
  }
}

export default Settings 