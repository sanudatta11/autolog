import React, { useState, useEffect } from 'react';
import axios from 'axios';

const LLMAPICalls = () => {
  const [apiCalls, setApiCalls] = useState([]);
  const [selectedCall, setSelectedCall] = useState(null);
  const [loading, setLoading] = useState(true);
  const [logFileDetails, setLogFileDetails] = useState({});
  const [jobDetails, setJobDetails] = useState({});

  useEffect(() => {
    fetchAPICalls();
  }, []);

  const fetchAPICalls = async () => {
    try {
      const response = await axios.get('/admin/llm-api-calls', {
        headers: {
          Authorization: `Bearer ${localStorage.getItem('token')}`,
        },
      });
      setApiCalls(response.data.apiCalls);
    } catch (error) {
      console.error('Failed to fetch API calls:', error);
    } finally {
      setLoading(false);
    }
  };

  const fetchLogFileDetails = async (logFileId) => {
    if (logFileDetails[logFileId]) return logFileDetails[logFileId];
    
    try {
      const response = await axios.get(`/admin/log-files/${logFileId}`, {
        headers: {
          Authorization: `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const details = response.data.logFile;
      setLogFileDetails(prev => ({ ...prev, [logFileId]: details }));
      return details;
    } catch (error) {
      console.error('Failed to fetch log file details:', error);
      return null;
    }
  };

  const fetchJobDetails = async (jobId) => {
    if (jobDetails[jobId]) return jobDetails[jobId];
    
    try {
      const response = await axios.get(`/admin/jobs/${jobId}`, {
        headers: {
          Authorization: `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const details = response.data.job;
      setJobDetails(prev => ({ ...prev, [jobId]: details }));
      return details;
    } catch (error) {
      console.error('Failed to fetch job details:', error);
      return null;
    }
  };

  const clearAPICalls = async () => {
    try {
      await axios.delete('/admin/llm-api-calls', {
        headers: {
          Authorization: `Bearer ${localStorage.getItem('token')}`,
        },
      });
      setApiCalls([]);
    } catch (error) {
      console.error('Failed to clear API calls:', error);
    }
  };

  const formatDuration = (duration) => {
    const ms = parseInt(duration);
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  const getStatusText = (status) => {
    return status === 200 ? 'Success' : `Error (${status})`;
  };

  const truncateText = (text, maxLength) => {
    if (!text || text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
  };

  const getCallTypeBadgeClass = (callType) => {
    switch (callType) {
      case 'format_inference':
        return 'bg-primary text-white';
      case 'rca_analysis':
        return 'bg-info text-white';
      case 'rca_aggregation':
        return 'bg-success text-white';
      case 'embedding':
        return 'bg-warning text-white';
      case 'general':
        return 'bg-secondary text-white';
      default:
        return 'bg-secondary text-white';
    }
  };

  const handleLogFileClick = async (logFileId) => {
    const details = await fetchLogFileDetails(logFileId);
    if (details) {
      alert(`Log File Details:\nFilename: ${details.filename}\nSize: ${details.size} bytes\nStatus: ${details.status}\nUploaded: ${new Date(details.createdAt).toLocaleString()}`);
    }
  };

  const handleJobClick = async (jobId) => {
    const details = await fetchJobDetails(jobId);
    if (details) {
      alert(`Job Details:\nStatus: ${details.status}\nType: ${details.type}\nCreated: ${new Date(details.createdAt).toLocaleString()}\nCompleted: ${details.completedAt ? new Date(details.completedAt).toLocaleString() : 'Not completed'}`);
    }
  };

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-900">LLM API Calls</h1>
        <button
          onClick={clearAPICalls}
          className="bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded"
        >
          Clear All
        </button>
      </div>

      {loading ? (
        <div className="text-center py-8">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
          <p className="mt-4 text-gray-600">Loading API calls...</p>
        </div>
      ) : (
        <div className="bg-white shadow overflow-hidden sm:rounded-md">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Timestamp
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Call Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Model
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Duration
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Log File
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Job ID
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {Array.isArray(apiCalls) && apiCalls.map((call) => (
                <tr key={call.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    {new Date(call.timestamp).toLocaleString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${getCallTypeBadgeClass(call.callType)}`}>
                      {call.callType || 'general'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    {call.model}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${call.status === 200 ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}>
                      {getStatusText(call.status)}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    {formatDuration(call.duration)}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-900">
                    {call.logFileId ? (
                     <button
                       onClick={() => handleLogFileClick(call.logFileId)}
                       className="text-blue-600 hover:text-blue-800 underline cursor-pointer"
                     >
                       {call.logFileId}
                     </button>
                    ) : (
                      <span className="text-muted">-</span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-900">
                    {call.jobId ? (
                     <button
                       onClick={() => handleJobClick(call.jobId)}
                       className="text-blue-600 hover:text-blue-800 underline cursor-pointer"
                     >
                       {call.jobId}
                     </button>
                    ) : (
                      <span className="text-muted">-</span>
                    )}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    <button
                      onClick={() => setSelectedCall(call)}
                      className="text-blue-600 hover:text-blue-800 font-medium"
                    >
                      View Details
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Modal for detailed view */}
      {selectedCall && (
        <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50">
          <div className="relative top-20 mx-auto p-5 border w-11/12 md:w-3/4 lg:w-1/2 shadow-lg rounded-md bg-white">
            <div className="mt-3">
              <div className="flex justify-between items-center mb-4">
                <h3 className="text-lg font-medium text-gray-900">API Call Details</h3>
                <button
                  onClick={() => setSelectedCall(null)}
                  className="text-gray-400 hover:text-gray-600"
                >
                  <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
              
              <div className="space-y-3 text-sm">
                <div><strong>ID:</strong> {selectedCall.id}</div>
                <div><strong>Timestamp:</strong> {new Date(selectedCall.timestamp).toLocaleString()}</div>
                <div><strong>Call Type:</strong> {selectedCall.callType || 'general'}</div>
                <div><strong>Model:</strong> {selectedCall.model}</div>
                <div><strong>Status:</strong> {getStatusText(selectedCall.status)}</div>
                <div><strong>Duration:</strong> {formatDuration(selectedCall.duration)}</div>
                <div><strong>Log File:</strong> {selectedCall.logFileId ? <span className="text-blue-600">{selectedCall.logFileId}</span> : <span className="text-muted">-</span>}</div>
                <div><strong>Job ID:</strong> {selectedCall.jobId ? <span className="text-info">{selectedCall.jobId}</span> : <span className="text-muted">-</span>}</div>
                <div><strong>Payload:</strong></div>
                <pre className="bg-gray-100 p-3 rounded text-xs overflow-x-auto">{JSON.stringify(selectedCall.payload, null, 2)}</pre>
                <div><strong>Response:</strong></div>
                <pre className="bg-gray-100 p-3 rounded text-xs overflow-x-auto">{selectedCall.response}</pre>
                {selectedCall.error && (
                  <>
                    <div><strong>Error:</strong></div>
                    <pre className="bg-red-100 p-3 rounded text-xs overflow-x-auto text-red-800">{selectedCall.error}</pre>
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default LLMAPICalls; 