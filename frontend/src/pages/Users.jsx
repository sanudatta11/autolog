import React, { useState, useEffect } from 'react'
import api, { adminUsersAPI, managerUsersAPI } from '../services/api'
import { useAuth } from '../contexts/AuthContext'

function Users() {
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

  useEffect(() => {
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
    if (!window.confirm('Are you sure you want to delete this user?')) return
    try {
      await adminUsersAPI.deleteAdminUser(id)
      fetchUsers()
    } catch (err) {
      alert(err.response?.data?.error || 'Failed to delete user')
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

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading users...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Users</h1>
        <p className="text-gray-600 mt-1">Manage team members and their roles</p>
      </div>

      {canAddUsers && (
        <div className="mb-4">
          <button
            className="bg-primary-600 text-white px-4 py-2 rounded hover:bg-primary-700"
            onClick={() => setShowAddForm((v) => !v)}
          >
            {showAddForm ? 'Cancel' : 'Add User'}
          </button>
          {showAddForm && (
            <form className="mt-4 space-y-4 bg-white p-4 rounded shadow" onSubmit={handleAddUser}>
              {addError && <div className="text-red-600">{addError}</div>}
              {addSuccess && <div className="text-green-600">{addSuccess}</div>}
              <div className="flex space-x-4">
                <div className="flex-1">
                  <label className="block text-sm font-medium text-gray-700">First Name</label>
                  <input
                    type="text"
                    className="input mt-1"
                    value={addForm.firstName}
                    onChange={e => setAddForm(f => ({ ...f, firstName: e.target.value }))}
                    required
                  />
                </div>
                <div className="flex-1">
                  <label className="block text-sm font-medium text-gray-700">Last Name</label>
                  <input
                    type="text"
                    className="input mt-1"
                    value={addForm.lastName}
                    onChange={e => setAddForm(f => ({ ...f, lastName: e.target.value }))}
                    required
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Email</label>
                <input
                  type="email"
                  className="input mt-1"
                  value={addForm.email}
                  onChange={e => setAddForm(f => ({ ...f, email: e.target.value }))}
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Password</label>
                <input
                  type="password"
                  className="input mt-1"
                  value={addForm.password}
                  onChange={e => setAddForm(f => ({ ...f, password: e.target.value }))}
                  required
                  minLength={6}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Role</label>
                <select
                  className="input mt-1"
                  value={addForm.role}
                  onChange={e => setAddForm(f => ({ ...f, role: e.target.value }))}
                  required
                >
                  {user?.role === 'ADMIN' && <option value="ADMIN">Admin</option>}
                  <option value="MANAGER">Manager</option>
                  <option value="RESPONDER">Responder</option>
                  <option value="VIEWER">Viewer</option>
                </select>
              </div>
              <button
                type="submit"
                className="bg-primary-600 text-white px-4 py-2 rounded hover:bg-primary-700"
                disabled={addLoading}
              >
                {addLoading ? 'Adding...' : 'Add User'}
              </button>
            </form>
          )}
        </div>
      )}

      <div className="card">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">
            {users.length} User{users.length !== 1 ? 's' : ''}
          </h2>
        </div>
        <div className="divide-y divide-gray-200">
          {users.length === 0 ? (
            <div className="px-6 py-8 text-center text-gray-500">
              No users found
            </div>
          ) : (
            <>
              {users.map((userItem) => (
                <div key={userItem.id} className="px-6 py-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-4">
                      <div className="flex-shrink-0">
                        <div className="h-10 w-10 rounded-full bg-gray-300 flex items-center justify-center">
                          <span className="text-sm font-medium text-gray-700">
                            {userItem.firstName && userItem.lastName ? 
                              `${userItem.firstName.charAt(0)}${userItem.lastName.charAt(0)}` : 
                              userItem.email.charAt(0).toUpperCase()}
                          </span>
                        </div>
                      </div>
                      <div>
                        <h3 className="text-sm font-medium text-gray-900">
                          {userItem.firstName && userItem.lastName ? `${userItem.firstName} ${userItem.lastName}` : userItem.email}
                        </h3>
                        <p className="text-sm text-gray-500">{userItem.email}</p>
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <span className={`px-2 py-1 text-xs font-medium rounded-full ${getRoleColor(userItem.role)}`}>
                        {userItem.role || 'VIEWER'}
                      </span>
                      <span className="text-sm text-gray-500">
                        Joined {userItem.createdAt ? new Date(userItem.createdAt).toLocaleDateString() : 'Unknown'}
                      </span>
                      {canDeleteUsers && userItem.role !== 'ADMIN' && (
                        <button
                          className="ml-2 bg-red-600 text-white px-3 py-1 rounded text-xs hover:bg-red-700"
                          onClick={() => handleDeleteUser(userItem.id)}
                        >
                          Delete
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

export default Users 