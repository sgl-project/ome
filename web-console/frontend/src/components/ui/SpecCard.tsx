import { ReactNode } from 'react'
import { CopyButton } from './CopyButton'

interface SpecCardProps {
  label: string
  children: ReactNode
  copyValue?: string
  title?: string
}

export function SpecCard({ label, children, copyValue, title }: SpecCardProps) {
  return (
    <div className="rounded-lg bg-gray-50 p-3">
      <dt className="text-xs text-gray-500 mb-1">{label}</dt>
      <dd
        className={`text-sm font-medium text-gray-900 ${copyValue ? 'flex items-start gap-2' : ''} ${title ? 'truncate' : ''}`}
        title={title}
      >
        {copyValue ? (
          <>
            <span className="break-all flex-1">{children}</span>
            <CopyButton text={copyValue} />
          </>
        ) : (
          children
        )}
      </dd>
    </div>
  )
}
