import React, { useState, useEffect } from 'react';
import api from '../services/api';

const RCAnalysis = ({ logFileId, onAnalysisComplete }) => {
  const [jobId, setJobId] = useState(null);
  const [status, setStatus] = useState('idle'); // idle, pending, running, completed, failed
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState('');
  const [results, setResults] = useState(null);
  const [polling, setPolling] = useState(false);

  const startAnalysis = async () => {
    try {
      setStatus('pending');
      setError('');
      setProgress(0);
      
      const response = await api.post(`/logs/${logFileId}/analyze`);
      setJobId(response.data.jobId);
      setStatus('pending');
      setPolling(true);
      
      // Start polling for status
      pollJobStatus(response.data.jobId);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to start RCA analysis');
      setStatus('failed');
    }
  };

  const pollJobStatus = async (jobId) => {
    if (!polling) return;

    try {
      const response = await api.get(`/jobs/${jobId}/status`);
      const job = response.data.job;
      
      setStatus(job.status);
      setProgress(job.progress);
      
      if (job.status === 'completed') {
        setPolling(false);
        setResults(job.result);
        if (onAnalysisComplete) {
          onAnalysisComplete(job.result);
        }
      } else if (job.status === 'failed') {
        setPolling(false);
        setError(job.error || 'RCA analysis failed');
      } else {
        // Continue polling
        setTimeout(() => pollJobStatus(jobId), 2000);
      }
    } catch (err) {
      setPolling(false);
      setError('Failed to check job status');
      setStatus('failed');
    }
  };

  const getResults = async () => {
    try {
      const response = await api.get(`/logs/${logFileId}/rca-results`);
      setResults(response.data.analysis);
      setStatus('completed');
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to fetch RCA results');
    }
  };

  const getStatusColor = () => {
    switch (status) {
      case 'completed': return 'text-green-600';
      case 'running': return 'text-blue-600';
      case 'pending': return 'text-yellow-600';
      case 'failed': return 'text-red-600';
      default: return 'text-gray-600';
    }
  };

  const getStatusText = () => {
    switch (status) {
      case 'completed': return 'Completed';
      case 'running': return 'Running';
      case 'pending': return 'Pending';
      case 'failed': return 'Failed';
      default: return 'Not Started';
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Root Cause Analysis</h3>
        <div className="flex items-center space-x-2">
          <span className={`text-sm font-medium ${getStatusColor()}`}>
            {getStatusText()}
          </span>
          {status === 'running' && (
            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></div>
          )}
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-md p-3">
          <p className="text-red-800 text-sm">{error}</p>
        </div>
      )}

      {status === 'idle' && (
        <div className="bg-gray-50 border border-gray-200 rounded-md p-4">
          <p className="text-gray-600 text-sm mb-3">
            Start a Root Cause Analysis to get detailed insights into your log file.
          </p>
          <button
            onClick={startAnalysis}
            className="btn btn-primary"
          >
            Start RCA Analysis
          </button>
        </div>
      )}

      {(status === 'pending' || status === 'running') && (
        <div className="bg-blue-50 border border-blue-200 rounded-md p-4">
          <div className="flex items-center justify-between mb-2">
            <p className="text-blue-800 text-sm font-medium">
              RCA Analysis in Progress...
            </p>
            <span className="text-blue-600 text-sm">{progress}%</span>
          </div>
          <div className="w-full bg-blue-200 rounded-full h-2">
            <div 
              className="bg-blue-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            ></div>
          </div>
          <p className="text-blue-700 text-xs mt-2">
            This may take a few minutes. You can continue using the application.
          </p>
        </div>
      )}

      {status === 'completed' && !results && (
        <div className="bg-green-50 border border-green-200 rounded-md p-4">
          <p className="text-green-800 text-sm mb-3">
            RCA Analysis completed successfully!
          </p>
          <button
            onClick={getResults}
            className="btn btn-primary"
          >
            View Results
          </button>
        </div>
      )}

      {results && (
        <div className="bg-white border border-gray-200 rounded-md p-4">
          <h4 className="font-medium text-gray-900 mb-3">Analysis Results</h4>
          <div className="space-y-3">
            {results.analysis && (
              <>
                <div>
                  <h5 className="text-sm font-medium text-gray-700">Summary</h5>
                  <p className="text-sm text-gray-600 mt-1">{results.analysis.summary}</p>
                </div>
                <div>
                  <h5 className="text-sm font-medium text-gray-700">Root Cause</h5>
                  <p className="text-sm text-gray-600 mt-1">{results.analysis.rootCause}</p>
                </div>
                <div>
                  <h5 className="text-sm font-medium text-gray-700">Severity</h5>
                  <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                    results.analysis.severity === 'critical' ? 'bg-red-100 text-red-800' :
                    results.analysis.severity === 'high' ? 'bg-orange-100 text-orange-800' :
                    results.analysis.severity === 'medium' ? 'bg-yellow-100 text-yellow-800' :
                    'bg-green-100 text-green-800'
                  }`}>
                    {results.analysis.severity}
                  </span>
                </div>
                {results.analysis.recommendations && results.analysis.recommendations.length > 0 && (
                  <div>
                    <h5 className="text-sm font-medium text-gray-700">Recommendations</h5>
                    <ul className="text-sm text-gray-600 mt-1 list-disc list-inside">
                      {results.analysis.recommendations.map((rec, index) => (
                        <li key={index}>{rec}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default RCAnalysis; 