import React from 'react';

export default function Toast({ message, onClose }) {
  if (!message) return null;
  return (
    <div className="fixed top-6 right-6 z-50 bg-red-600 text-white px-4 py-2 rounded shadow-lg flex items-center animate-fade-in">
      <span>{message}</span>
      <button className="ml-4 text-white font-bold text-lg" onClick={onClose} aria-label="Close">&times;</button>
    </div>
  );
} 