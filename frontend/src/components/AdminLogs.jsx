import React, { useState, useEffect } from 'react';
import api from '../services/api';

const AdminLogs = () => {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showLogs, setShowLogs] = useState(false);

  const fetchLogs = async () => {
    try {
      setLoading(true);
      setError('');
      const response = await api.get('/admin/logs');
      setLogs(response.data.logs);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to fetch logs');
    } finally {
      setLoading(false);
    }
  };

  const getLogLevelColor = (level) => {
    switch (level?.toLowerCase()) {
      case 'error': return 'text-red-600 bg-red-50';
      case 'warning': return 'text-yellow-600 bg-yellow-50';
      case 'info': return 'text-blue-600 bg-blue-50';
      case 'debug': return 'text-gray-600 bg-gray-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  const formatTimestamp = (timestamp) => {
    return new Date(timestamp).toLocaleString();
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Admin Logs</h3>
        <button
          onClick={() => {
            if (showLogs) {
              setShowLogs(false);
            } else {
              setShowLogs(true);
              fetchLogs();
            }
          }}
          className="btn btn-secondary"
        >
          {showLogs ? 'Hide Logs' : 'View Recent Logs'}
        </button>
      </div>

      {showLogs && (
        <div className="bg-white border border-gray-200 rounded-md">
          <div className="p-4 border-b border-gray-200">
            <div className="flex items-center justify-between">
              <h4 className="font-medium text-gray-900">Recent Log Entries</h4>
              <button
                onClick={fetchLogs}
                disabled={loading}
                className="btn btn-sm btn-outline"
              >
                {loading ? 'Refreshing...' : 'Refresh'}
              </button>
            </div>
          </div>

          {error && (
            <div className="p-4 bg-red-50 border-b border-red-200">
              <p className="text-red-800 text-sm">{error}</p>
            </div>
          )}

          <div className="max-h-96 overflow-y-auto">
            {loading ? (
              <div className="p-4 text-center">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600 mx-auto"></div>
                <p className="text-gray-600 text-sm mt-2">Loading logs...</p>
              </div>
            ) : logs.length === 0 ? (
              <div className="p-4 text-center text-gray-500">
                No logs found
              </div>
            ) : (
              <div className="divide-y divide-gray-200">
                {logs.map((log, index) => (
                  <div key={index} className="p-4 hover:bg-gray-50">
                    <div className="flex items-start justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center space-x-2 mb-1">
                          <span className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${getLogLevelColor(log.level)}`}>
                            {log.level || 'UNKNOWN'}
                          </span>
                          <span className="text-xs text-gray-500">
                            {formatTimestamp(log.timestamp)}
                          </span>
                        </div>
                        <p className="text-sm text-gray-900 font-mono break-all">
                          {log.message}
                        </p>
                        {log.source && (
                          <p className="text-xs text-gray-500 mt-1">
                            Source: {log.source}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default AdminLogs; 