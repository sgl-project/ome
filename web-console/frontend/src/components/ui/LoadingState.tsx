interface LoadingStateProps {
  message?: string
  className?: string
}

export function LoadingState({ message = 'Loading...', className = '' }: LoadingStateProps) {
  return (
    <div className={`flex min-h-screen items-center justify-center ${className}`}>
      <div className="text-lg">{message}</div>
    </div>
  )
}
