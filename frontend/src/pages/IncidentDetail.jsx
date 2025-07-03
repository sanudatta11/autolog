import React, { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import api from '../services/api'

function IncidentDetail() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [incident, setIncident] = useState(null)
  const [updates, setUpdates] = useState([])
  const [loading, setLoading] = useState(true)
  const [newUpdate, setNewUpdate] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    fetchIncidentData()
  }, [id])

  const fetchIncidentData = async () => {
    try {
      const [incidentResponse, updatesResponse] = await Promise.all([
        api.get(`/api/v1/incidents/${id}`),
        api.get(`/api/v1/incidents/${id}/updates`)
      ])
      
      setIncident(incidentResponse.data.data)
      setUpdates(updatesResponse.data.data || [])
    } catch (error) {
      console.error('Error fetching incident data:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSubmitUpdate = async (e) => {
    e.preventDefault()
    if (!newUpdate.trim()) return

    setSubmitting(true)
    try {
      await api.post(`/api/v1/incidents/${id}/updates`, {
        content: newUpdate,
        type: 'COMMENT'
      })
      
      setNewUpdate('')
      fetchIncidentData() // Refresh data
    } catch (error) {
      console.error('Error submitting update:', error)
    } finally {
      setSubmitting(false)
    }
  }

  const getStatusColor = (status) => {
    switch (status) {
      case 'OPEN': return 'bg-yellow-100 text-yellow-800'
      case 'IN_PROGRESS': return 'bg-blue-100 text-blue-800'
      case 'RESOLVED': return 'bg-green-100 text-green-800'
      case 'CLOSED': return 'bg-gray-100 text-gray-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  const getPriorityColor = (priority) => {
    switch (priority) {
      case 'CRITICAL': return 'bg-red-100 text-red-800'
      case 'HIGH': return 'bg-orange-100 text-orange-800'
      case 'MEDIUM': return 'bg-yellow-100 text-yellow-800'
      case 'LOW': return 'bg-green-100 text-green-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading incident...</div>
      </div>
    )
  }

  if (!incident) {
    return (
      <div className="text-center py-12">
        <h2 className="text-xl font-semibold text-gray-900">Incident not found</h2>
        <button
          onClick={() => navigate('/incidents')}
          className="btn btn-primary mt-4"
        >
          Back to Incidents
        </button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{incident.title}</h1>
          <p className="text-gray-600 mt-1">Incident #{incident.id}</p>
        </div>
        <div className="flex space-x-2">
          <span className={`px-3 py-1 text-sm font-medium rounded-full ${getStatusColor(incident.status)}`}>
            {incident.status}
          </span>
          <span className={`px-3 py-1 text-sm font-medium rounded-full ${getPriorityColor(incident.priority)}`}>
            {incident.priority}
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Content */}
        <div className="lg:col-span-2 space-y-6">
          {/* Incident Details */}
          <div className="card p-6">
            <h2 className="text-lg font-medium text-gray-900 mb-4">Description</h2>
            <p className="text-gray-700 whitespace-pre-wrap">{incident.description}</p>
          </div>

          {/* Updates */}
          <div className="card">
            <div className="px-6 py-4 border-b border-gray-200">
              <h2 className="text-lg font-medium text-gray-900">Updates</h2>
            </div>
            
            <div className="p-6">
              <form onSubmit={handleSubmitUpdate} className="mb-6">
                <textarea
                  value={newUpdate}
                  onChange={(e) => setNewUpdate(e.target.value)}
                  placeholder="Add an update or comment..."
                  rows={3}
                  className="input mb-3"
                />
                <div className="flex justify-end">
                  <button
                    type="submit"
                    disabled={submitting || !newUpdate.trim()}
                    className="btn btn-primary"
                  >
                    {submitting ? 'Posting...' : 'Post Update'}
                  </button>
                </div>
              </form>

              <div className="space-y-4">
                {updates.length === 0 ? (
                  <p className="text-gray-500 text-center py-4">No updates yet</p>
                ) : (
                  updates.map((update) => (
                    <div key={update.id} className="border-l-4 border-gray-200 pl-4">
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center space-x-2">
                          <span className="font-medium text-gray-900">
                            {update.user.firstName} {update.user.lastName}
                          </span>
                          <span className="text-sm text-gray-500">
                            {new Date(update.createdAt).toLocaleString()}
                          </span>
                        </div>
                        <span className="text-xs text-gray-500 uppercase">
                          {update.type}
                        </span>
                      </div>
                      <p className="text-gray-700">{update.content}</p>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Incident Info */}
          <div className="card p-6">
            <h3 className="text-lg font-medium text-gray-900 mb-4">Incident Information</h3>
            <dl className="space-y-3">
              <div>
                <dt className="text-sm font-medium text-gray-500">Status</dt>
                <dd className="text-sm text-gray-900">{incident.status}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Priority</dt>
                <dd className="text-sm text-gray-900">{incident.priority}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Severity</dt>
                <dd className="text-sm text-gray-900">{incident.severity}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Reporter</dt>
                <dd className="text-sm text-gray-900">
                  {incident.reporter.firstName} {incident.reporter.lastName}
                </dd>
              </div>
              {incident.assignee && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Assignee</dt>
                  <dd className="text-sm text-gray-900">
                    {incident.assignee.firstName} {incident.assignee.lastName}
                  </dd>
                </div>
              )}
              <div>
                <dt className="text-sm font-medium text-gray-500">Created</dt>
                <dd className="text-sm text-gray-900">
                  {new Date(incident.createdAt).toLocaleString()}
                </dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Last Updated</dt>
                <dd className="text-sm text-gray-900">
                  {new Date(incident.updatedAt).toLocaleString()}
                </dd>
              </div>
            </dl>
          </div>

          {/* Tags */}
          {incident.tags && incident.tags.length > 0 && (
            <div className="card p-6">
              <h3 className="text-lg font-medium text-gray-900 mb-4">Tags</h3>
              <div className="flex flex-wrap gap-2">
                {incident.tags.map((tag, index) => (
                  <span
                    key={index}
                    className="px-2 py-1 bg-gray-100 text-gray-700 text-sm rounded-md"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default IncidentDetail 