import React, { useState, useEffect } from 'react'
import { parsingRulesAPI } from '../services/api'

function ParsingRules() {
  const [rules, setRules] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showTestModal, setShowTestModal] = useState(false)
  const [selectedRule, setSelectedRule] = useState(null)
  const [testResult, setTestResult] = useState(null)
  const [testLoading, setTestLoading] = useState(false)

  // Form state for creating/editing rules
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    isActive: true,
    isTemplate: false,
    fieldMappings: [],
    regexPatterns: []
  })

  // Test form state
  const [testData, setTestData] = useState({
    sampleLogs: ''
  })

  useEffect(() => {
    loadParsingRules()
  }, [])

  const loadParsingRules = async () => {
    try {
      setLoading(true)
      const response = await parsingRulesAPI.getUserParsingRules()
      setRules(response.data.rules || [])
    } catch (err) {
      setError('Failed to load parsing rules')
      console.error('Error loading parsing rules:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleCreateRule = async () => {
    try {
      await parsingRulesAPI.createParsingRule(formData)
      setShowCreateModal(false)
      resetForm()
      loadParsingRules()
    } catch (err) {
      setError('Failed to create parsing rule')
      console.error('Error creating parsing rule:', err)
    }
  }

  const handleUpdateRule = async (id) => {
    try {
      await parsingRulesAPI.updateParsingRule(id, formData)
      setShowCreateModal(false)
      resetForm()
      loadParsingRules()
    } catch (err) {
      setError('Failed to update parsing rule')
      console.error('Error updating parsing rule:', err)
    }
  }

  const handleDeleteRule = async (id) => {
    if (!window.confirm('Are you sure you want to delete this parsing rule?')) {
      return
    }

    try {
      await parsingRulesAPI.deleteParsingRule(id)
      loadParsingRules()
    } catch (err) {
      setError('Failed to delete parsing rule')
      console.error('Error deleting parsing rule:', err)
    }
  }

  const handleTestRule = async () => {
    if (!selectedRule) return

    try {
      setTestLoading(true)
      const sampleLogs = testData.sampleLogs.split('\n').filter(line => line.trim())
      const response = await parsingRulesAPI.testParsingRule(selectedRule, sampleLogs)
      setTestResult(response.data.result)
    } catch (err) {
      setError('Failed to test parsing rule')
      console.error('Error testing parsing rule:', err)
    } finally {
      setTestLoading(false)
    }
  }

  const resetForm = () => {
    setFormData({
      name: '',
      description: '',
      isActive: true,
      isTemplate: false,
      fieldMappings: [],
      regexPatterns: []
    })
  }

  const addFieldMapping = () => {
    setFormData(prev => ({
      ...prev,
      fieldMappings: [...prev.fieldMappings, { sourceField: '', targetField: '', description: '', isActive: true }]
    }))
  }

  const removeFieldMapping = (index) => {
    setFormData(prev => ({
      ...prev,
      fieldMappings: prev.fieldMappings.filter((_, i) => i !== index)
    }))
  }

  const updateFieldMapping = (index, field, value) => {
    setFormData(prev => ({
      ...prev,
      fieldMappings: prev.fieldMappings.map((mapping, i) => 
        i === index ? { ...mapping, [field]: value } : mapping
      )
    }))
  }

  const addRegexPattern = () => {
    setFormData(prev => ({
      ...prev,
      regexPatterns: [...prev.regexPatterns, { name: '', pattern: '', description: '', priority: 0, isActive: true }]
    }))
  }

  const removeRegexPattern = (index) => {
    setFormData(prev => ({
      ...prev,
      regexPatterns: prev.regexPatterns.filter((_, i) => i !== index)
    }))
  }

  const updateRegexPattern = (index, field, value) => {
    setFormData(prev => ({
      ...prev,
      regexPatterns: prev.regexPatterns.map((pattern, i) => 
        i === index ? { ...pattern, [field]: value } : pattern
      )
    }))
  }

  const openEditModal = (rule) => {
    setFormData({
      name: rule.name,
      description: rule.description,
      isActive: rule.isActive,
      isTemplate: rule.isTemplate,
      fieldMappings: rule.fieldMappings || [],
      regexPatterns: rule.regexPatterns || []
    })
    setSelectedRule(rule)
    setShowCreateModal(true)
  }

  const openTestModal = (rule) => {
    setSelectedRule(rule)
    setTestData({ sampleLogs: '' })
    setTestResult(null)
    setShowTestModal(true)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-lg">Loading parsing rules...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Parsing Rules</h1>
          <p className="text-gray-600">Manage custom parsing rules for log processing</p>
        </div>
        <button
          onClick={() => {
            resetForm()
            setSelectedRule(null)
            setShowCreateModal(true)
          }}
          className="bg-primary-600 text-white px-4 py-2 rounded-md hover:bg-primary-700 transition-colors"
        >
          Create Rule
        </button>
      </div>

      {/* Error Message */}
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          {error}
        </div>
      )}

      {/* Rules List */}
      <div className="bg-white shadow rounded-lg">
        {rules.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            <p className="text-lg mb-2">No parsing rules found</p>
            <p>Create your first parsing rule to get started</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Description
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Field Mappings
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Regex Patterns
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {rules.map((rule) => (
                  <tr key={rule.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900">{rule.name}</div>
                      {rule.isTemplate && (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                          Template
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      <div className="text-sm text-gray-900">{rule.description}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        rule.isActive 
                          ? 'bg-green-100 text-green-800' 
                          : 'bg-gray-100 text-gray-800'
                      }`}>
                        {rule.isActive ? 'Active' : 'Inactive'}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                      {rule.fieldMappings?.length || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                      {rule.regexPatterns?.length || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                      <button
                        onClick={() => openTestModal(rule)}
                        className="text-primary-600 hover:text-primary-900"
                      >
                        Test
                      </button>
                      <button
                        onClick={() => openEditModal(rule)}
                        className="text-indigo-600 hover:text-indigo-900"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDeleteRule(rule.id)}
                        className="text-red-600 hover:text-red-900"
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Create/Edit Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50">
          <div className="relative top-20 mx-auto p-5 border w-11/12 max-w-4xl shadow-lg rounded-md bg-white">
            <div className="mt-3">
              <h3 className="text-lg font-medium text-gray-900 mb-4">
                {selectedRule ? 'Edit Parsing Rule' : 'Create Parsing Rule'}
              </h3>
              
              <div className="space-y-4">
                {/* Basic Info */}
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Name</label>
                    <input
                      type="text"
                      value={formData.name}
                      onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
                      className="mt-1 block w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Status</label>
                    <select
                      value={formData.isActive}
                      onChange={(e) => setFormData(prev => ({ ...prev, isActive: e.target.value === 'true' }))}
                      className="mt-1 block w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                    >
                      <option value={true}>Active</option>
                      <option value={false}>Inactive</option>
                    </select>
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700">Description</label>
                  <textarea
                    value={formData.description}
                    onChange={(e) => setFormData(prev => ({ ...prev, description: e.target.value }))}
                    rows={3}
                    className="mt-1 block w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  />
                </div>

                {/* Field Mappings */}
                <div>
                  <div className="flex justify-between items-center mb-2">
                    <h4 className="text-md font-medium text-gray-900">Field Mappings</h4>
                    <button
                      onClick={addFieldMapping}
                      className="text-sm bg-primary-600 text-white px-3 py-1 rounded hover:bg-primary-700"
                    >
                      Add Mapping
                    </button>
                  </div>
                  <div className="space-y-2">
                    {formData.fieldMappings.map((mapping, index) => (
                      <div key={index} className="flex items-center space-x-2 p-3 border rounded">
                        <input
                          type="text"
                          placeholder="Source Field"
                          value={mapping.sourceField}
                          onChange={(e) => updateFieldMapping(index, 'sourceField', e.target.value)}
                          className="flex-1 border border-gray-300 rounded px-2 py-1 text-sm"
                        />
                        <span className="text-gray-500">→</span>
                        <input
                          type="text"
                          placeholder="Target Field"
                          value={mapping.targetField}
                          onChange={(e) => updateFieldMapping(index, 'targetField', e.target.value)}
                          className="flex-1 border border-gray-300 rounded px-2 py-1 text-sm"
                        />
                        <input
                          type="text"
                          placeholder="Description"
                          value={mapping.description}
                          onChange={(e) => updateFieldMapping(index, 'description', e.target.value)}
                          className="flex-1 border border-gray-300 rounded px-2 py-1 text-sm"
                        />
                        <button
                          onClick={() => removeFieldMapping(index)}
                          className="text-red-600 hover:text-red-800"
                        >
                          ✕
                        </button>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Regex Patterns */}
                <div>
                  <div className="flex justify-between items-center mb-2">
                    <h4 className="text-md font-medium text-gray-900">Regex Patterns</h4>
                    <button
                      onClick={addRegexPattern}
                      className="text-sm bg-primary-600 text-white px-3 py-1 rounded hover:bg-primary-700"
                    >
                      Add Pattern
                    </button>
                  </div>
                  <div className="space-y-2">
                    {formData.regexPatterns.map((pattern, index) => (
                      <div key={index} className="p-3 border rounded space-y-2">
                        <div className="flex items-center space-x-2">
                          <input
                            type="text"
                            placeholder="Pattern Name"
                            value={pattern.name}
                            onChange={(e) => updateRegexPattern(index, 'name', e.target.value)}
                            className="flex-1 border border-gray-300 rounded px-2 py-1 text-sm"
                          />
                          <input
                            type="number"
                            placeholder="Priority"
                            value={pattern.priority}
                            onChange={(e) => updateRegexPattern(index, 'priority', parseInt(e.target.value))}
                            className="w-20 border border-gray-300 rounded px-2 py-1 text-sm"
                          />
                          <button
                            onClick={() => removeRegexPattern(index)}
                            className="text-red-600 hover:text-red-800"
                          >
                            ✕
                          </button>
                        </div>
                        <input
                          type="text"
                          placeholder="Regex Pattern"
                          value={pattern.pattern}
                          onChange={(e) => updateRegexPattern(index, 'pattern', e.target.value)}
                          className="w-full border border-gray-300 rounded px-2 py-1 text-sm"
                        />
                        <input
                          type="text"
                          placeholder="Description"
                          value={pattern.description}
                          onChange={(e) => updateRegexPattern(index, 'description', e.target.value)}
                          className="w-full border border-gray-300 rounded px-2 py-1 text-sm"
                        />
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              <div className="flex justify-end space-x-3 mt-6">
                <button
                  onClick={() => setShowCreateModal(false)}
                  className="px-4 py-2 text-gray-700 bg-gray-200 rounded-md hover:bg-gray-300"
                >
                  Cancel
                </button>
                <button
                  onClick={() => selectedRule ? handleUpdateRule(selectedRule.id) : handleCreateRule()}
                  className="px-4 py-2 bg-primary-600 text-white rounded-md hover:bg-primary-700"
                >
                  {selectedRule ? 'Update' : 'Create'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Test Modal */}
      {showTestModal && (
        <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50">
          <div className="relative top-20 mx-auto p-5 border w-11/12 max-w-4xl shadow-lg rounded-md bg-white">
            <div className="mt-3">
              <h3 className="text-lg font-medium text-gray-900 mb-4">
                Test Parsing Rule: {selectedRule?.name}
              </h3>
              
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Sample Log Lines (one per line)
                  </label>
                  <textarea
                    value={testData.sampleLogs}
                    onChange={(e) => setTestData(prev => ({ ...prev, sampleLogs: e.target.value }))}
                    rows={8}
                    className="w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                    placeholder="Enter sample log lines here..."
                  />
                </div>

                {testResult && (
                  <div className="bg-gray-50 p-4 rounded-md">
                    <h4 className="font-medium text-gray-900 mb-2">Test Results</h4>
                    <div className="grid grid-cols-3 gap-4 text-sm">
                      <div>
                        <span className="text-gray-600">Total Logs:</span>
                        <span className="ml-2 font-medium">{testResult.totalLogs}</span>
                      </div>
                      <div>
                        <span className="text-gray-600">Success:</span>
                        <span className="ml-2 font-medium text-green-600">{testResult.successCount}</span>
                      </div>
                      <div>
                        <span className="text-gray-600">Failure:</span>
                        <span className="ml-2 font-medium text-red-600">{testResult.failureCount}</span>
                      </div>
                    </div>
                    
                    {testResult.details && testResult.details.length > 0 && (
                      <div className="mt-4">
                        <h5 className="font-medium text-gray-900 mb-2">Details</h5>
                        <div className="max-h-64 overflow-y-auto space-y-2">
                          {testResult.details.map((detail, index) => (
                            <div key={index} className={`p-2 rounded text-sm ${
                              detail.success ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'
                            }`}>
                              <div className="font-medium">Line {detail.logIndex + 1}:</div>
                              <div className="font-mono text-xs mt-1">{detail.logLine}</div>
                              {detail.matchedPattern && (
                                <div className="text-xs mt-1">Matched: {detail.matchedPattern}</div>
                              )}
                              {detail.errors && detail.errors.length > 0 && (
                                <div className="text-xs mt-1">
                                  Errors: {detail.errors.join(', ')}
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                <div className="flex justify-end space-x-3">
                  <button
                    onClick={() => setShowTestModal(false)}
                    className="px-4 py-2 text-gray-700 bg-gray-200 rounded-md hover:bg-gray-300"
                  >
                    Close
                  </button>
                  <button
                    onClick={handleTestRule}
                    disabled={testLoading || !testData.sampleLogs.trim()}
                    className="px-4 py-2 bg-primary-600 text-white rounded-md hover:bg-primary-700 disabled:opacity-50"
                  >
                    {testLoading ? 'Testing...' : 'Test Rule'}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default ParsingRules 