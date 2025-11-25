import Link from 'next/link'

interface ErrorStateProps {
  error: Error | unknown
  backLink?: {
    href: string
    label: string
  }
  className?: string
}

export function ErrorState({ error, backLink, className = '' }: ErrorStateProps) {
  const errorMessage = error instanceof Error ? error.message : 'An unexpected error occurred'

  return (
    <div className={`flex min-h-screen items-center justify-center ${className}`}>
      <div className="text-center">
        <div className="text-lg text-red-600 mb-4">{errorMessage}</div>
        {backLink && (
          <Link href={backLink.href} className="text-blue-600 hover:text-blue-800">
            ← {backLink.label}
          </Link>
        )}
      </div>
    </div>
  )
}
