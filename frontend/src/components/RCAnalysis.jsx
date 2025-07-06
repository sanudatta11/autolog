import React, { useState, useEffect, useRef } from 'react';
import api from '../services/api';
import { PollingContext } from './Layout';

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
  const [logFileDetails, setLogFileDetails] = useState(null);
  const [pollingEnabled, setPollingEnabled] = React.useContext(PollingContext);
  const [showResults, setShowResults] = useState(false);

  // Add modal state
  const [reviewModalOpen, setReviewModalOpen] = useState(false);
  const [reviewTargetJob, setReviewTargetJob] = useState(null);
  const [reviewIsCorrect, setReviewIsCorrect] = useState(null);
  const [reviewCorrection, setReviewCorrection] = useState('');
  const [reviewSubmitting, setReviewSubmitting] = useState(false);
  const [reviewSubmitted, setReviewSubmitted] = useState(false);
  const [jobFeedbacks, setJobFeedbacks] = useState({}); // { jobId: feedbackObj }

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

  // Helper to determine if polling should be active
  const shouldPoll = () => {
    return pollingEnabled && (status === 'pending' || status === 'running' || status === 'processing');
  };

  // On mount or when logFileId changes, check for active RCA job (but do not set up polling here)
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
      }
    });
    return () => {
      cancelled = true;
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, [logFileId]);

  // Polling effect: only set up polling interval if shouldPoll() is true
  useEffect(() => {
    if (shouldPoll() && jobId) {
      pollingRef.current = setInterval(() => pollJobStatus(jobId), POLL_INTERVAL);
    } else if (pollingRef.current) {
      clearInterval(pollingRef.current);
    }
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, [jobId, status, pollingEnabled]);

  // Fetch all RCA jobs for this log file
  const fetchJobs = async () => {
    setLoadingJobs(true);
    try {
      const response = await api.get(`/logs/${logFileId}/analyses`);
      setJobs(response.data.analyses || []);
    } catch (err) {
      setJobs([]);
    }
    setLoadingJobs(false);
  };

  // Fetch log file details
  const fetchLogFileDetails = async () => {
    try {
      const response = await api.get(`/logs/${logFileId}`);
      setLogFileDetails(response.data.logFile);
    } catch (err) {
      console.error('Failed to fetch log file details:', err);
    }
  };

  useEffect(() => {
    if (logFileId) {
      fetchJobs();
      fetchLogFileDetails();
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
      if (shouldPoll()) {
      pollingRef.current = setInterval(() => pollJobStatus(response.data.jobId), POLL_INTERVAL);
      }
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
      // Get the analysisMemoryId from the latest completed job
      const response = await api.get(`/logs/${logFileId}/analyses`);
      console.log('Analyses response:', response.data);
      const completedJob = (response.data.analyses || []).find(j => j.status === 'completed');
      console.log('Completed job:', completedJob);
      if (!completedJob || !completedJob.analysisMemoryId) {
        alert('No completed RCA job found for this log file.');
        setIsSubmitting(false);
        return;
      }
      
      console.log('Submitting feedback to:', `/api/v1/analyses/${completedJob.analysisMemoryId}/feedback`);
      console.log('Feedback data:', { isCorrect, correction });
      
      await api.post(`/api/v1/analyses/${completedJob.analysisMemoryId}/feedback`, {
        isCorrect,
        correction,
      });
      setFeedbackSubmitted(true);
    } catch (err) {
      console.error('Feedback submission error:', err);
      alert('Failed to submit feedback: ' + (err.response?.data?.error || err.message));
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
      if (shouldPoll()) {
      pollingRef.current = setInterval(() => pollJobStatus(response.data.jobId), POLL_INTERVAL);
      }
      
      fetchJobs();
    } catch (err) {
      setError('Failed to start new RCA analysis');
      setStatus('failed');
    }
    setIsSubmitting(false);
  };

  // Before rendering RCA controls, check if RCA is possible
  if (logFileDetails && logFileDetails.isRCAPossible === false) {
  return (
      <div className="bg-blue-50 border border-blue-200 rounded-md p-4 flex items-center">
        <svg className="h-6 w-6 text-blue-400 mr-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M12 20a8 8 0 100-16 8 8 0 000 16z" /></svg>
        <div>
          <h4 className="text-blue-800 font-semibold mb-1">No RCA Needed</h4>
          <p className="text-blue-700 text-sm">{logFileDetails.rcaNotPossibleReason || 'This log file does not require Root Cause Analysis.'}</p>
        </div>
      </div>
    );
  }

  // Advanced settings dropdown
  const AdvancedSettings = () => (
    <div className="mb-2">
      <button
        type="button"
        className="text-blue-600 text-sm underline focus:outline-none"
        onClick={() => setShowAdvanced((v) => !v)}
      >
        {showAdvanced ? 'Hide' : 'Show'} Advanced Settings
      </button>
      {showAdvanced && (
        <div className="mt-2 p-3 bg-gray-50 border border-gray-200 rounded-md space-y-2">
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">LLM Timeout (seconds)</label>
            <input
              type="number"
              min={30}
              max={1800}
              value={llmTimeout}
              onChange={e => setLlmTimeout(Number(e.target.value))}
              className="w-24 px-2 py-1 border rounded text-sm"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">Chunking</label>
            <div className="flex items-center space-x-4">
              <label className="inline-flex items-center">
                <input
                  type="radio"
                  checked={useChunking}
                  onChange={() => setUseChunking(true)}
                  className="form-radio"
                />
                <span className="ml-1 text-sm">Yes</span>
              </label>
              <label className="inline-flex items-center">
                <input
                  type="radio"
                  checked={!useChunking}
                  onChange={() => setUseChunking(false)}
                  className="form-radio"
                />
                <span className="ml-1 text-sm">No</span>
              </label>
            </div>
          </div>
        </div>
      )}
    </div>
  );

  // Main RCA action button logic
  const renderMainButton = () => {
    if (status === 'pending' || status === 'running') {
      return (
        <button className="btn btn-primary opacity-60 cursor-not-allowed" disabled>
          RCA Analysis In Progress...
        </button>
      );
    }
    if (status === 'completed' && results) {
      return (
        <button className="btn btn-primary" onClick={getResults}>
          View Results
        </button>
      );
    }
    // Idle, not_started, or completed (no results yet)
    return (
          <button
            className="btn btn-primary"
        onClick={handleNewRun}
        disabled={isSubmitting}
      >
        {isSubmitting ? 'Starting...' : 'Start RCA Analysis'}
      </button>
    );
  };

  // Placeholder icons (replace with actual icon components or imports as needed)
  const PlayIcon = () => <span className="inline-block align-middle">▶️</span>;
  const CheckIcon = () => <span className="inline-block align-middle">✔️</span>;
  const ChevronIcon = ({ open }) => (
    <span className={`inline-block transition-transform ${open ? 'rotate-180' : ''}`}>▼</span>
  );
  const InfoIcon = ({ tooltip }) => (
    <span className="ml-1 text-blue-400 cursor-pointer" title={tooltip}>ℹ️</span>
  );

  // Spinner icon placeholder
  const SpinnerIcon = () => <span className="inline-block animate-spin mr-2">⏳</span>;

  // Fetch feedbacks for all jobs after jobs are loaded
  useEffect(() => {
    const fetchFeedbacks = async () => {
      const feedbackMap = {};
      for (const job of jobs) {
        if (job.analysisMemoryId) {
          try {
            const res = await api.get(`/api/v1/analyses/${job.analysisMemoryId}/feedback`);
            if (res.data.feedback && res.data.feedback.length > 0) {
              feedbackMap[job.id] = res.data.feedback[0];
            }
          } catch {}
        }
      }
      setJobFeedbacks(feedbackMap);
    };
    if (jobs.length > 0) fetchFeedbacks();
  }, [jobs]);

  return (
    <div className="max-w-lg mx-auto bg-white shadow rounded-xl p-6 space-y-6">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-lg font-semibold text-gray-900">Root Cause Analysis</h2>
        <span className="px-2 py-0.5 rounded-full bg-gray-100 text-xs font-semibold text-gray-700 border border-gray-200">{getStatusText()}</span>
      </div>
      {/* Advanced Settings Accordion */}
      <div className="mt-2">
          <button
          type="button"
          className="flex items-center gap-2 text-blue-700 font-medium focus:outline-none"
          onClick={() => setShowAdvanced((v) => !v)}
          >
          <ChevronIcon open={showAdvanced} />
          {showAdvanced ? 'Hide' : 'Show'} Advanced Settings
          </button>
        {showAdvanced && (
          <div className="mt-2 p-4 bg-gray-50 border border-gray-200 rounded-md space-y-3">
            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">LLM Timeout (seconds)</label>
              <input
                type="number"
                min={30}
                max={1800}
                value={llmTimeout}
                onChange={e => setLlmTimeout(Number(e.target.value))}
                className="w-24 px-2 py-1 border rounded text-sm"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">Chunking</label>
              <div className="flex items-center space-x-4">
                <label className="inline-flex items-center">
                  <input
                    type="radio"
                    checked={useChunking}
                    onChange={() => setUseChunking(true)}
                    className="form-radio"
                  />
                  <span className="ml-1 text-sm">Yes</span>
                </label>
                <label className="inline-flex items-center">
                  <input
                    type="radio"
                    checked={!useChunking}
                    onChange={() => setUseChunking(false)}
                    className="form-radio"
                  />
                  <span className="ml-1 text-sm">No</span>
                </label>
              </div>
            </div>
        </div>
      )}
      </div>
      {/* Progress percentage and bar if running/pending */}
      {(status === 'pending' || status === 'running') && (
        <>
          <div className="flex justify-end text-xs text-gray-500 font-medium mb-1">
            Progress: {progress}%
          </div>
          <div className="w-full bg-blue-200 rounded-full h-2 mb-2">
            <div 
              className="bg-blue-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            ></div>
          </div>
        </>
      )}
      {/* Error message */}
      {error && (
        <div className="bg-red-50 border border-red-200 rounded-md p-3 mt-2">
          <p className="text-red-800 text-sm">{error}</p>
        </div>
      )}
      {/* RCA Results Section (collapsible as before) */}
      {status === 'completed' && results && (
        <>
          <button
            className="w-full flex items-center justify-center gap-2 btn btn-secondary mb-2"
            onClick={() => setShowResults(v => !v)}
          >
            {showResults ? 'Hide Results' : 'View Results'}
          </button>
          {/* Feedback Form always visible after RCA completion */}
          <div className="mt-6 p-4 border rounded bg-gray-50">
            <h3 className="font-semibold mb-2">Was this analysis correct?</h3>
            <form onSubmit={handleFeedbackSubmit} className="space-y-3">
              <div className="flex items-center gap-6 mb-2">
                <label className="flex items-center gap-1">
                  <input
                    type="radio"
                    name="isCorrect"
                    value="true"
                    checked={isCorrect === true}
                    onChange={() => setIsCorrect(true)}
                    className="form-radio"
                  /> Yes
                </label>
                <label className="flex items-center gap-1">
                  <input
                    type="radio"
                    name="isCorrect"
                    value="false"
                    checked={isCorrect === false}
                    onChange={() => setIsCorrect(false)}
                    className="form-radio"
                  /> No
                </label>
        </div>
              <textarea
                className="w-full border rounded p-2"
                placeholder="Correction or comments (optional)"
                value={correction}
                onChange={e => setCorrection(e.target.value)}
              />
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
          <div
            className={`transition-all duration-300 overflow-hidden ${showResults ? 'max-h-[1000px] opacity-100' : 'max-h-0 opacity-0'}`}
          >
            <div className="bg-green-50 border border-green-200 rounded-md p-4 mt-2">
              <p className="text-green-800 text-sm mb-3">
                {(() => {
                  const analysis = results.analysis?.final || results.analysis || results;
                  const isNoErrorsAnalysis = analysis.severity === 'low' && 
                    analysis.criticalErrors === 0 && 
                    analysis.nonCriticalErrors === 0 && 
                    (!analysis.errorAnalysis || analysis.errorAnalysis.length === 0) &&
                    (analysis.summary?.toLowerCase().includes('no error') || 
                      analysis.rootCause?.toLowerCase().includes('no error'));
                  return isNoErrorsAnalysis 
                    ? "RCA Analysis completed successfully! ✅ No errors detected - your system is healthy."
                    : "RCA Analysis completed successfully! Issues have been identified and analyzed.";
                })()}
              </p>
            </div>
            <div className="bg-white border border-gray-200 rounded-md p-4 mt-2">
          <h4 className="font-medium text-gray-900 mb-3">Analysis Results</h4>
          <div className="space-y-3">
            {(() => {
              // Use analysis.final if present, else analysis
              const analysis = results.analysis?.final || results.analysis || results;
                  
                  // Check if this is a "no errors found" analysis
                  const isNoErrorsAnalysis = analysis.severity === 'low' && 
                    analysis.criticalErrors === 0 && 
                    analysis.nonCriticalErrors === 0 && 
                    (!analysis.errorAnalysis || analysis.errorAnalysis.length === 0) &&
                    (analysis.summary?.toLowerCase().includes('no error') || 
                      analysis.rootCause?.toLowerCase().includes('no error'));

                  if (isNoErrorsAnalysis) {
                    return (
                      <div className="bg-green-50 border border-green-200 rounded-lg p-4">
                        <div className="flex items-center mb-3">
                          <div className="flex-shrink-0">
                            <svg className="h-8 w-8 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                          </div>
                          <div className="ml-3">
                            <h5 className="text-lg font-medium text-green-800">No Errors Detected</h5>
                            <p className="text-sm text-green-700">Your log file contains no ERROR or FATAL entries</p>
                          </div>
                        </div>
                        
                        <div className="bg-white rounded-md p-3 border border-green-200">
                          <div className="mb-3">
                            <h6 className="text-sm font-medium text-gray-700 mb-1">Summary</h6>
                            <p className="text-sm text-gray-600">{analysis.summary}</p>
                          </div>
                          
                          <div className="mb-3">
                            <h6 className="text-sm font-medium text-gray-700 mb-1">Status</h6>
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                              ✅ System Healthy - No RCA Needed
                            </span>
                          </div>
                          
                          {Array.isArray(analysis.recommendations) && analysis.recommendations.length > 0 && (
                            <div>
                              <h6 className="text-sm font-medium text-gray-700 mb-1">Recommendations</h6>
                              <ul className="text-sm text-gray-600 list-disc list-inside">
                                {analysis.recommendations.map((rec, index) => (
                                  <li key={index}>{rec}</li>
                                ))}
                              </ul>
                            </div>
                          )}
                        </div>
                        
                        <div className="mt-3 p-3 bg-blue-50 border border-blue-200 rounded-md">
                          <p className="text-sm text-blue-800">
                            <strong>Note:</strong> Since no errors were detected, no Root Cause Analysis is needed. 
                            Your system appears to be functioning normally. Continue monitoring for any new issues.
                          </p>
                        </div>
                      </div>
                    );
                  }

                  // Standard error analysis display
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
            </div>
        </>
      )}

      {/* Table rendering (replace the old table section) */}
      <div className="mt-8">
        <h3 className="font-semibold mb-2">Past RCA Runs</h3>
        <div className="bg-white shadow rounded-lg overflow-x-auto">
        {loadingJobs ? (
            <div className="p-4">Loading past runs...</div>
        ) : jobs.length === 0 ? (
            <div className="p-4 text-gray-500">No past RCA runs found.</div>
        ) : (
            <table className="min-w-full text-sm">
            <thead>
                <tr className="bg-gray-50">
                  <th className="px-3 py-2 text-left">Run #</th>
                  <th className="px-3 py-2 text-left">Status</th>
                  <th className="px-3 py-2 text-right">Progress</th>
                  <th className="px-3 py-2 text-left">Error</th>
                  <th className="px-3 py-2 text-left">Started</th>
                  <th className="px-3 py-2 text-left">Completed</th>
                  <th className="px-3 py-2 text-right">Chunks</th>
                  <th className="px-3 py-2 text-right">Failed Chunk</th>
                  <th className="px-3 py-2 text-center">Actions</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job, idx) => (
                  <tr key={job.id} className={idx % 2 === 0 ? 'bg-white hover:bg-blue-50' : 'bg-gray-50 hover:bg-blue-50'}>
                    <td className="px-3 py-2">{jobs.length - idx}</td>
                    <td className="px-3 py-2">
                      <span className={`inline-block px-2 py-0.5 rounded text-xs font-semibold ${
                        job.status === 'completed' ? 'bg-green-100 text-green-800' :
                        job.status === 'failed' ? 'bg-red-100 text-red-800' :
                        job.status === 'running' ? 'bg-blue-100 text-blue-800' :
                        job.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
                        'bg-gray-100 text-gray-800'
                      }`}>
                        {job.status}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-right">{job.progress}%</td>
                    <td className="px-3 py-2 text-red-700">{job.error || '-'}</td>
                    <td className="px-3 py-2">{job.startedAt ? new Date(job.startedAt).toLocaleString() : '-'}</td>
                    <td className="px-3 py-2">{job.completedAt ? new Date(job.completedAt).toLocaleString() : '-'}</td>
                    <td className="px-3 py-2 text-right">{job.totalChunks || '-'}</td>
                    <td className="px-3 py-2 text-right">{job.failedChunk || '-'}</td>
                    <td className="px-3 py-2 text-center space-x-1">
                      <button
                        className="inline-block px-2 py-1 rounded bg-blue-500 text-white text-xs font-medium disabled:opacity-50"
                        disabled={job.status !== 'completed'}
                        title="View Results"
                        onClick={() => {
                          setResults(job.result);
                          setShowResults(true);
                        }}
                      >
                        View
                      </button>
                      {job.status === 'completed' && (
                        jobFeedbacks[job.id] ? (
                          <button className="inline-block px-2 py-1 rounded bg-green-400 text-white text-xs font-medium opacity-60 cursor-not-allowed" disabled>Reviewed</button>
                        ) : (
                          <button
                            className="inline-block px-2 py-1 rounded bg-purple-500 text-white text-xs font-medium"
                            onClick={() => {
                              setReviewTargetJob(job);
                              setReviewIsCorrect(null);
                              setReviewCorrection('');
                              setReviewModalOpen(true);
                            }}
                          >
                            Give Review
                          </button>
                        )
                      )}
                      {job.status === 'failed' && (
                        <button
                          className="inline-block px-2 py-1 rounded bg-yellow-500 text-white text-xs font-medium"
                        onClick={handleNewRun}
                        disabled={isSubmitting}
                          title="Retry RCA"
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

      {/* Review Modal */}
      {reviewModalOpen && reviewTargetJob && (
        <div className="fixed inset-0 bg-black bg-opacity-40 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-lg p-6 w-full max-w-md">
            <h3 className="font-semibold text-lg mb-2">Review RCA Run #{jobs.length - jobs.findIndex(j => j.id === reviewTargetJob.id)}</h3>
            <div className="mb-2 text-sm text-gray-700">
              <div><b>Status:</b> {reviewTargetJob.status}</div>
              <div><b>Started:</b> {reviewTargetJob.startedAt ? new Date(reviewTargetJob.startedAt).toLocaleString() : '-'}</div>
              <div><b>Completed:</b> {reviewTargetJob.completedAt ? new Date(reviewTargetJob.completedAt).toLocaleString() : '-'}</div>
            </div>
            <form
              onSubmit={async (e) => {
                e.preventDefault();
                setReviewSubmitting(true);
                try {
                  await api.post(`/api/v1/analyses/${reviewTargetJob.analysisMemoryId}/feedback`, {
                    isCorrect: reviewIsCorrect,
                    correction: reviewCorrection,
                  });
                  setReviewSubmitted(true);
                  setJobFeedbacks(f => ({ ...f, [reviewTargetJob.id]: { isCorrect: reviewIsCorrect, correction: reviewCorrection } }));
                  setTimeout(() => {
                    setReviewModalOpen(false);
                    setReviewSubmitted(false);
                  }, 1200);
                } catch {
                  alert('Failed to submit review');
                }
                setReviewSubmitting(false);
              }}
              className="space-y-3"
            >
              <div className="flex items-center gap-6 mb-2">
                <label className="flex items-center gap-1">
                  <input
                    type="radio"
                    name="reviewIsCorrect"
                    value="true"
                    checked={reviewIsCorrect === true}
                    onChange={() => setReviewIsCorrect(true)}
                    className="form-radio"
                  /> Yes
                </label>
                <label className="flex items-center gap-1">
                  <input
                    type="radio"
                    name="reviewIsCorrect"
                    value="false"
                    checked={reviewIsCorrect === false}
                    onChange={() => setReviewIsCorrect(false)}
                    className="form-radio"
                  /> No
                </label>
              </div>
              <textarea
                className="w-full border rounded p-2"
                placeholder="Correction or comments (optional)"
                value={reviewCorrection}
                onChange={e => setReviewCorrection(e.target.value)}
              />
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  className="px-4 py-2 rounded bg-gray-200 text-gray-700"
                  onClick={() => setReviewModalOpen(false)}
                  disabled={reviewSubmitting}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded bg-blue-600 text-white"
                  disabled={reviewSubmitting || reviewIsCorrect === null}
                >
                  {reviewSubmitting ? 'Submitting...' : 'Submit Review'}
                </button>
              </div>
              {reviewSubmitted && <div className="text-green-700 mt-2">Thank you for your feedback!</div>}
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default RCAnalysis; 