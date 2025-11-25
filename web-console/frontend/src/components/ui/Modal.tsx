'use client'

import { useEffect } from 'react'

interface ModalProps {
  isOpen: boolean
  onClose: () => void
  title: string
  children: React.ReactNode
}

export function Modal({ isOpen, onClose, title, children }: ModalProps) {
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }

    if (isOpen) {
      document.addEventListener('keydown', handleEscape)
      document.body.style.overflow = 'hidden'
    }

    return () => {
      document.removeEventListener('keydown', handleEscape)
      document.body.style.overflow = 'unset'
    }
  }, [isOpen, onClose])

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black bg-opacity-50 transition-opacity" onClick={onClose} />

      {/* Modal */}
      <div className="flex min-h-full items-center justify-center p-4">
        <div className="relative w-full max-w-md transform rounded-lg bg-white p-6 shadow-xl transition-all">
          <h3 className="text-lg font-medium leading-6 text-gray-900 mb-4">{title}</h3>
          {children}
        </div>
      </div>
    </div>
  )
}

interface ConfirmDeleteModalProps {
  isOpen: boolean
  onClose: () => void
  onConfirm: () => void
  resourceName: string
  resourceType?: string
  isDeleting?: boolean
}

export function ConfirmDeleteModal({
  isOpen,
  onClose,
  onConfirm,
  resourceName,
  resourceType = 'resource',
  isDeleting = false,
}: ConfirmDeleteModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Confirm Deletion">
      <div className="mb-4">
        <p className="text-sm text-gray-500">
          Are you sure you want to delete{' '}
          <span className="font-semibold text-gray-900">{resourceName}</span>?
        </p>
        <p className="mt-2 text-sm text-gray-500">
          This {resourceType} will be permanently removed. This action cannot be undone.
        </p>
      </div>

      <div className="flex justify-end gap-3">
        <button
          type="button"
          onClick={onClose}
          disabled={isDeleting}
          className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={onConfirm}
          disabled={isDeleting}
          className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:bg-red-400"
        >
          {isDeleting ? 'Deleting...' : 'Delete'}
        </button>
      </div>
    </Modal>
  )
}
