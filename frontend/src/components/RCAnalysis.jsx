import React, { useState, useEffect, useRef } from 'react';
import api from '../services/api';

const POLL_INTERVAL = 2000;

const RCAnalysis = ({ logFileId, initialStatus = 'idle', onAnalysisComplete }) => {
  const [jobId, setJobId] = useState(null);
  const [status, setStatus] = useState(initialStatus); // idle, pending, running, completed, failed
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState('');
  const [results, setResults] = useState(null);
  const [totalChunks, setTotalChunks] = useState(null);
  const [failedChunk, setFailedChunk] = useState(null);
  const pollingRef = useRef(null);
  const [isCorrect, setIsCorrect] = useState(null);
  const [correction, setCorrection] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [feedbackSubmitted, setFeedbackSubmitted] = useState(false);
  const [currentChunk, setCurrentChunk] = useState(null);
  const [jobs, setJobs] = useState([]);
  const [loadingJobs, setLoadingJobs] = useState(false);
  const [llmTimeout, setLlmTimeout] = useState(300); // default 300 seconds
  const [useChunking, setUseChunking] = useState(true);
  const [showAdvanced, setShowAdvanced] = useState(false);

  // Helper to poll job status
  const pollJobStatus = (jobId) => {
    api.get(`/jobs/${jobId}/status`).then(response => {
      const job = response.data.job;
      setStatus(job.status);
      setProgress(job.progress);
      setTotalChunks(response.data.totalChunks || job.totalChunks || null);
      setFailedChunk(response.data.failedChunk || job.failedChunk || null);
      setCurrentChunk(response.data.currentChunk || job.currentChunk || null);
      if (job.status === 'completed') {
        setResults(job.result);
        clearInterval(pollingRef.current);
        if (onAnalysisComplete) onAnalysisComplete(job.result);
      } else if (job.status === 'failed') {
        setError(job.error || 'RCA analysis failed');
        clearInterval(pollingRef.current);
      }
    }).catch(() => {
      setError('Failed to check job status');
      clearInterval(pollingRef.current);
    });
  };

  // On mount, always check for active RCA job and start polling if found
  useEffect(() => {
    let cancelled = false;
    api.get(`/logs/${logFileId}/analyses`).then(res => {
      if (cancelled) return;
      const jobs = res.data.analyses || [];
      const activeJob = jobs.find(j => j.status === 'pending' || j.status === 'running');
      if (activeJob) {
        setJobId(activeJob.id);
        setProgress(activeJob.progress || 0);
        setStatus(activeJob.status);
        pollingRef.current = setInterval(() => pollJobStatus(activeJob.id), POLL_INTERVAL);
      }
    });
    return () => {
      cancelled = true;
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, [logFileId]);

  // Fetch all RCA jobs for this log file
  const fetchJobs = async () => {
    setLoadingJobs(true);
    try {
      const response = await api.get(`/logs/${logFileId}/jobs`);
      setJobs(response.data.jobs || []);
    } catch (err) {
      setJobs([]);
    }
    setLoadingJobs(false);
  };

  useEffect(() => {
    if (logFileId) {
      fetchJobs();
    }
  }, [logFileId]);

  // When starting a new analysis, also start polling
  const startAnalysis = async () => {
    try {
      setStatus('pending');
      setError('');
      setProgress(0);
      
      const response = await api.post(`/logs/${logFileId}/analyze`);
      setJobId(response.data.jobId);
      setStatus('pending');
      pollingRef.current = setInterval(() => pollJobStatus(response.data.jobId), POLL_INTERVAL);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to start RCA analysis');
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

  const handleFeedbackSubmit = async (e) => {
    e.preventDefault();
    setIsSubmitting(true);
    try {
      await api.post('/feedback', {
        analysisMemoryId: results?.analysisMemoryId || results?.id, // Use correct ID from RCA result
        isCorrect,
        correction,
      });
      setFeedbackSubmitted(true);
    } catch (err) {
      alert('Failed to submit feedback');
    }
    setIsSubmitting(false);
  };

  // Handler to trigger a new RCA run
  const handleNewRun = async () => {
    setIsSubmitting(true);
    setFeedbackSubmitted(false);
    setError('');
    try {
      setStatus('pending');
      setProgress(0);
      
      const response = await api.post(`/logs/${logFileId}/analyze`, {
        timeout: llmTimeout,
        chunking: useChunking,
      });
      
      setJobId(response.data.jobId);
      setStatus('pending');
      pollingRef.current = setInterval(() => pollJobStatus(response.data.jobId), POLL_INTERVAL);
      
      fetchJobs();
    } catch (err) {
      setError('Failed to start new RCA analysis');
      setStatus('failed');
    }
    setIsSubmitting(false);
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

      {/* Show chunk info if available */}
      {totalChunks && (
        <div className="text-xs text-gray-500">Total Chunks: {totalChunks}</div>
      )}
      {failedChunk && status === 'failed' && (
        <div className="text-xs text-red-600">Failed at chunk: {failedChunk}</div>
      )}

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-md p-3">
          <p className="text-red-800 text-sm">{error}</p>
        </div>
      )}

      {/* Retry button if failed and failedChunk is set */}
      {status === 'failed' && failedChunk && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-md p-4">
          <p className="text-yellow-800 text-sm mb-3">
            RCA analysis failed at chunk {failedChunk}. You can retry the analysis.
          </p>
          <button
            onClick={startAnalysis}
            className="btn btn-primary"
          >
            Retry RCA Analysis
          </button>
        </div>
      )}

      {/* Only show the button if not in progress */}
      {(status === 'idle' || status === 'not_started' || status === 'completed') && (
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
          {status === 'running' && totalChunks && (
            <div className="mb-2 text-sm text-gray-700">
              Divided into {totalChunks} chunks.<br />
              {currentChunk ? (
                <>Processing chunk {currentChunk} of {totalChunks}...</>
              ) : (
                <>Preparing chunks...</>
              )}
            </div>
          )}
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
            {(() => {
              // Use analysis.final if present, else analysis
              const analysis = results.analysis?.final || results.analysis || results;
              return (
                <>
                  <div>
                    <h5 className="text-sm font-medium text-gray-700">Summary</h5>
                    <p className="text-sm text-gray-600 mt-1">{analysis.summary}</p>
                  </div>
                  <div>
                    <h5 className="text-sm font-medium text-gray-700">Root Cause</h5>
                    <p className="text-sm text-gray-600 mt-1">{analysis.rootCause}</p>
                  </div>
                  <div>
                    <h5 className="text-sm font-medium text-gray-700">Severity</h5>
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      analysis.severity === 'critical' ? 'bg-red-100 text-red-800' :
                      analysis.severity === 'high' ? 'bg-orange-100 text-orange-800' :
                      analysis.severity === 'medium' ? 'bg-yellow-100 text-yellow-800' :
                      'bg-green-100 text-green-800'
                    }`}>
                      {analysis.severity}
                    </span>
                  </div>
                  {Array.isArray(analysis.recommendations) && analysis.recommendations.length > 0 && (
                    <div>
                      <h5 className="text-sm font-medium text-gray-700">Recommendations</h5>
                      <ul className="text-sm text-gray-600 mt-1 list-disc list-inside">
                        {analysis.recommendations.map((rec, index) => (
                          <li key={index}>{rec}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                  {/* Advanced: Raw LLM response */}
                  {results.analysis?.rawLLMResponse && (
                    <div className="mt-4">
                      <button
                        className="text-xs text-blue-600 underline"
                        onClick={() => setShowAdvanced(v => !v)}
                      >
                        {showAdvanced ? 'Hide' : 'Show'} Advanced (Raw LLM Response)
                      </button>
                      {showAdvanced && (
                        <pre className="mt-2 p-2 bg-gray-100 text-xs rounded overflow-x-auto max-h-64">
                          {results.analysis.rawLLMResponse}
                        </pre>
                      )}
                    </div>
                  )}
                </>
              );
            })()}
          </div>
        </div>
      )}

      {status === 'completed' && results && (
        <div className="mt-6 p-4 border rounded bg-gray-50">
          <h3 className="font-semibold mb-2">Was this analysis correct?</h3>
          <form onSubmit={handleFeedbackSubmit}>
            <div className="mb-2">
              <label>
                <input
                  type="radio"
                  name="isCorrect"
                  value="true"
                  checked={isCorrect === true}
                  onChange={() => setIsCorrect(true)}
                /> Yes
              </label>
              <label className="ml-4">
                <input
                  type="radio"
                  name="isCorrect"
                  value="false"
                  checked={isCorrect === false}
                  onChange={() => setIsCorrect(false)}
                /> No
              </label>
            </div>
            <div className="mb-2">
              <textarea
                className="w-full border rounded p-2"
                placeholder="Correction or comments (optional)"
                value={correction}
                onChange={e => setCorrection(e.target.value)}
              />
            </div>
            <button
              type="submit"
              className="bg-blue-600 text-white px-4 py-2 rounded"
              disabled={isSubmitting || isCorrect === null}
            >
              Submit Feedback
            </button>
            {feedbackSubmitted && (
              <div className="text-green-700 mt-2">Thank you for your feedback!</div>
            )}
          </form>
        </div>
      )}

      <div className="mt-8">
        <h3 className="font-semibold mb-2">Start New RCA Analysis</h3>
        <div className="mb-2">
          <label className="mr-2 font-medium">LLM Timeout (seconds):</label>
          <input
            type="number"
            min="30"
            max="1800"
            value={llmTimeout}
            onChange={e => setLlmTimeout(Number(e.target.value))}
            disabled={isSubmitting || status === 'running' || status === 'pending'}
            className="border rounded px-2 py-1 w-24"
          />
        </div>
        <div className="mb-2">
          <span className="font-medium mr-2">Chunking:</span>
          <label className="mr-4">
            <input
              type="radio"
              name="chunking"
              checked={useChunking}
              onChange={() => setUseChunking(true)}
              disabled={isSubmitting || status === 'running' || status === 'pending'}
            /> Yes
          </label>
          <label>
            <input
              type="radio"
              name="chunking"
              checked={!useChunking}
              onChange={() => setUseChunking(false)}
              disabled={isSubmitting || status === 'running' || status === 'pending'}
            /> No
          </label>
        </div>
        <button
          className="mb-2 bg-blue-600 text-white px-3 py-1 rounded"
          onClick={handleNewRun}
          disabled={isSubmitting || status === 'running' || status === 'pending'}
        >
          Trigger New RCA Run
        </button>
        {loadingJobs ? (
          <div>Loading past runs...</div>
        ) : jobs.length === 0 ? (
          <div>No past RCA runs found.</div>
        ) : (
          <table className="w-full border text-sm">
            <thead>
              <tr>
                <th className="border px-2 py-1">Run #</th>
                <th className="border px-2 py-1">Status</th>
                <th className="border px-2 py-1">Progress</th>
                <th className="border px-2 py-1">Error</th>
                <th className="border px-2 py-1">Started</th>
                <th className="border px-2 py-1">Completed</th>
                <th className="border px-2 py-1">Chunks</th>
                <th className="border px-2 py-1">Failed Chunk</th>
                <th className="border px-2 py-1">Retry</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job, idx) => (
                <tr key={job.id} className="border">
                  <td className="border px-2 py-1">{jobs.length - idx}</td>
                  <td className="border px-2 py-1">{job.status}</td>
                  <td className="border px-2 py-1">{job.progress}%</td>
                  <td className="border px-2 py-1 text-red-700">{job.error || '-'}</td>
                  <td className="border px-2 py-1">{job.startedAt ? new Date(job.startedAt).toLocaleString() : '-'}</td>
                  <td className="border px-2 py-1">{job.completedAt ? new Date(job.completedAt).toLocaleString() : '-'}</td>
                  <td className="border px-2 py-1">{job.totalChunks || '-'}</td>
                  <td className="border px-2 py-1">{job.failedChunk || '-'}</td>
                  <td className="border px-2 py-1">
                    {job.status === 'failed' && (
                      <button
                        className="bg-yellow-500 text-white px-2 py-1 rounded"
                        onClick={handleNewRun}
                        disabled={isSubmitting}
                      >
                        Retry
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
};

export default RCAnalysis; 