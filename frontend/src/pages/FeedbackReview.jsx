import React, { useEffect, useState } from 'react';
import api from '../services/api';

const FeedbackReview = () => {
  const [feedback, setFeedback] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    fetchFeedback();
  }, []);

  const fetchFeedback = async () => {
    setLoading(true);
    setError('');
    try {
      const response = await api.get('/analyses/export/all');
      setFeedback(response.data.feedback || []);
    } catch (err) {
      setError('Failed to fetch feedback: ' + (err.response?.data?.error || err.message));
    } finally {
      setLoading(false);
    }
  };

  const handleExport = () => {
    const dataStr = 'data:text/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(feedback, null, 2));
    const downloadAnchorNode = document.createElement('a');
    downloadAnchorNode.setAttribute('href', dataStr);
    downloadAnchorNode.setAttribute('download', 'log_analysis_feedback.json');
    document.body.appendChild(downloadAnchorNode);
    downloadAnchorNode.click();
    downloadAnchorNode.remove();
  };

  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-2xl font-bold mb-6">Log Analysis Feedback Review</h1>
      <button
        className="bg-blue-600 text-white px-4 py-2 rounded mb-4 hover:bg-blue-700"
        onClick={handleExport}
        disabled={feedback.length === 0}
      >
        Export All Feedback (JSON)
      </button>
      {loading ? (
        <div>Loading feedback...</div>
      ) : error ? (
        <div className="text-red-600">{error}</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full border border-gray-200">
            <thead>
              <tr className="bg-gray-100">
                <th className="px-4 py-2 border">Analysis ID</th>
                <th className="px-4 py-2 border">User ID</th>
                <th className="px-4 py-2 border">Is Correct</th>
                <th className="px-4 py-2 border">Correction</th>
                <th className="px-4 py-2 border">Created At</th>
              </tr>
            </thead>
            <tbody>
              {feedback.map((fb) => (
                <tr key={fb.id}>
                  <td className="px-4 py-2 border">{fb.analysisMemoryId}</td>
                  <td className="px-4 py-2 border">{fb.userId || '-'}</td>
                  <td className="px-4 py-2 border">{fb.isCorrect ? 'Yes' : 'No'}</td>
                  <td className="px-4 py-2 border">{fb.correction}</td>
                  <td className="px-4 py-2 border">{new Date(fb.createdAt).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default FeedbackReview; 