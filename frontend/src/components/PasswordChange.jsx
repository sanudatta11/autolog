import React, { useState } from 'react'
import { passwordAPI } from '../services/api'

function PasswordChange() {
  const [formData, setFormData] = useState({
    currentPassword: '',
    newPassword: '',
    confirmPassword: ''
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const handleSubmit = async (e) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    setSuccess('')

    // Validate passwords match
    if (formData.newPassword !== formData.confirmPassword) {
      setError('New passwords do not match')
      setLoading(false)
      return
    }

    // Validate password length
    if (formData.newPassword.length < 6) {
      setError('New password must be at least 6 characters long')
      setLoading(false)
      return
    }

    try {
      await passwordAPI.changePassword(formData.currentPassword, formData.newPassword)
      setSuccess('Password changed successfully!')
      setFormData({
        currentPassword: '',
        newPassword: '',
        confirmPassword: ''
      })
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to change password')
    } finally {
      setLoading(false)
    }
  }

  const handleInputChange = (e) => {
    const { name, value } = e.target
    setFormData(prev => ({
      ...prev,
      [name]: value
    }))
    // Clear messages when user starts typing
    if (error) setError('')
    if (success) setSuccess('')
  }

  return (
    <div className="card">
      <div className="px-6 py-4 border-b border-gray-200">
        <h2 className="text-lg font-medium text-gray-900">Change Password</h2>
        <p className="text-sm text-gray-600 mt-1">
          Update your account password. Make sure to use a strong password.
        </p>
      </div>
      <div className="p-6">
        <form onSubmit={handleSubmit} className="space-y-4">
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
            <label htmlFor="currentPassword" className="block text-sm font-medium text-gray-700 mb-2">
              Current Password
            </label>
            <input
              type="password"
              id="currentPassword"
              name="currentPassword"
              required
              className="input w-full"
              value={formData.currentPassword}
              onChange={handleInputChange}
              placeholder="Enter your current password"
            />
          </div>

          <div>
            <label htmlFor="newPassword" className="block text-sm font-medium text-gray-700 mb-2">
              New Password
            </label>
            <input
              type="password"
              id="newPassword"
              name="newPassword"
              required
              className="input w-full"
              value={formData.newPassword}
              onChange={handleInputChange}
              placeholder="Enter your new password"
            />
            <p className="text-xs text-gray-500 mt-1">
              Password must be at least 6 characters long
            </p>
          </div>

          <div>
            <label htmlFor="confirmPassword" className="block text-sm font-medium text-gray-700 mb-2">
              Confirm New Password
            </label>
            <input
              type="password"
              id="confirmPassword"
              name="confirmPassword"
              required
              className="input w-full"
              value={formData.confirmPassword}
              onChange={handleInputChange}
              placeholder="Confirm your new password"
            />
          </div>

          <div className="flex justify-end pt-4">
            <button
              type="submit"
              className="bg-primary-600 text-white px-6 py-2 rounded hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed"
              disabled={loading || !formData.currentPassword || !formData.newPassword || !formData.confirmPassword}
            >
              {loading ? 'Changing Password...' : 'Change Password'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export default PasswordChange 