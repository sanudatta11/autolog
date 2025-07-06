import React, { useContext, useState } from 'react'
import { Outlet, Link, useLocation } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'

// Create a polling context
export const PollingContext = React.createContext({ pollingEnabled: true, setPollingEnabled: () => {} });

function Layout() {
  const { user, logout } = useAuth()
  const location = useLocation()

  // Add polling state here so it is global
  const [pollingEnabled, setPollingEnabled] = useState(true);

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