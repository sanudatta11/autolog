import React, { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import api from '../services/api';

const Logs = () => {
  const { token } = useAuth();
  const [logFiles, setLogFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState(null);
  const [message, setMessage] = useState('');
  const [selectedLogFile, setSelectedLogFile] = useState(null);
  const [logEntries, setLogEntries] = useState([]);
  const [analyses, setAnalyses] = useState([]);
  const [showAllEntries, setShowAllEntries] = useState(false);
  const [llmModalOpen, setLlmModalOpen] = useState(false);
  const [llmModalAnalysis, setLlmModalAnalysis] = useState(null);
  const [llmModalLogFile, setLlmModalLogFile] = useState(null);

  useEffect(() => {
    fetchLogFiles();
  }, []);

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
      setMessage('Analysis completed: ' + response.data.message);
      
      // Fetch analyses
      const analysesResponse = await api.get(`/logs/${logFileId}/analyses`);
      setAnalyses(analysesResponse.data.analyses || []);
    } catch (error) {
      setMessage('Analysis failed: ' + error.message);
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
        setAnalyses([]);
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

  // Fetch and show LLM analysis modal
  const handleShowLLMAnalysis = async (logFile) => {
    try {
      const response = await api.get(`/logs/${logFile.id}/analyses`);
      const analyses = response.data.analyses || [];
      if (analyses.length > 0) {
        setLlmModalAnalysis(analyses[0]); // Show the latest analysis
        setLlmModalLogFile(logFile);
        setLlmModalOpen(true);
      } else {
        setLlmModalAnalysis(null);
        setLlmModalLogFile(logFile);
        setLlmModalOpen(true);
      }
    } catch (error) {
      setLlmModalAnalysis({ error: 'Failed to fetch LLM analysis: ' + error.message });
      setLlmModalLogFile(logFile);
      setLlmModalOpen(true);
    }
  };

  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold mb-8">Log Analysis</h1>

      {/* Upload Section */}
      <div className="bg-white rounded-lg shadow-md p-6 mb-8">
        <h2 className="text-xl font-semibold mb-4">Upload Log File</h2>
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

      {/* Log Files List */}
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-xl font-semibold mb-4">Log Files</h2>
        
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
                    {logFile.status === 'completed' && (
                      <button
                        onClick={() => handleAnalyze(logFile.id)}
                        className="bg-green-600 text-white px-3 py-1 rounded text-sm hover:bg-green-700"
                      >
                        Analyze
                      </button>
                    )}
                    <button
                      onClick={() => handleShowLLMAnalysis(logFile)}
                      className="bg-purple-600 text-white px-3 py-1 rounded text-sm hover:bg-purple-700"
                    >
                      LLM Analysis
                    </button>
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
          
                     {/* Analyses */}
           {analyses.length > 0 && (
             <div className="mb-6">
               <h3 className="text-lg font-medium mb-3">Analyses</h3>
               {analyses.map((analysis) => (
                 <div key={analysis.id} className="border border-gray-200 rounded p-4 mb-3">
                   <div className="flex items-center justify-between mb-2">
                     <div className="flex items-center space-x-4">
                       <span className="font-medium">Severity: {analysis.severity}</span>
                       {analysis.metadata?.aiGenerated && (
                         <span className="bg-blue-100 text-blue-800 text-xs px-2 py-1 rounded">
                           AI Generated
                         </span>
                       )}
                     </div>
                     <span className="text-sm text-gray-600">
                       {new Date(analysis.createdAt).toLocaleString()}
                     </span>
                   </div>
                   <p className="text-gray-700 mb-3">{analysis.summary}</p>
                   {/* DETAILED ERROR ANALYSIS */}
                   {Array.isArray(analysis.metadata?.errorAnalysis) && analysis.metadata.errorAnalysis.length > 0 && (
                     <div className="mb-3">
                       <h4 className="font-medium text-sm text-gray-800 mb-1">Error Patterns & RCA:</h4>
                       <div className="space-y-2">
                         {analysis.metadata.errorAnalysis.map((err, idx) => (
                           <div key={idx} className="border border-gray-100 rounded p-2 bg-gray-50">
                             <div className="flex flex-wrap gap-2 items-center mb-1">
                               <span className="font-semibold text-red-700">{err.errorPattern}</span>
                               <span className="text-xs text-gray-600">Count: {err.errorCount}</span>
                               <span className="text-xs text-gray-600">Severity: {err.severity}</span>
                               <span className="text-xs text-gray-600">First: {err.firstOccurrence}</span>
                               <span className="text-xs text-gray-600">Last: {err.lastOccurrence}</span>
                             </div>
                             <div className="text-xs text-gray-700 mb-1"><b>Root Cause:</b> {err.rootCause}</div>
                             <div className="text-xs text-gray-700 mb-1"><b>Impact:</b> {err.impact}</div>
                             <div className="text-xs text-gray-700 mb-1"><b>Fix:</b> {err.fix}</div>
                             {err.relatedErrors && err.relatedErrors.length > 0 && (
                               <div className="text-xs text-gray-500"><b>Related Errors:</b> {err.relatedErrors.join(', ')}</div>
                             )}
                           </div>
                         ))}
                       </div>
                     </div>
                   )}
                   {/* ... existing code for root cause, recommendations, etc ... */}
                   {analysis.metadata?.rootCause && (
                     <div className="mb-3">
                       <h4 className="font-medium text-sm text-gray-800 mb-1">Root Cause:</h4>
                       <p className="text-sm text-gray-600">{analysis.metadata.rootCause}</p>
                     </div>
                   )}
                   {analysis.metadata?.recommendations && analysis.metadata.recommendations.length > 0 && (
                     <div className="mb-3">
                       <h4 className="font-medium text-sm text-gray-800 mb-1">Recommendations:</h4>
                       <ul className="text-sm text-gray-600 list-disc list-inside">
                         {analysis.metadata.recommendations.map((rec, index) => (
                           <li key={index}>{rec}</li>
                         ))}
                       </ul>
                     </div>
                   )}
                   {analysis.metadata?.incidentType && (
                     <div className="mb-3">
                       <h4 className="font-medium text-sm text-gray-800 mb-1">Incident Type:</h4>
                       <p className="text-sm text-gray-600">{analysis.metadata.incidentType}</p>
                     </div>
                   )}
                   <div className="text-sm text-gray-600 mt-2">
                     Errors: {analysis.errorCount} | Warnings: {analysis.warningCount}
                   </div>
                 </div>
               ))}
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
              LLM Analysis for: {llmModalLogFile?.filename}
            </h2>
            {llmModalAnalysis?.error ? (
              <div className="text-red-600">{llmModalAnalysis.error}</div>
            ) : llmModalAnalysis ? (
              <div>
                <div className="mb-2">
                  <span className="font-medium">Severity:</span> {llmModalAnalysis.severity}
                </div>
                <div className="mb-2">
                  <span className="font-medium">Summary:</span> {llmModalAnalysis.summary}
                </div>
                {llmModalAnalysis.metadata?.errorAnalysis && llmModalAnalysis.metadata.errorAnalysis.length > 0 && (
                  <div className="mb-3">
                    <h4 className="font-medium text-sm text-gray-800 mb-1">Error Patterns & RCA:</h4>
                    <div className="space-y-2">
                      {llmModalAnalysis.metadata.errorAnalysis.map((err, idx) => (
                        <div key={idx} className="border border-gray-100 rounded p-2 bg-gray-50">
                          <div className="flex flex-wrap gap-2 items-center mb-1">
                            <span className="font-semibold text-red-700">{err.errorPattern}</span>
                            <span className="text-xs text-gray-600">Count: {err.errorCount}</span>
                            <span className="text-xs text-gray-600">Severity: {err.severity}</span>
                            <span className="text-xs text-gray-600">First: {err.firstOccurrence}</span>
                            <span className="text-xs text-gray-600">Last: {err.lastOccurrence}</span>
                          </div>
                          <div className="text-xs text-gray-700 mb-1"><b>Root Cause:</b> {err.rootCause}</div>
                          <div className="text-xs text-gray-700 mb-1"><b>Impact:</b> {err.impact}</div>
                          <div className="text-xs text-gray-700 mb-1"><b>Fix:</b> {err.fix}</div>
                          {err.relatedErrors && err.relatedErrors.length > 0 && (
                            <div className="text-xs text-gray-500"><b>Related Errors:</b> {err.relatedErrors.join(', ')}</div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {llmModalAnalysis.metadata?.rootCause && (
                  <div className="mb-2">
                    <span className="font-medium">Root Cause:</span> {llmModalAnalysis.metadata.rootCause}
                  </div>
                )}
                {llmModalAnalysis.metadata?.recommendations && llmModalAnalysis.metadata.recommendations.length > 0 && (
                  <div className="mb-2">
                    <span className="font-medium">Recommendations:</span>
                    <ul className="list-disc list-inside text-sm text-gray-700">
                      {llmModalAnalysis.metadata.recommendations.map((rec, idx) => (
                        <li key={idx}>{rec}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {llmModalAnalysis.metadata?.incidentType && (
                  <div className="mb-2">
                    <span className="font-medium">Incident Type:</span> {llmModalAnalysis.metadata.incidentType}
                  </div>
                )}
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