import React, { useContext, useState } from 'react'
import { Outlet, Link, useLocation } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import { updateApiBaseURL } from '../services/api'

// Create a polling context
export const PollingContext = React.createContext({ pollingEnabled: true, setPollingEnabled: () => {} });

function Layout() {
  const { user, logout } = useAuth()
  const location = useLocation()

  // Add polling state here so it is global
  const [pollingEnabled, setPollingEnabled] = useState(true);
  const [showSettings, setShowSettings] = useState(false);
  const [backendUrl, setBackendUrl] = useState(localStorage.getItem('backendUrl') || '');

  const navigation = [
    { name: 'Dashboard', href: '/', icon: '📊' },
    { name: 'Log Analysis', href: '/logs', icon: '🔍' },
    { name: 'Parsing Rules', href: '/parsing-rules', icon: '⚙️' },
    { name: 'LLM Status', href: '/llm-status', icon: '🤖' },
    { name: 'Users', href: '/users', icon: '👥' },
  ]

  // Add Feedback Review for admins
  if (user?.role === 'ADMIN') {
    navigation.push({ name: 'Feedback Review', href: '/feedback-review', icon: '📝' })
  }

  const handleBackendUrlChange = () => {
    if (backendUrl.trim()) {
      localStorage.setItem('backendUrl', backendUrl.trim());
      updateApiBaseURL(backendUrl.trim());
      setShowSettings(false);
      // Reload the page to ensure all components use the new URL
      window.location.reload();
    }
  };

  return (
    <PollingContext.Provider value={{ pollingEnabled, setPollingEnabled }}>
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center h-16">
            <div className="flex items-center">
              <img
                className="h-9 w-auto mr-3"
                src="/autolog.svg"
                alt="AutoLog Logo"
              />
              <h1 className="text-xl font-semibold text-gray-900">
                AutoLog
              </h1>
            </div>
            <div className="flex items-center space-x-4">
              {/* Polling toggle - modern switch style with clear label */}
              <div className="flex items-center space-x-2">
                <span className="text-xs text-gray-500 font-medium">Polling</span>
                <label className="relative inline-flex items-center cursor-pointer" aria-label="Toggle polling">
                  <input
                    type="checkbox"
                    className="sr-only peer"
                    checked={pollingEnabled}
                    onChange={() => setPollingEnabled(v => !v)}
                  />
                  <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-500 rounded-full peer peer-checked:bg-green-500 transition-colors duration-200 after:content-[''] after:absolute after:top-0.5 after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all after:duration-200 peer-checked:after:translate-x-full peer-checked:after:border-white"></div>
                </label>
              </div>
              
              {/* Settings button */}
              <button
                onClick={() => setShowSettings(!showSettings)}
                className="text-gray-500 hover:text-gray-700 p-2 rounded-md"
                title="Settings"
              >
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
              </button>
              
              <span className="text-sm text-gray-700">
                Welcome, {user?.firstName} {user?.lastName}
              </span>
              <button
                onClick={logout}
                className="text-sm text-gray-500 hover:text-gray-700"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Settings Modal */}
      {showSettings && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-40">
          <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-medium text-gray-900">Settings</h3>
              <button
                onClick={() => setShowSettings(false)}
                className="text-gray-400 hover:text-gray-600"
              >
                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            
            <div className="space-y-4">
              <div>
                <label htmlFor="backendUrl" className="block text-sm font-medium text-gray-700">
                  Backend URL
                </label>
                <input
                  id="backendUrl"
                  type="url"
                  value={backendUrl}
                  onChange={(e) => setBackendUrl(e.target.value)}
                  className="input mt-1 w-full"
                  placeholder="https://your-backend-url.com"
                />
                <p className="mt-1 text-xs text-gray-500">
                  Current backend server URL
                </p>
              </div>
              
              <div className="flex justify-end space-x-3">
                <button
                  onClick={() => setShowSettings(false)}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200"
                >
                  Cancel
                </button>
                <button
                  onClick={handleBackendUrlChange}
                  className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-md hover:bg-primary-700"
                >
                  Save
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="flex">
        {/* Sidebar */}
        <nav className="w-64 bg-white shadow-sm border-r border-gray-200 min-h-screen">
          <div className="p-4">
            <nav className="space-y-2">
              {navigation.map((item) => {
                const isActive = location.pathname === item.href
                return (
                  <Link
                    key={item.name}
                    to={item.href}
                    className={`flex items-center px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                      isActive
                        ? 'bg-primary-50 text-primary-700 border-r-2 border-primary-700'
                        : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
                    }`}
                  >
                    <span className="mr-3">{item.icon}</span>
                    {item.name}
                  </Link>
                )
              })}
            </nav>
          </div>
        </nav>

        {/* Main content */}
        <main className="flex-1 p-8">
          <Outlet />
        </main>
      </div>
    </div>
    </PollingContext.Provider>
  )
}

export default Layout 