import React, { useState } from 'react'
import api, { adminUsersAPI, managerUsersAPI } from '../services/api'
import { useAuth } from '../contexts/AuthContext'
import PasswordChange from '../components/PasswordChange'

function Settings() {
  const [selectedSection, setSelectedSection] = useState(null)
  const { user } = useAuth()

  const settingsOptions = [
    { 
      id: 'llm', 
      name: 'LLM Settings', 
      icon: '🤖', 
      description: 'Configure your LLM endpoint for RCA analysis and log processing',
      color: 'blue'
    },
    { 
      id: 'users', 
      name: 'User Management', 
      icon: '👥', 
      description: 'Manage user accounts and permissions',
      color: 'green'
    },
    { 
      id: 'password', 
      name: 'Change Password', 
      icon: '🔒', 
      description: 'Update your account password',
      color: 'purple'
    },
  ]

  // If no section is selected, show the selection screen
  if (!selectedSection) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
          <p className="text-gray-600 mt-1">Choose a settings section to configure</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {settingsOptions.map((option) => (
            <div
              key={option.id}
              onClick={() => setSelectedSection(option.id)}
              className="card cursor-pointer hover:shadow-md transition-shadow duration-200 p-6"
            >
              <div className="flex items-center space-x-4">
                <div className={`text-3xl ${
                  option.color === 'blue' ? 'text-blue-500' : 
                  option.color === 'green' ? 'text-green-500' : 
                  option.color === 'purple' ? 'text-purple-500' : 'text-gray-500'
                }`}>
                  {option.icon}
                </div>
                <div className="flex-1">
                  <h3 className="text-lg font-medium text-gray-900">{option.name}</h3>
                  <p className="text-sm text-gray-600 mt-1">{option.description}</p>
                </div>
                <div className="text-gray-400">
                  <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                  </svg>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  // If a section is selected, show the back button and the selected component
  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-4">
        <button
          onClick={() => setSelectedSection(null)}
          className="flex items-center space-x-2 text-gray-600 hover:text-gray-900 transition-colors"
        >
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          <span>Back to Settings</span>
        </button>
        <div className="h-6 w-px bg-gray-300"></div>
        <div>
          <h1 className="text-2xl font-bold text-gray-900">
            {settingsOptions.find(opt => opt.id === selectedSection)?.name}
          </h1>
        </div>
      </div>

      {selectedSection === 'llm' && <LLMSettings />}
      {selectedSection === 'users' && <UserManagement />}
      {selectedSection === 'password' && <PasswordChange />}
    </div>
  )
}

// LLM Settings Component
function LLMSettings() {
  const [llmEndpoint, setLlmEndpoint] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [lastCheck, setLastCheck] = useState(null)
  const [isTested, setIsTested] = useState(false)
  const [testResult, setTestResult] = useState(null)
  const { user } = useAuth()

  React.useEffect(() => {
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

  const handleTestLLMEndpoint = async () => {
    if (!llmEndpoint.trim()) {
      setError('Please enter an LLM endpoint URL to test')
      return
    }

    setTesting(true)
    setError('')
    setSuccess('')
    setTestResult(null)

    try {
      // Test the endpoint by calling the backend test endpoint
      const response = await api.post('/settings/test-llm-endpoint', {
        llm_endpoint: llmEndpoint
      })
      setTestResult({ success: true, message: 'LLM endpoint is working correctly!' })
      setIsTested(true)
      setSuccess('LLM endpoint test successful! You can now save the settings.')
    } catch (err) {
      setTestResult({ 
        success: false, 
        message: err.response?.data?.message || err.response?.data?.error || 'Failed to connect to LLM endpoint' 
      })
      setIsTested(false)
      setError('LLM endpoint test failed. Please check the URL and try again.')
    } finally {
      setTesting(false)
    }
  }

  const handleSaveLLMEndpoint = async (e) => {
    e.preventDefault()
    
    if (!isTested) {
      setError('Please test the LLM endpoint before saving')
      return
    }

    setSaving(true)
    setError('')
    setSuccess('')

    try {
      await api.post('/settings/llm-endpoint', {
        llm_endpoint: llmEndpoint
      })
      setSuccess('LLM endpoint updated successfully!')
      setIsTested(false) // Reset test status after saving
      setTestResult(null)
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
        <div className="text-gray-500">Loading LLM settings...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
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
                onChange={(e) => {
                  setLlmEndpoint(e.target.value)
                  setIsTested(false) // Reset test status when URL changes
                  setTestResult(null)
                }}
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

            {/* Test Result Display */}
            {testResult && (
              <div className={`p-4 rounded-md ${
                testResult.success 
                  ? 'bg-green-50 border border-green-200 text-green-700' 
                  : 'bg-red-50 border border-red-200 text-red-700'
              }`}>
                <div className="flex items-center">
                  <div className="flex-shrink-0">
                    {testResult.success ? (
                      <svg className="h-5 w-5 text-green-400" fill="currentColor" viewBox="0 0 20 20">
                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                      </svg>
                    ) : (
                      <svg className="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                      </svg>
                    )}
                  </div>
                  <div className="ml-3">
                    <p className="text-sm font-medium">
                      {testResult.message}
                    </p>
                  </div>
                </div>
              </div>
            )}

            <div className="flex items-center justify-between pt-4">
              <div className="text-sm text-gray-600">
                <p>• This endpoint will be used for all LLM operations</p>
                <p>• Make sure your LLM service is accessible from this application</p>
                <p>• The endpoint should support Ollama-compatible API</p>
                <p>• You must test the endpoint before saving</p>
              </div>
              <div className="flex space-x-3">
                <button
                  type="button"
                  onClick={handleTestLLMEndpoint}
                  className="bg-gray-600 text-white px-6 py-2 rounded hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={testing || !llmEndpoint.trim() || !isURLValid}
                >
                  {testing ? 'Testing...' : 'Test Endpoint'}
                </button>
                <button
                  type="submit"
                  className="bg-primary-600 text-white px-6 py-2 rounded hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={saving || !isTested || (!isURLValid && llmEndpoint)}
                >
                  {saving ? 'Saving...' : 'Save Settings'}
                </button>
              </div>
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

// User Management Component
function UserManagement() {
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showAddForm, setShowAddForm] = useState(false)
  const [addForm, setAddForm] = useState({
    firstName: '',
    lastName: '',
    email: '',
    password: '',
    role: 'VIEWER',
  })
  const [addLoading, setAddLoading] = useState(false)
  const [addError, setAddError] = useState('')
  const [addSuccess, setAddSuccess] = useState('')
  const { user } = useAuth()
  const [deleteError, setDeleteError] = useState('')

  React.useEffect(() => {
    fetchUsers()
  }, [])

  const fetchUsers = async () => {
    try {
      setLoading(true)
      const response = await api.get('/users')
      setUsers(response.data.users || [])
    } catch (error) {
      setError('Error fetching users')
      console.error('Error fetching users:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleAddUser = async (e) => {
    e.preventDefault()
    setAddLoading(true)
    setAddError('')
    setAddSuccess('')
    try {
      // Use appropriate API based on user role
      if (user?.role === 'ADMIN') {
        await adminUsersAPI.addAdminUser(addForm)
      } else if (user?.role === 'MANAGER') {
        await managerUsersAPI.addManagerUser(addForm)
      }
      setAddSuccess('User added successfully!')
      setShowAddForm(false)
      setAddForm({ firstName: '', lastName: '', email: '', password: '', role: 'VIEWER' })
      fetchUsers()
    } catch (err) {
      setAddError(err.response?.data?.error || 'Failed to add user')
    } finally {
      setAddLoading(false)
    }
  }

  const handleDeleteUser = async (id) => {
    setDeleteError('')
    if (!window.confirm('Are you sure you want to delete this user?')) return
    try {
      await adminUsersAPI.deleteAdminUser(id)
      fetchUsers()
    } catch (err) {
      setDeleteError(err.response?.data?.error || 'Failed to delete user')
    }
  }

  const getRoleColor = (role) => {
    switch (role) {
      case 'ADMIN': return 'bg-red-100 text-red-800'
      case 'MANAGER': return 'bg-blue-100 text-blue-800'
      case 'RESPONDER': return 'bg-green-100 text-green-800'
      case 'VIEWER': return 'bg-gray-100 text-gray-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  // Check if user can add users (admin or manager)
  const canAddUsers = user?.role === 'ADMIN' || user?.role === 'MANAGER'
  
  // Check if user can delete users (admin only)
  const canDeleteUsers = user?.role === 'ADMIN'

  // Count number of admins
  const adminCount = users.filter(u => u.role === 'ADMIN').length

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading users...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-lg font-medium text-gray-900">User Management</h2>
          <p className="text-sm text-gray-600 mt-1">Manage user accounts and permissions</p>
        </div>
        {canAddUsers && (
          <button
            onClick={() => setShowAddForm(true)}
            className="bg-primary-600 text-white px-4 py-2 rounded-md hover:bg-primary-700 transition-colors"
          >
            Add User
          </button>
        )}
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          {error}
        </div>
      )}
      {deleteError && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          {deleteError}
        </div>
      )}

      {/* Users List */}
      <div className="bg-white shadow rounded-lg">
        {users.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            <p className="text-lg mb-2">No users found</p>
            <p>Users will appear here once they are added</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Email
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Role
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Created
                  </th>
                  {canDeleteUsers && (
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Actions
                    </th>
                  )}
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {users.map((userItem) => {
                  const isCurrentUser = userItem.id === user?.id
                  const isAdmin = userItem.role === 'ADMIN'
                  // Only allow delete if not current user, and if admin, only if more than 1 admin
                  const canDeleteThisUser = canDeleteUsers && !isCurrentUser && (!isAdmin || adminCount > 1)
                  return (
                    <tr key={userItem.id}>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-medium text-gray-900">
                          {userItem.firstName} {userItem.lastName}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-900">{userItem.email}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getRoleColor(userItem.role)}`}>
                          {userItem.role}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {new Date(userItem.createdAt).toLocaleDateString()}
                      </td>
                      {canDeleteUsers && (
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                          <button
                            onClick={() => canDeleteThisUser ? handleDeleteUser(userItem.id) : null}
                            className={`text-red-600 hover:text-red-900 ${!canDeleteThisUser ? 'opacity-50 cursor-not-allowed' : ''}`}
                            disabled={!canDeleteThisUser}
                            title={isCurrentUser ? 'You cannot delete your own account' : (isAdmin && adminCount <= 1 ? 'At least one admin must remain' : 'Delete user')}
                          >
                            Delete
                          </button>
                        </td>
                      )}
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Add User Modal */}
      {showAddForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-40">
          <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-medium text-gray-900">Add New User</h3>
              <button
                onClick={() => setShowAddForm(false)}
                className="text-gray-400 hover:text-gray-600"
              >
                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <form onSubmit={handleAddUser} className="space-y-4">
              {addError && (
                <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
                  {addError}
                </div>
              )}
              {addSuccess && (
                <div className="bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded">
                  {addSuccess}
                </div>
              )}

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  First Name
                </label>
                <input
                  type="text"
                  required
                  className="input w-full"
                  value={addForm.firstName}
                  onChange={(e) => setAddForm({ ...addForm, firstName: e.target.value })}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Last Name
                </label>
                <input
                  type="text"
                  required
                  className="input w-full"
                  value={addForm.lastName}
                  onChange={(e) => setAddForm({ ...addForm, lastName: e.target.value })}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Email
                </label>
                <input
                  type="email"
                  required
                  className="input w-full"
                  value={addForm.email}
                  onChange={(e) => setAddForm({ ...addForm, email: e.target.value })}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Password
                </label>
                <input
                  type="password"
                  required
                  className="input w-full"
                  value={addForm.password}
                  onChange={(e) => setAddForm({ ...addForm, password: e.target.value })}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Role
                </label>
                <select
                  className="input w-full"
                  value={addForm.role}
                  onChange={(e) => setAddForm({ ...addForm, role: e.target.value })}
                >
                  <option value="VIEWER">Viewer</option>
                  <option value="RESPONDER">Responder</option>
                  {user?.role === 'ADMIN' && <option value="MANAGER">Manager</option>}
                  {user?.role === 'ADMIN' && <option value="ADMIN">Admin</option>}
                </select>
              </div>

              <div className="flex justify-end space-x-3 pt-4">
                <button
                  type="button"
                  onClick={() => setShowAddForm(false)}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 text-sm font-medium text-white bg-primary-600 border border-transparent rounded-md hover:bg-primary-700 disabled:opacity-50"
                  disabled={addLoading}
                >
                  {addLoading ? 'Adding...' : 'Add User'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
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