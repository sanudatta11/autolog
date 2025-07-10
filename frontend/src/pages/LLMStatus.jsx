import React, { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { Link } from 'react-router-dom';
import api from '../services/api';
import LLMAPICalls from '../components/LLMAPICalls';

const LLMStatus = () => {
  const { token } = useAuth();
  const [llmStatus, setLlmStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchLLMStatus();
  }, []);

  const fetchLLMStatus = async () => {
    setLoading(true);
    try {
      const response = await api.get('/llm/status');
      setLlmStatus(response.data);
      setError(null);
    } catch (err) {
      if (err.response?.status === 400 && err.response?.data?.status === 'unconfigured') {
        setError('LLM endpoint not configured');
        setLlmStatus({ status: 'unconfigured', error: err.response.data.error });
      } else {
        setError('Failed to fetch LLM status: ' + err.message);
      }
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'healthy': return 'text-green-600';
      case 'unhealthy': return 'text-red-600';
      case 'unconfigured': return 'text-yellow-600';
      default: return 'text-gray-600';
    }
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'healthy': return '🟢';
      case 'unhealthy': return '🔴';
      case 'unconfigured': return '🟡';
      default: return '⚪';
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="text-center">Loading LLM status...</div>
      </div>
    );
  }

  // Show configuration prompt if LLM endpoint is not configured
  if (llmStatus?.status === 'unconfigured') {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-3xl font-bold">LLM Service Status</h1>
          <button
            onClick={fetchLLMStatus}
            className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700"
          >
            Refresh
          </button>
        </div>

        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-6 mb-6">
          <div className="flex items-center space-x-3 mb-4">
            <span className="text-2xl">🟡</span>
            <h2 className="text-xl font-semibold text-yellow-800">LLM Endpoint Not Configured</h2>
          </div>
          <p className="text-yellow-700 mb-4">
            {llmStatus.error || 'You need to configure your LLM endpoint before using LLM features.'}
          </p>
          <div className="flex space-x-4">
            <Link
              to="/settings"
              className="bg-yellow-600 text-white px-4 py-2 rounded hover:bg-yellow-700"
            >
              Configure LLM Endpoint
            </Link>
            <button
              onClick={fetchLLMStatus}
              className="bg-gray-600 text-white px-4 py-2 rounded hover:bg-gray-700"
            >
              Check Again
            </button>
          </div>
        </div>

        <div className="bg-blue-50 rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-3">How to Configure LLM Endpoint</h3>
          <ol className="list-decimal list-inside space-y-2 text-sm text-gray-700">
            <li>Go to <Link to="/settings" className="text-blue-600 hover:underline">Settings</Link></li>
            <li>Enter your LLM service URL (e.g., Ollama server address)</li>
            <li>Save the configuration</li>
            <li>Return to this page to check the status</li>
          </ol>
        </div>
      </div>
    );
  }

  if (error && llmStatus?.status !== 'unconfigured') {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-3xl font-bold">LLM Service Status</h1>
        <button
          onClick={fetchLLMStatus}
          className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700"
        >
          Refresh
        </button>
      </div>

      {/* Service Status */}
      <div className="bg-white rounded-lg shadow-md p-6 mb-6">
        <h2 className="text-xl font-semibold mb-4">Service Status</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="flex items-center space-x-3">
            <span className="text-2xl">{getStatusIcon(llmStatus.status)}</span>
            <div>
              <div className="font-medium">Status</div>
              <div className={`text-sm ${getStatusColor(llmStatus.status)}`}>
                {llmStatus.status}
              </div>
            </div>
          </div>
          <div>
            <div className="font-medium">User Endpoint</div>
            <div className="text-sm text-gray-600">{llmStatus.userEndpoint}</div>
            {llmStatus.lastLLMStatusCheck && (
              <div className="text-xs text-gray-500 mt-1">
                Last successful check: {new Date(llmStatus.lastLLMStatusCheck).toLocaleString()}
              </div>
            )}
          </div>
        </div>
        
        {llmStatus.healthError && (
          <div className="mt-4 p-3 bg-red-100 border border-red-400 text-red-700 rounded">
            <div className="font-medium">Health Error:</div>
            <div className="text-sm">{llmStatus.healthError}</div>
          </div>
        )}
      </div>

      {/* Current Model */}
      <div className="bg-white rounded-lg shadow-md p-6 mb-6">
        <h2 className="text-xl font-semibold mb-4">Current Configuration</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <div className="font-medium">Active Model</div>
            <div className="text-sm text-gray-600">{llmStatus.currentModel}</div>
          </div>
          <div>
            <div className="font-medium">Model Status</div>
            <div className="text-sm text-gray-600">
              {llmStatus.availableModels?.some(model => model.startsWith(llmStatus.currentModel))
                ? '✅ Available' 
                : '❌ Not Available'}
            </div>
          </div>
        </div>
      </div>

      {/* Available Models */}
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-xl font-semibold mb-4">Available Models</h2>
        {llmStatus.modelsError ? (
          <div className="p-3 bg-red-100 border border-red-400 text-red-700 rounded">
            <div className="font-medium">Error fetching models:</div>
            <div className="text-sm">{llmStatus.modelsError}</div>
          </div>
        ) : llmStatus.availableModels && llmStatus.availableModels.length > 0 ? (
          <div className="space-y-2">
            {llmStatus.availableModels.map((model, index) => (
              <div key={index} className="flex items-center justify-between p-3 border border-gray-200 rounded">
                <div className="flex items-center space-x-3">
                  <span className="text-green-600">✅</span>
                  <span className="font-medium">{model}</span>
                </div>
                <div className="text-sm text-gray-600">
                  {model === llmStatus.currentModel ? 'Active' : 'Available'}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-8 text-gray-500">
            No models available. Please install models using Ollama.
          </div>
        )}
      </div>

      {/* Model Installation Instructions */}
      <div className="bg-blue-50 rounded-lg p-6 mt-6">
        <h3 className="text-lg font-semibold mb-3">Install Additional Models</h3>
        <p className="text-sm text-gray-700 mb-3">
          To install additional models, you can use the following commands:
        </p>
        <div className="bg-gray-800 text-green-400 p-4 rounded font-mono text-sm">
          <div># Install popular models for better analysis:</div>
          <div>docker exec autolog-ollama ollama pull llama2:13b</div>
          <div>docker exec autolog-ollama ollama pull mistral:7b</div>
          <div>docker exec autolog-ollama ollama pull codellama:7b</div>
          <div>docker exec autolog-ollama ollama pull neural-chat:7b</div>
        </div>
        <p className="text-xs text-gray-600 mt-2">
          Note: Larger models provide better analysis but require more memory and processing time.
        </p>
      </div>

      {/* LLM API Calls Section */}
      <div className="mt-8">
        <LLMAPICalls />
      </div>
    </div>
  );
};

export default LLMStatus; 