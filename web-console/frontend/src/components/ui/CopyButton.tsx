'use client'

import { useState, useEffect, useCallback } from 'react'

interface CopyButtonProps {
  text: string
  className?: string
}

export function CopyButton({ text, className = '' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  // Cleanup timeout on unmount
  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(timer)
  }, [copied])

  const handleCopy = useCallback(async () => {
    // Check if clipboard API is available
    if (!navigator.clipboard) {
      console.error('Clipboard API not available')
      return
    }

    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }, [text])

  return (
    <button
      type="button"
      onClick={handleCopy}
      className={`inline-flex items-center justify-center p-1 rounded hover:bg-gray-200 transition-colors shrink-0 ${className}`}
      aria-label={copied ? 'Copied to clipboard' : 'Copy to clipboard'}
      title={copied ? 'Copied!' : 'Copy to clipboard'}
    >
      {copied ? (
        <svg
          className="h-4 w-4 text-green-600"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      ) : (
        <svg
          className="h-4 w-4 text-gray-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
          />
        </svg>
      )}
    </button>
  )
}
