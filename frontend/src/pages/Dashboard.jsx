import React, { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import api from '../services/api'

function Dashboard() {
  const [stats, setStats] = useState({
    totalLogs: 0,
    analyzedLogs: 0,
    rcaReports: 0,
    anomalies: 0
  })
  const [recentAnalyses, setRecentAnalyses] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchDashboardData()
  }, [])

  const fetchDashboardData = async () => {
    try {
      const logsResponse = await api.get('/logs?limit=1000');
      const logs = logsResponse.data?.logFiles || [];
      setRecentAnalyses(logs.slice(0, 5));
      // Use real stats from logs
      const stats = {
        totalLogs: logs.length,
        analyzedLogs: logs.filter(l => l.status === 'completed').length,
        rcaReports: logs.filter(l => l.rcaAnalysisStatus === 'completed').length,
        anomalies: logs.filter(l => l.errorCount > 0).length
      };
      setStats(stats);
    } catch (error) {
      console.error('Error fetching dashboard data:', error);
    } finally {
      setLoading(false);
    }
  };

  const getAnalysisStatusColor = (status) => {
    switch (status) {
      case 'COMPLETED': return 'bg-green-100 text-green-800'
      case 'IN_PROGRESS': return 'bg-blue-100 text-blue-800'
      case 'FAILED': return 'bg-red-100 text-red-800'
      case 'PENDING': return 'bg-yellow-100 text-yellow-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  const getSeverityColor = (severity) => {
    switch (severity) {
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
        <div className="text-gray-500">Loading dashboard...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900">Log Analysis Dashboard</h1>
        <Link
          to="/logs"
          className="btn btn-primary"
        >
          Analyze Logs
        </Link>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div className="card p-6">
          <div className="flex items-center">
            <div className="p-2 bg-blue-100 rounded-lg">
              <span className="text-2xl">📊</span>
            </div>
            <div className="ml-4">
              <p className="text-sm font-medium text-gray-600">Total Logs</p>
              <p className="text-2xl font-semibold text-gray-900">{stats.totalLogs.toLocaleString()}</p>
            </div>
          </div>
        </div>

        <div className="card p-6">
          <div className="flex items-center">
            <div className="p-2 bg-green-100 rounded-lg">
              <span className="text-2xl">🔍</span>
            </div>
            <div className="ml-4">
              <p className="text-sm font-medium text-gray-600">Analyzed</p>
              <p className="text-2xl font-semibold text-gray-900">{stats.analyzedLogs.toLocaleString()}</p>
            </div>
          </div>
        </div>

        <div className="card p-6">
          <div className="flex items-center">
            <div className="p-2 bg-purple-100 rounded-lg">
              <span className="text-2xl">📋</span>
            </div>
            <div className="ml-4">
              <p className="text-sm font-medium text-gray-600">RCA Reports</p>
              <p className="text-2xl font-semibold text-gray-900">{stats.rcaReports}</p>
            </div>
          </div>
        </div>

        <div className="card p-6">
          <div className="flex items-center">
            <div className="p-2 bg-red-100 rounded-lg">
              <span className="text-2xl">⚠️</span>
            </div>
            <div className="ml-4">
              <p className="text-sm font-medium text-gray-600">Anomalies</p>
              <p className="text-2xl font-semibold text-gray-900">{stats.anomalies}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Log Analyses */}
      <div className="card">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">Recent Log Analyses</h2>
        </div>
        <div className="divide-y divide-gray-200">
          {recentAnalyses.length === 0 ? (
            <div className="px-6 py-4 text-center text-gray-500">
              No log analyses found
            </div>
          ) : (
            recentAnalyses.map((analysis) => (
              <div key={analysis.id} className="px-6 py-4">
                <div className="flex items-center justify-between">
                  <div className="flex-1">
                    <Link
                      to={`/logs/${analysis.id}`}
                      className="text-sm font-medium text-primary-600 hover:text-primary-700"
                    >
                      {analysis.filename || `Log Analysis ${analysis.id}`}
                    </Link>
                    <p className="text-sm text-gray-500 mt-1">
                      {analysis.source} • {analysis.logCount} logs • {analysis.analysisType}
                    </p>
                  </div>
                  <div className="flex items-center space-x-2">
                    <span className={`px-2 py-1 text-xs font-medium rounded-full ${getAnalysisStatusColor(analysis.analysisStatus)}`}>
                      {analysis.analysisStatus}
                    </span>
                    {analysis.severity && (
                      <span className={`px-2 py-1 text-xs font-medium rounded-full ${getSeverityColor(analysis.severity)}`}>
                        {analysis.severity}
                      </span>
                    )}
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
        {recentAnalyses.length > 0 && (
          <div className="px-6 py-3 border-t border-gray-200">
            <Link
              to="/logs"
              className="text-sm text-primary-600 hover:text-primary-700"
            >
              View all analyses →
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}

export default Dashboard 