'use client'

import Link from 'next/link'
import { Button, ButtonIcons } from '@/components/ui/Button'

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-background">
      {/* Hero Section */}
      <section className="relative overflow-hidden">
        {/* Background gradient effect */}
        <div className="absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-accent/5 pointer-events-none" />

        <div className="relative mx-auto max-w-7xl px-4 py-24 sm:px-6 lg:px-8 lg:py-32">
          <div className="text-center">
            {/* Logo */}
            <div className="mx-auto mb-8 flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-accent shadow-lg shadow-primary/25 animate-in">
              <svg className="w-8 h-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
              </svg>
            </div>

            {/* Main heading */}
            <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl lg:text-6xl animate-in">
              <span className="block text-foreground">OME Web Console</span>
              <span className="block mt-2 text-primary">
                Model Orchestration Made Simple
              </span>
            </h1>

            {/* Description */}
            <p className="mt-6 text-lg leading-8 text-muted-foreground max-w-2xl mx-auto animate-in animate-in-delay-1">
              Manage your AI models, serving runtimes, and inference services with a modern,
              intuitive interface. Deploy, monitor, and scale your ML infrastructure with ease.
            </p>

            {/* CTA Button */}
            <div className="mt-10 flex items-center justify-center gap-4 animate-in animate-in-delay-2">
              <Button href="/dashboard" size="lg" icon={ButtonIcons.arrowRight} iconPosition="right">
                Get Started
              </Button>
              <Button href="/models" variant="outline" size="lg">
                Browse Models
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className="relative py-24 sm:py-32 border-t border-border">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl text-foreground">
              Everything you need for ML operations
            </h2>
            <p className="mt-3 text-muted-foreground">
              Powerful tools to manage your AI infrastructure
            </p>
          </div>

          {/* Feature Cards */}
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {/* Models Card */}
            <Link
              href="/models"
              className="group relative overflow-hidden rounded-xl border border-border bg-card shadow-sm hover:shadow-md hover:border-primary/30 transition-all duration-300 animate-in"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-primary/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
              <div className="relative p-6">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
                    <svg className="h-6 w-6 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
                    </svg>
                  </div>
                  <svg className="w-5 h-5 text-muted-foreground group-hover:text-primary group-hover:translate-x-1 transition-all duration-150" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </div>
                <h3 className="text-lg font-semibold mb-2 text-foreground">Models</h3>
                <p className="text-sm text-muted-foreground">
                  Manage ClusterBaseModel and BaseModel resources. Import from HuggingFace or create custom models.
                </p>
              </div>
            </Link>

            {/* Runtimes Card */}
            <Link
              href="/runtimes"
              className="group relative overflow-hidden rounded-xl border border-border bg-card shadow-sm hover:shadow-md hover:border-accent/30 transition-all duration-300 animate-in animate-in-delay-1"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-accent/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
              <div className="relative p-6">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-accent/10">
                    <svg className="h-6 w-6 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
                    </svg>
                  </div>
                  <svg className="w-5 h-5 text-muted-foreground group-hover:text-accent group-hover:translate-x-1 transition-all duration-150" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </div>
                <h3 className="text-lg font-semibold mb-2 text-foreground">Runtimes</h3>
                <p className="text-sm text-muted-foreground">
                  Configure serving runtimes with custom containers, accelerators, and resource requirements.
                </p>
              </div>
            </Link>

            {/* Services Card */}
            <Link
              href="/services"
              className="group relative overflow-hidden rounded-xl border border-border bg-card shadow-sm hover:shadow-md hover:border-success/30 transition-all duration-300 animate-in animate-in-delay-2 sm:col-span-2 lg:col-span-1"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-success/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
              <div className="relative p-6">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-success/10">
                    <svg className="h-6 w-6 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M5.636 5.636a9 9 0 1012.728 0M12 3v9" />
                    </svg>
                  </div>
                  <svg className="w-5 h-5 text-muted-foreground group-hover:text-success group-hover:translate-x-1 transition-all duration-150" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </div>
                <h3 className="text-lg font-semibold mb-2 text-foreground">Inference Services</h3>
                <p className="text-sm text-muted-foreground">
                  Deploy and manage InferenceService resources. Monitor status and scale deployments.
                </p>
              </div>
            </Link>
          </div>
        </div>
      </section>

      {/* Stats Section */}
      <section className="relative py-16 sm:py-24 bg-muted/30">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-1 gap-8 sm:grid-cols-3">
            <div className="text-center">
              <div className="text-3xl font-semibold text-primary">Fast</div>
              <div className="mt-2 text-sm text-muted-foreground">Deploy models in seconds</div>
            </div>
            <div className="text-center">
              <div className="text-3xl font-semibold text-accent">Scalable</div>
              <div className="mt-2 text-sm text-muted-foreground">Auto-scaling infrastructure</div>
            </div>
            <div className="text-center">
              <div className="text-3xl font-semibold text-success">Reliable</div>
              <div className="mt-2 text-sm text-muted-foreground">Production-ready deployments</div>
            </div>
          </div>
        </div>
      </section>

      {/* Footer CTA */}
      <section className="relative py-16 border-t border-border">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="relative overflow-hidden rounded-2xl border border-border bg-card p-8 sm:p-12 text-center">
            <div className="absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-accent/5 pointer-events-none" />
            <div className="relative">
              <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl text-foreground">
                Ready to get started?
              </h2>
              <p className="mt-3 text-muted-foreground">
                Access your dashboard and start managing your ML infrastructure today.
              </p>
              <div className="mt-8">
                <Button href="/dashboard" size="lg" icon={ButtonIcons.arrowRight} iconPosition="right">
                  Open Dashboard
                </Button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}
