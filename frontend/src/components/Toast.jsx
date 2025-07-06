import React from 'react';

const typeStyles = {
  success: 'bg-green-600',
  error: 'bg-red-600',
  warning: 'bg-yellow-500 text-black',
};

export default function Toast({ message, onClose, type = 'error' }) {
  if (!message) return null;
  return (
    <div className={`fixed top-6 right-6 z-50 ${typeStyles[type] || typeStyles.error} text-white px-4 py-2 rounded shadow-lg flex items-center animate-fade-in`}>
      <span>{message}</span>
      <button className="ml-4 text-white font-bold text-lg" onClick={onClose} aria-label="Close">&times;</button>
    </div>
  );
} 