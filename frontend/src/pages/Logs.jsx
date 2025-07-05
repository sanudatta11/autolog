import React, { useState, useEffect, useRef } from 'react';
import { useAuth } from '../contexts/AuthContext';
import api from '../services/api';
import RCAnalysis from '../components/RCAnalysis';
import AdminLogs from '../components/AdminLogs';
import jsPDF from "jspdf";
import autoTable from "jspdf-autotable";
import {
  LOGS_POLL_INTERVAL_MS,
  UPLOAD_PROGRESS_BAR_HEIGHT,
  PDF_SUMMARY_WRAP_WIDTH,
  PDF_ROOT_CAUSE_WRAP_WIDTH,
  PDF_RECOMMENDATION_WRAP_WIDTH,
  PDF_TABLE_COLUMN_WIDTHS
} from '../constants';

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
  const [uploadProgress, setUploadProgress] = useState(0);

  const [userRole, setUserRole] = useState('');
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleteTargetLogFile, setDeleteTargetLogFile] = useState(null);
  const [hardDelete, setHardDelete] = useState(false);

  // After logFiles are updated, check if polling should be running
  useEffect(() => {
    let interval = null;
    const shouldPoll = logFiles.some(
      (log) =>
        log.status === 'processing' ||
        log.rcaAnalysisStatus === 'pending' ||
        log.rcaAnalysisStatus === 'running'
    );
    if (shouldPoll) {
      interval = setInterval(() => {
        fetchLogFiles();
      }, LOGS_POLL_INTERVAL_MS);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [logFiles]);

  // On mount, fetch logs and user role
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
    setUploadProgress(0);
    const formData = new FormData();
    formData.append('logfile', selectedFile);

    try {
      const response = await api.post('/logs/upload', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
        onUploadProgress: (progressEvent) => {
          if (progressEvent.total) {
            const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total);
            setUploadProgress(percent);
          }
        },
      });
      setMessage('Log file uploaded successfully! Processing in background...');
      setSelectedFile(null);
      setUploadProgress(0);
      document.getElementById('file-input').value = '';

      // Find the uploaded log file (by filename, most recent)
      let uploadedLogFile = null;
      let pollInterval = null;
      const pollLogFileStatus = async () => {
        await fetchLogFiles();
        // Try to find the uploaded file by filename
        const latestLogFiles = await api.get('/logs');
        const files = latestLogFiles.data.logFiles || [];
        uploadedLogFile = files.find(f => f.filename === selectedFile?.name) || files[0];
        if (uploadedLogFile) {
          if (uploadedLogFile.status === 'completed') {
            setMessage('Log file parsed successfully!');
            clearInterval(pollInterval);
          } else if (uploadedLogFile.status === 'failed') {
            setMessage('Log file parsing failed.');
            clearInterval(pollInterval);
          } else {
            setMessage('Log file is being parsed...');
          }
        }
      };
      // Start polling every 2 seconds
      pollInterval = setInterval(pollLogFileStatus, 2000);
      // Run once immediately
      pollLogFileStatus();
    } catch (error) {
      setMessage('Upload failed: ' + error.message);
      setUploadProgress(0);
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
      const msg = response.data && response.data.message
        ? response.data.message
        : 'RCA analysis started.';
      setMessage('RCA analysis started: ' + msg);
      // Refresh log files to show updated status
      fetchLogFiles();
    } catch (error) {
      setMessage('Analysis failed: ' + (error.response?.data?.error || error.message));
    }
  };

  const handleDelete = (logFileId) => {
    const logFile = logFiles.find(l => l.id === logFileId);
    setDeleteTargetLogFile(logFile);
    setShowDeleteModal(true);
  };

  const confirmDelete = async () => {
    if (!deleteTargetLogFile) return;
    try {
      await api.delete(`/logs/${deleteTargetLogFile.id}?hardDelete=${hardDelete}`);
      setMessage('Log file deleted successfully');
      setShowDeleteModal(false);
      setDeleteTargetLogFile(null);
      setHardDelete(false);
      fetchLogFiles();
      if (selectedLogFile && selectedLogFile.id === deleteTargetLogFile.id) {
        setSelectedLogFile(null);
        setLogEntries([]);
      }
    } catch (error) {
      setMessage('Failed to delete log file: ' + error.message);
      setShowDeleteModal(false);
      setDeleteTargetLogFile(null);
      setHardDelete(false);
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

  // Helper to generate PDF from RCA data
  const handleDownloadRcaPdf = () => {
    if (!llmModalAnalysis) return;
    let analysis = llmModalAnalysis;
    if (analysis && typeof analysis === 'object') {
      if ('final' in analysis) analysis = analysis.final;
      else if ('analysis' in analysis) analysis = analysis.analysis;
    }
    const doc = new jsPDF();
    let y = 10;
    doc.setFontSize(16);
    doc.text(`Root Cause Analysis for: ${llmModalLogFile?.filename || ''}`, 10, y);
    y += 10;
    doc.setFontSize(12);
    doc.text(`Summary: ${analysis.summary || ''}`, 10, y);
    y += 8;
    doc.text(`Severity: ${analysis.severity || ''}`, 10, y);
    y += 8;
    if (analysis.rootCause) {
      doc.text(`Root Cause: ${analysis.rootCause}`, 10, y);
      y += 8;
    }
    if (Array.isArray(analysis.recommendations) && analysis.recommendations.length > 0) {
      doc.text('Recommendations:', 10, y);
      y += 8;
      analysis.recommendations.forEach((rec) => {
        doc.text(`- ${rec}`, 14, y);
        y += 7;
      });
    }
    y += 4;
    if (typeof analysis.criticalErrors === 'number') {
      doc.text(`Critical Errors: ${analysis.criticalErrors}`, 10, y);
      y += 7;
    }
    if (typeof analysis.nonCriticalErrors === 'number') {
      doc.text(`Non-critical Errors: ${analysis.nonCriticalErrors}`, 10, y);
      y += 7;
    }
    if (typeof analysis.errorCount === 'number') {
      doc.text(`Total Errors: ${analysis.errorCount}`, 10, y);
      y += 7;
    }
    if (typeof analysis.warningCount === 'number') {
      doc.text(`Warnings: ${analysis.warningCount}`, 10, y);
      y += 7;
    }
    y += 4;
    if (Array.isArray(analysis.errorAnalysis) && analysis.errorAnalysis.length > 0) {
      doc.text('Error Analysis:', 10, y);
      y += 8;
      // Table header
      doc.setFont(undefined, 'bold');
      doc.text('Pattern', 10, y);
      doc.text('Count', 50, y);
      doc.text('Severity', 65, y);
      doc.text('First', 85, y);
      doc.text('Last', 120, y);
      doc.text('Root Cause', 10, y + 6);
      doc.text('Impact', 50, y + 6);
      doc.text('Fix', 85, y + 6);
      doc.setFont(undefined, 'normal');
      y += 12;
      analysis.errorAnalysis.forEach((err) => {
        if (y > 270) { doc.addPage(); y = 10; }
        doc.text(String(err.errorPattern || ''), 10, y);
        doc.text(String(err.errorCount || ''), 50, y);
        doc.text(String(err.severity || ''), 65, y);
        doc.text(String(err.firstOccurrence || ''), 85, y);
        doc.text(String(err.lastOccurrence || ''), 120, y);
        y += 6;
        doc.text(String(err.rootCause || ''), 10, y);
        doc.text(String(err.impact || ''), 50, y);
        doc.text(String(err.fix || ''), 85, y);
        y += 8;
      });
    }
    doc.save(`RCA_${llmModalLogFile?.filename || 'report'}.pdf`);
  };

  const handleDownloadRcaPdfForLog = async (logFile) => {
    try {
      // Fetch RCA analysis for this log file
      const response = await api.get(`/logs/${logFile.id}/rca-results`);
      let analysis = response.data.analysis;
      if (analysis && typeof analysis === 'object') {
        if ('final' in analysis) analysis = analysis.final;
        else if ('analysis' in analysis) analysis = analysis.analysis;
      }
      // Load logo image as base64
      const logoUrl = '/autolog.png';
      const getImageBase64 = (url) => new Promise((resolve, reject) => {
        const img = new window.Image();
        img.crossOrigin = 'Anonymous';
        img.onload = function () {
          const canvas = document.createElement('canvas');
          canvas.width = img.width;
          canvas.height = img.height;
          const ctx = canvas.getContext('2d');
          ctx.drawImage(img, 0, 0);
          resolve(canvas.toDataURL('image/png'));
        };
        img.onerror = reject;
        img.src = url;
      });
      const logoBase64 = await getImageBase64(logoUrl);
      const doc = new jsPDF();
      // Add logo
      doc.addImage(logoBase64, 'PNG', 10, 8, 32, 16);
      // Branding/Header
      doc.setFontSize(18);
      doc.setTextColor(40, 40, 80);
      doc.text('AutoLog - Root Cause Analysis Report', 105, 18, { align: 'center' });
      doc.setFontSize(10);
      doc.setTextColor(100);
      doc.text(`Generated: ${new Date().toLocaleString()}`, 200, 10, { align: 'right' });
      doc.setDrawColor(40, 40, 80);
      doc.line(10, 22, 200, 22);
      let y = 28;
      doc.setFontSize(12);
      doc.setTextColor(0);
      doc.text(`Log File: ${logFile.filename || ''}`, 14, y);
      y += 8;
      doc.setFont(undefined, 'bold');
      doc.text('Summary', 14, y);
      doc.setFont(undefined, 'normal');
      const summaryLines = doc.splitTextToSize(analysis.summary || '', PDF_SUMMARY_WRAP_WIDTH);
      doc.text(summaryLines, 40, y);
      y += summaryLines.length * 6;
      doc.setFont(undefined, 'bold');
      doc.text('Severity:', 14, y);
      doc.setFont(undefined, 'normal');
      doc.text(String(analysis.severity || ''), 40, y);
      y += 8;
      doc.setFont(undefined, 'bold');
      doc.text('Root Cause', 14, y);
      doc.setFont(undefined, 'normal');
      const rootCauseLines = doc.splitTextToSize(analysis.rootCause || '', PDF_ROOT_CAUSE_WRAP_WIDTH);
      doc.text(rootCauseLines, 40, y);
      y += rootCauseLines.length * 6;
      doc.setFont(undefined, 'bold');
      doc.text('Recommendations', 14, y);
      doc.setFont(undefined, 'normal');
      if (Array.isArray(analysis.recommendations) && analysis.recommendations.length > 0) {
        analysis.recommendations.forEach((rec) => {
          y += 7;
          const recLines = doc.splitTextToSize(`- ${rec}`, PDF_RECOMMENDATION_WRAP_WIDTH);
          doc.text(recLines, 20, y);
          y += (recLines.length - 1) * 6;
        });
      } else {
        y += 7;
        doc.text('-', 20, y);
      }
      y += 10;
      doc.setFont(undefined, 'bold');
      doc.text('Critical Errors:', 14, y);
      doc.setFont(undefined, 'normal');
      doc.text(String(analysis.criticalErrors ?? '-'), 50, y);
      doc.setFont(undefined, 'bold');
      doc.text('Non-critical Errors:', 80, y);
      doc.setFont(undefined, 'normal');
      doc.text(String(analysis.nonCriticalErrors ?? '-'), 130, y);
      y += 10;
      doc.setFont(undefined, 'bold');
      doc.text('Error Analysis', 14, y);
      y += 4;
      // Error Analysis Table
      if (Array.isArray(analysis.errorAnalysis) && analysis.errorAnalysis.length > 0) {
        autoTable(doc, {
          startY: y + 2,
          head: [[
            'Pattern', 'Count', 'Severity', 'First', 'Last',
            'Root Cause', 'Impact', 'Fix', 'Related'
          ]],
          body: analysis.errorAnalysis.map((err) => [
            err.errorPattern || '',
            err.errorCount || '',
            err.severity || '',
            err.firstOccurrence || '',
            err.lastOccurrence || '',
            err.rootCause || '',
            err.impact || '',
            err.fix || '',
            (Array.isArray(err.relatedErrors) && err.relatedErrors.length > 0)
              ? err.relatedErrors.join(', ') : '-'
          ]),
          styles: { fontSize: 8, cellPadding: 2 },
          headStyles: { fillColor: [40, 40, 80], textColor: 255, fontStyle: 'bold' },
          alternateRowStyles: { fillColor: [245, 245, 255] },
          margin: { left: 10, right: 10 },
          tableWidth: 'wrap',
          columnStyles: {
            0: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.pattern }, // Pattern
            1: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.count }, // Count
            2: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.severity }, // Severity
            3: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.first }, // First
            4: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.last }, // Last
            5: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.rootCause, cellPadding: 2, valign: 'top', maxWidth: PDF_TABLE_COLUMN_WIDTHS.rootCause }, // Root Cause
            6: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.impact, cellPadding: 2, valign: 'top', maxWidth: PDF_TABLE_COLUMN_WIDTHS.impact }, // Impact
            7: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.fix, cellPadding: 2, valign: 'top', maxWidth: PDF_TABLE_COLUMN_WIDTHS.fix }, // Fix
            8: { cellWidth: PDF_TABLE_COLUMN_WIDTHS.related, cellPadding: 2, valign: 'top', maxWidth: PDF_TABLE_COLUMN_WIDTHS.related }, // Related
          },
          didDrawCell: (data) => {
            // Optionally, further custom cell drawing logic
          },
        });
        y = doc.lastAutoTable.finalY + 10;
      } else {
        y += 8;
        doc.setFont(undefined, 'normal');
        doc.text('No error analysis data available.', 14, y);
      }
      // Footer
      doc.setFontSize(10);
      doc.setTextColor(120);
      doc.text('© 2024 AutoLog. All rights reserved.', 105, 290, { align: 'center' });
      doc.save(`RCA_${logFile.filename || 'report'}.pdf`);
    } catch (error) {
      alert('Failed to fetch RCA analysis or generate PDF.');
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
          {uploadProgress > 0 && uploading && (
            <div className="w-full bg-gray-200 rounded" style={{ height: `${UPLOAD_PROGRESS_BAR_HEIGHT}px`, marginTop: '1rem' }}>
              <div
                className="bg-blue-500 h-full rounded transition-all duration-200"
                style={{ width: `${uploadProgress}%` }}
              ></div>
              <div className="text-xs text-gray-700 mt-1 text-right">{uploadProgress}%</div>
            </div>
          )}
        </div>

        {/* Log Connectors */}
        <div className="bg-white rounded-lg shadow-md p-6 opacity-60 pointer-events-none select-none">
          <h2 className="text-xl font-semibold mb-4">🔗 Log Connectors <span className='ml-2 text-sm text-gray-500'>(To Be Developed)</span></h2>
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 border border-gray-200 rounded">
              <div className="flex items-center">
                <span className="text-2xl mr-3">☁️</span>
                <div>
                  <h3 className="font-medium">CloudWatch</h3>
                  <p className="text-sm text-gray-600">AWS Logs Integration</p>
                </div>
              </div>
              <button className="bg-orange-300 text-white px-4 py-2 rounded text-sm cursor-not-allowed opacity-70" disabled>
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
              <button className="bg-blue-300 text-white px-4 py-2 rounded text-sm cursor-not-allowed opacity-70" disabled>
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
              <button className="bg-green-300 text-white px-4 py-2 rounded text-sm cursor-not-allowed opacity-70" disabled>
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
                    {/* Show Generate or Re-Generate RCA based on status */}
                    {logFile.status === 'completed' && (logFile.rcaAnalysisStatus === 'not_started' || !logFile.rcaAnalysisStatus) && (
                      <button
                        onClick={() => handleAnalyze(logFile.id)}
                        className="bg-green-600 text-white px-3 py-1 rounded text-sm hover:bg-green-700"
                      >
                        Generate RCA
                      </button>
                    )}
                    {logFile.status === 'completed' && (logFile.rcaAnalysisStatus === 'completed' || logFile.rcaAnalysisStatus === 'failed') && (
                      <button
                        onClick={() => handleAnalyze(logFile.id)}
                        className="bg-yellow-600 text-white px-3 py-1 rounded text-sm hover:bg-yellow-700"
                      >
                        Re-Generate RCA
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
                      <>
                        <button
                          onClick={() => handleShowLLMAnalysis(logFile)}
                          className="bg-purple-600 text-white px-3 py-1 rounded text-sm hover:bg-purple-700"
                        >
                          View RCA
                        </button>
                        <button
                          onClick={() => handleDownloadRcaPdfForLog(logFile)}
                          className="bg-blue-500 hover:bg-blue-700 text-white px-3 py-1 rounded text-sm font-semibold shadow transition duration-150 ease-in-out"
                          title="Download RCA as PDF"
                        >
                          Download RCA PDF
                        </button>
                      </>
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
          <div className="bg-white rounded-lg shadow-lg w-full max-w-lg sm:max-w-xl md:max-w-2xl p-2 sm:p-4 md:p-6 relative overflow-y-auto max-h-[90vh]">
            <button
              onClick={() => setLlmModalOpen(false)}
              className="absolute top-2 right-2 text-gray-500 hover:text-gray-800 text-2xl z-10"
              aria-label="Close"
            >
              &times;
            </button>
            {llmModalAnalysis?.error ? (
              <div className="text-red-600">{llmModalAnalysis.error}</div>
            ) : llmModalAnalysis ? (
              <div className="space-y-4">
                {(() => {
                  // Robustly extract the analysis object
                  let analysis = llmModalAnalysis;
                  if (analysis && typeof analysis === 'object') {
                    if ('final' in analysis) analysis = analysis.final;
                    else if ('analysis' in analysis) analysis = analysis.analysis;
                  }
                  
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
                            <h3 className="text-lg font-medium text-green-800">No Errors Detected</h3>
                            <p className="text-sm text-green-700">Your log file contains no ERROR or FATAL entries</p>
                          </div>
                        </div>
                        
                        <div className="bg-white rounded-md p-3 border border-green-200">
                          <div className="mb-3">
                            <h4 className="text-sm font-medium text-gray-700 mb-1">Summary</h4>
                            <p className="text-sm text-gray-600">{analysis.summary}</p>
                          </div>
                          
                          <div className="mb-3">
                            <h4 className="text-sm font-medium text-gray-700 mb-1">Status</h4>
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                              ✅ System Healthy - No RCA Needed
                            </span>
                          </div>
                          
                          {Array.isArray(analysis.recommendations) && analysis.recommendations.length > 0 && (
                            <div>
                              <h4 className="text-sm font-medium text-gray-700 mb-1">Recommendations</h4>
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
      {showDeleteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-40">
          <div className="bg-white rounded-lg shadow-lg p-6 w-full max-w-sm relative">
            <h2 className="text-lg font-semibold mb-4">Delete Log File</h2>
            <p className="mb-4">Are you sure you want to delete <span className="font-bold">{deleteTargetLogFile?.filename}</span>?</p>
            <label className="flex items-center mb-4">
              <input
                type="checkbox"
                checked={hardDelete}
                onChange={e => setHardDelete(e.target.checked)}
                className="mr-2"
              />
              <span>Hard Delete (remove all related entries, jobs, and data from DB)</span>
            </label>
            <div className="flex justify-end space-x-2">
              <button
                onClick={() => { setShowDeleteModal(false); setDeleteTargetLogFile(null); setHardDelete(false); }}
                className="bg-gray-300 text-gray-800 px-4 py-2 rounded hover:bg-gray-400"
              >
                Cancel
              </button>
              <button
                onClick={confirmDelete}
                className="bg-red-600 text-white px-4 py-2 rounded hover:bg-red-700"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Logs; 