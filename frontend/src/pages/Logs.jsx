import React, { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import api from '../services/api';
import RCAnalysis from '../components/RCAnalysis';
import AdminLogs from '../components/AdminLogs';

const Logs = () => {
  const { token } = useAuth();
  const [logFiles, setLogFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState(null);
  const [message, setMessage] = useState('');
  const [selectedLogFile, setSelectedLogFile] = useState(null);
  const [logEntries, setLogEntries] = useState([]);
  const [showAllEntries, setShowAllEntries] = useState(false);
  const [llmModalOpen, setLlmModalOpen] = useState(false);
  const [llmModalAnalysis, setLlmModalAnalysis] = useState(null);
  const [llmModalLogFile, setLlmModalLogFile] = useState(null);

  const [userRole, setUserRole] = useState('');

  useEffect(() => {
    fetchLogFiles();
    fetchUserRole();
  }, []);

  const fetchUserRole = async () => {
    try {
      const response = await api.get('/users/me');
      setUserRole(response.data.user.role);
    } catch (error) {
      console.error('Failed to fetch user role:', error);
    }
  };

  const fetchLogFiles = async () => {
    setLoading(true);
    try {
      const response = await api.get('/logs');
      setLogFiles(response.data.logFiles || []);
    } catch (error) {
      setMessage('Failed to fetch log files: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  const handleFileSelect = (event) => {
    const file = event.target.files[0];
    if (file) {
      // Validate file type
      const allowedTypes = ['.json', '.log', '.txt'];
      const fileExtension = file.name.toLowerCase().substring(file.name.lastIndexOf('.'));
      
      if (!allowedTypes.includes(fileExtension)) {
        setMessage('Please select a JSON, LOG, or TXT file');
        return;
      }
      
      setSelectedFile(file);
      setMessage('');
    }
  };

  const handleUpload = async () => {
    if (!selectedFile) {
      setMessage('Please select a file to upload');
      return;
    }

    setUploading(true);
    const formData = new FormData();
    formData.append('logfile', selectedFile);

    try {
      const response = await api.post('/logs/upload', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      
      setMessage('Log file uploaded successfully! Processing in background...');
      setSelectedFile(null);
      document.getElementById('file-input').value = '';
      
      // Refresh log files list
      setTimeout(fetchLogFiles, 2000);
    } catch (error) {
      setMessage('Upload failed: ' + error.message);
    } finally {
      setUploading(false);
    }
  };

  const handleViewLogFile = async (logFile) => {
    try {
      const response = await api.get(`/logs/${logFile.id}`);
      setSelectedLogFile(response.data.logFile);
      setLogEntries(response.data.logFile.entries || []);
    } catch (error) {
      setMessage('Failed to fetch log file details: ' + error.message);
    }
  };

  const handleAnalyze = async (logFileId) => {
    try {
      const response = await api.post(`/logs/${logFileId}/analyze`);
      setMessage('RCA analysis started: ' + response.data.message);
      
      // Refresh log files to show updated status
      setTimeout(fetchLogFiles, 1000);
    } catch (error) {
      setMessage('Analysis failed: ' + (error.response?.data?.error || error.message));
    }
  };

  const handleDelete = async (logFileId) => {
    if (!window.confirm('Are you sure you want to delete this log file?')) {
      return;
    }

    try {
      await api.delete(`/logs/${logFileId}`);
      setMessage('Log file deleted successfully');
      fetchLogFiles();
      
      if (selectedLogFile && selectedLogFile.id === logFileId) {
        setSelectedLogFile(null);
        setLogEntries([]);
      }
    } catch (error) {
      setMessage('Failed to delete log file: ' + error.message);
    }
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed': return 'text-green-600';
      case 'processing': return 'text-yellow-600';
      case 'failed': return 'text-red-600';
      default: return 'text-gray-600';
    }
  };

  const getLevelColor = (level) => {
    switch (level) {
      case 'ERROR':
      case 'FATAL': return 'text-red-600';
      case 'WARN': return 'text-yellow-600';
      case 'INFO': return 'text-blue-600';
      case 'DEBUG': return 'text-gray-600';
      default: return 'text-gray-600';
    }
  };

  // Helper to filter error/fatal entries
  const getErrorEntries = (entries) => entries.filter(e => e.level === 'ERROR' || e.level === 'FATAL');

  // Fetch and show RCA analysis modal
  const handleShowLLMAnalysis = async (logFile) => {
    try {
      // Check if RCA analysis is completed
      if (logFile.rcaAnalysisStatus === 'completed') {
        const response = await api.get(`/logs/${logFile.id}/rca-results`);
        setLlmModalAnalysis(response.data.analysis);
        setLlmModalLogFile(logFile);
        setLlmModalOpen(true);
      } else {
        setLlmModalAnalysis({ error: 'RCA analysis not completed yet' });
        setLlmModalLogFile(logFile);
        setLlmModalOpen(true);
      }
    } catch (error) {
      setLlmModalAnalysis({ error: 'Failed to fetch RCA analysis: ' + error.message });
      setLlmModalLogFile(logFile);
      setLlmModalOpen(true);
    }
  };



  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold mb-8">Log Analysis & RCA</h1>

      {/* Log Sources Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* File Upload */}
        <div className="bg-white rounded-lg shadow-md p-6">
          <h2 className="text-xl font-semibold mb-4">📁 File Upload</h2>
          <div className="flex items-center space-x-4">
            <input
              id="file-input"
              type="file"
              accept=".json,.log,.txt"
              onChange={handleFileSelect}
              className="border border-gray-300 rounded px-3 py-2 flex-1"
            />
            <button
              onClick={handleUpload}
              disabled={uploading || !selectedFile}
              className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700 disabled:bg-gray-400"
            >
              {uploading ? 'Uploading...' : 'Upload'}
            </button>
          </div>
          {message && (
            <div className="mt-4 p-3 bg-blue-100 text-blue-800 rounded">
              {message}
            </div>
          )}
        </div>

        {/* Log Connectors */}
        <div className="bg-white rounded-lg shadow-md p-6">
          <h2 className="text-xl font-semibold mb-4">🔗 Log Connectors</h2>
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 border border-gray-200 rounded">
              <div className="flex items-center">
                <span className="text-2xl mr-3">☁️</span>
                <div>
                  <h3 className="font-medium">CloudWatch</h3>
                  <p className="text-sm text-gray-600">AWS Logs Integration</p>
                </div>
              </div>
              <button className="bg-orange-600 text-white px-4 py-2 rounded text-sm hover:bg-orange-700">
                Configure
              </button>
            </div>
            
            <div className="flex items-center justify-between p-3 border border-gray-200 rounded">
              <div className="flex items-center">
                <span className="text-2xl mr-3">🔍</span>
                <div>
                  <h3 className="font-medium">Splunk</h3>
                  <p className="text-sm text-gray-600">Enterprise Logs</p>
                </div>
              </div>
              <button className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700">
                Configure
              </button>
            </div>
            
            <div className="flex items-center justify-between p-3 border border-gray-200 rounded">
              <div className="flex items-center">
                <span className="text-2xl mr-3">📊</span>
                <div>
                  <h3 className="font-medium">Elasticsearch</h3>
                  <p className="text-sm text-gray-600">Search & Analytics</p>
                </div>
              </div>
              <button className="bg-green-600 text-white px-4 py-2 rounded text-sm hover:bg-green-700">
                Configure
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Log Files List */}
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-xl font-semibold mb-4">📋 Log Analysis History</h2>
        
        {loading ? (
          <div className="text-center py-8">Loading...</div>
        ) : logFiles.length === 0 ? (
          <div className="text-center py-8 text-gray-500">No log files uploaded yet</div>
        ) : (
          <div className="space-y-4">
            {logFiles.map((logFile) => (
              <div key={logFile.id} className="border border-gray-200 rounded-lg p-4">
                <div className="flex items-center justify-between">
                  <div className="flex-1">
                    <h3 className="font-semibold">{logFile.filename}</h3>
                    <div className="text-sm text-gray-600 mt-1">
                      <span>Size: {(logFile.size / 1024).toFixed(2)} KB</span>
                      <span className="mx-2">•</span>
                      <span>Entries: {logFile.entryCount}</span>
                      <span className="mx-2">•</span>
                      <span>Errors: {logFile.errorCount}</span>
                      <span className="mx-2">•</span>
                      <span>Warnings: {logFile.warningCount}</span>
                      <span className="mx-2">•</span>
                      <span className={getStatusColor(logFile.status)}>
                        Status: {logFile.status}
                      </span>
                      <span className="mx-2">•</span>
                      <span className={getStatusColor(logFile.rcaAnalysisStatus)}>
                        RCA: {logFile.rcaAnalysisStatus || 'not_started'}
                      </span>
                    </div>
                    <div className="text-xs text-gray-500 mt-1">
                      Uploaded: {new Date(logFile.createdAt).toLocaleString()}
                    </div>
                  </div>
                  <div className="flex space-x-2">
                    <button
                      onClick={() => handleViewLogFile(logFile)}
                      className="bg-gray-600 text-white px-3 py-1 rounded text-sm hover:bg-gray-700"
                    >
                      View
                    </button>
                    {logFile.status === 'completed' && logFile.rcaAnalysisStatus !== 'pending' && logFile.rcaAnalysisStatus !== 'running' && (
                      <button
                        onClick={() => handleAnalyze(logFile.id)}
                        className="bg-green-600 text-white px-3 py-1 rounded text-sm hover:bg-green-700"
                      >
                        Generate RCA
                      </button>
                    )}
                    {(logFile.rcaAnalysisStatus === 'pending' || logFile.rcaAnalysisStatus === 'running') && (
                      <button
                        disabled
                        className="bg-gray-400 text-white px-3 py-1 rounded text-sm cursor-not-allowed"
                      >
                        RCA Running...
                      </button>
                    )}
                    {logFile.rcaAnalysisStatus === 'completed' && (
                      <button
                        onClick={() => handleShowLLMAnalysis(logFile)}
                        className="bg-purple-600 text-white px-3 py-1 rounded text-sm hover:bg-purple-700"
                      >
                        View RCA
                      </button>
                    )}
                    <button
                      onClick={() => handleDelete(logFile.id)}
                      className="bg-red-600 text-white px-3 py-1 rounded text-sm hover:bg-red-700"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Log File Details */}
      {selectedLogFile && (
        <div className="bg-white rounded-lg shadow-md p-6 mt-8">
          <h2 className="text-xl font-semibold mb-4">
            Log Details: {selectedLogFile.filename}
          </h2>
          
          {/* RCA Analysis Component */}
          <div className="mb-6">
            <RCAnalysis 
              logFileId={selectedLogFile.id} 
              initialStatus={selectedLogFile.rcaAnalysisStatus || 'idle'}
              onAnalysisComplete={(results) => {
                setMessage('RCA analysis completed successfully!');
                fetchLogFiles();
              }}
            />
          </div>

          {/* Admin Logs Component */}
          {userRole === 'ADMIN' && (
            <div className="mb-6">
              <AdminLogs />
            </div>
          )}
          
           

          {/* Log Entries Toggle */}
          <div className="flex items-center mb-2">
            <h3 className="text-lg font-medium mr-4">Log Entries</h3>
            <label className="flex items-center text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={showAllEntries}
                onChange={() => setShowAllEntries((v) => !v)}
                className="mr-1"
              />
              Show all entries
            </label>
          </div>
          <div className="max-h-96 overflow-y-auto">
            {(showAllEntries ? logEntries : getErrorEntries(logEntries)).map((entry) => (
              <div key={entry.id} className="border-b border-gray-100 py-2">
                <div className="flex items-start space-x-3">
                  <span className={`font-mono text-sm ${getLevelColor(entry.level)}`}>
                    {entry.level}
                  </span>
                  <span className="text-sm text-gray-600 font-mono">
                    {new Date(entry.timestamp).toLocaleString()}
                  </span>
                  <span className="text-sm flex-1">{entry.message}</span>
                </div>
                {entry.metadata && Object.keys(entry.metadata).length > 0 && (
                  <div className="ml-8 mt-1">
                    <pre className="text-xs text-gray-500 bg-gray-50 p-2 rounded">
                      {JSON.stringify(entry.metadata, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* LLM Analysis Modal */}
      {llmModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-40">
          <div className="bg-white rounded-lg shadow-lg p-6 w-full max-w-2xl relative">
            <button
              onClick={() => setLlmModalOpen(false)}
              className="absolute top-2 right-2 text-gray-500 hover:text-gray-800 text-2xl"
              aria-label="Close"
            >
              &times;
            </button>
            <h2 className="text-xl font-semibold mb-4">
              Root Cause Analysis for: {llmModalLogFile?.filename}
            </h2>
            {llmModalAnalysis?.error ? (
              <div className="text-red-600">{llmModalAnalysis.error}</div>
            ) : llmModalAnalysis ? (
              <div className="space-y-4">
                {(() => {
                  const analysis = llmModalAnalysis.analysis || llmModalAnalysis;
                  return (
                    <>
                      {/* Summary & Severity */}
                      <div>
                        <h3 className="font-medium text-lg mb-1">Summary</h3>
                        <p className="text-gray-800 mb-2">{analysis.summary}</p>
                        <span className={`inline-block px-3 py-1 rounded-full text-xs font-semibold ${
                          analysis.severity === 'critical' ? 'bg-red-100 text-red-800' :
                          analysis.severity === 'high' ? 'bg-orange-100 text-orange-800' :
                          analysis.severity === 'medium' ? 'bg-yellow-100 text-yellow-800' :
                          'bg-green-100 text-green-800'
                        }`}>
                          Severity: {analysis.severity}
                        </span>
                      </div>

                      {/* Root Cause */}
                      {analysis.rootCause && (
                        <div>
                          <h3 className="font-medium text-lg mb-1 mt-4">Root Cause</h3>
                          <p className="text-gray-800">{analysis.rootCause}</p>
                        </div>
                      )}

                      {/* Recommendations */}
                      {Array.isArray(analysis.recommendations) && analysis.recommendations.length > 0 && (
                        <div>
                          <h3 className="font-medium text-lg mb-1 mt-4">Recommendations</h3>
                          <ul className="list-disc list-inside text-gray-800">
                            {analysis.recommendations.map((rec, idx) => (
                              <li key={idx}>{rec}</li>
                            ))}
                          </ul>
                        </div>
                      )}

                      {/* Error Counts */}
                      <div className="flex space-x-6 mt-4">
                        {typeof analysis.criticalErrors === 'number' && (
                          <div className="text-red-700 font-semibold">Critical Errors: {analysis.criticalErrors}</div>
                        )}
                        {typeof analysis.nonCriticalErrors === 'number' && (
                          <div className="text-yellow-700 font-semibold">Non-critical Errors: {analysis.nonCriticalErrors}</div>
                        )}
                        {typeof analysis.errorCount === 'number' && (
                          <div className="text-gray-700 font-semibold">Total Errors: {analysis.errorCount}</div>
                        )}
                        {typeof analysis.warningCount === 'number' && (
                          <div className="text-blue-700 font-semibold">Warnings: {analysis.warningCount}</div>
                        )}
                      </div>

                      {/* Error Analysis Table */}
                      {Array.isArray(analysis.errorAnalysis) && analysis.errorAnalysis.length > 0 && (
                        <div className="mt-6">
                          <h3 className="font-medium text-lg mb-2">Error Analysis</h3>
                          <div className="overflow-x-auto">
                            <table className="min-w-full border text-xs">
                              <thead>
                                <tr className="bg-gray-100">
                                  <th className="px-2 py-1 border">Pattern</th>
                                  <th className="px-2 py-1 border">Count</th>
                                  <th className="px-2 py-1 border">Severity</th>
                                  <th className="px-2 py-1 border">First</th>
                                  <th className="px-2 py-1 border">Last</th>
                                  <th className="px-2 py-1 border">Root Cause</th>
                                  <th className="px-2 py-1 border">Impact</th>
                                  <th className="px-2 py-1 border">Fix</th>
                                  <th className="px-2 py-1 border">Related</th>
                                </tr>
                              </thead>
                              <tbody>
                                {analysis.errorAnalysis.map((err, idx) => (
                                  <tr key={idx} className="border-b">
                                    <td className="px-2 py-1 border font-mono">{err.errorPattern}</td>
                                    <td className="px-2 py-1 border text-center">{err.errorCount}</td>
                                    <td className={`px-2 py-1 border text-center ${err.severity === 'critical' ? 'text-red-700' : 'text-yellow-700'}`}>{err.severity}</td>
                                    <td className="px-2 py-1 border font-mono">{err.firstOccurrence}</td>
                                    <td className="px-2 py-1 border font-mono">{err.lastOccurrence}</td>
                                    <td className="px-2 py-1 border">{err.rootCause}</td>
                                    <td className="px-2 py-1 border">{err.impact}</td>
                                    <td className="px-2 py-1 border">{err.fix}</td>
                                    <td className="px-2 py-1 border">
                                      {Array.isArray(err.relatedErrors) && err.relatedErrors.length > 0 ? (
                                        <ul className="list-disc list-inside">
                                          {err.relatedErrors.map((rel, i) => (
                                            <li key={i}>{rel}</li>
                                          ))}
                                        </ul>
                                      ) : (
                                        <span className="text-gray-400">-</span>
                                      )}
                                    </td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          </div>
                        </div>
                      )}
                    </>
                  );
                })()}
              </div>
            ) : (
              <div>No analysis found for this log file.</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default Logs; 