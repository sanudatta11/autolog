import React, { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
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
      setError('Failed to fetch LLM status: ' + err.message);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'healthy': return 'text-green-600';
      case 'unhealthy': return 'text-red-600';
      default: return 'text-gray-600';
    }
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'healthy': return '🟢';
      case 'unhealthy': return '🔴';
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

  if (error) {
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
            <div className="font-medium">Ollama URL</div>
            <div className="text-sm text-gray-600">{llmStatus.ollamaUrl}</div>
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