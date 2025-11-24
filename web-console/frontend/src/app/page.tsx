'use client'

import Link from 'next/link'

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-background via-background to-muted/20">
      {/* Hero Section */}
      <section className="relative overflow-hidden">
        {/* Background gradient effect */}
        <div className="absolute inset-0 bg-gradient-to-br from-primary/10 via-transparent to-accent/10 pointer-events-none" />

        <div className="relative mx-auto max-w-7xl px-4 py-24 sm:px-6 lg:px-8 lg:py-32">
          <div className="text-center">
            {/* Main heading */}
            <h1 className="text-5xl font-bold tracking-tight sm:text-6xl lg:text-7xl animate-in">
              <span className="block">OME Web Console</span>
              <span className="block mt-2 bg-gradient-to-r from-primary via-accent to-primary bg-clip-text text-transparent">
                Model Orchestration Made Simple
              </span>
            </h1>

            {/* Description */}
            <p className="mt-6 text-lg leading-8 text-muted-foreground max-w-3xl mx-auto animate-in animate-in-delay-1">
              Manage your AI models, serving runtimes, and inference services with a modern,
              intuitive interface. Deploy, monitor, and scale your ML infrastructure with ease.
            </p>

            {/* CTA Button */}
            <div className="mt-10 flex items-center justify-center gap-x-6 animate-in animate-in-delay-2">
              <Link
                href="/dashboard"
                className="gradient-border relative rounded-lg bg-gradient-to-r from-primary to-accent px-8 py-4 text-lg font-medium text-white hover:shadow-lg hover:shadow-primary/25 transition-all group"
              >
                <span className="flex items-center gap-2">
                  Get Started
                  <svg
                    className="w-5 h-5 group-hover:translate-x-1 transition-transform duration-150"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </span>
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className="relative py-24 sm:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
              Everything you need for ML operations
            </h2>
            <p className="mt-4 text-lg text-muted-foreground">
              Powerful tools to manage your AI infrastructure
            </p>
          </div>

          {/* Feature Cards */}
          <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
            {/* Models Card */}
            <Link
              href="/models"
              className="group relative overflow-hidden rounded-2xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-xl transition-all duration-300 animate-in"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-primary/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
              <div className="relative p-8">
                <div className="flex items-center justify-between mb-4">
                  <div className="p-3 rounded-xl bg-primary/10 ring-1 ring-primary/20">
                    <svg className="h-8 w-8 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                    </svg>
                  </div>
                  <svg
                    className="w-5 h-5 text-muted-foreground group-hover:translate-x-1 group-hover:text-primary transition-all duration-150"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </div>
                <h3 className="text-xl font-semibold mb-2">Models</h3>
                <p className="text-muted-foreground">
                  Manage ClusterBaseModel and BaseModel resources. Import from HuggingFace or create custom models.
                </p>
              </div>
            </Link>

            {/* Runtimes Card */}
            <Link
              href="/runtimes"
              className="group relative overflow-hidden rounded-2xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-xl transition-all duration-300 animate-in animate-in-delay-1"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-purple-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
              <div className="relative p-8">
                <div className="flex items-center justify-between mb-4">
                  <div className="p-3 rounded-xl bg-purple-500/10 ring-1 ring-purple-500/20">
                    <svg className="h-8 w-8 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                    </svg>
                  </div>
                  <svg
                    className="w-5 h-5 text-muted-foreground group-hover:translate-x-1 group-hover:text-purple-600 transition-all duration-150"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </div>
                <h3 className="text-xl font-semibold mb-2">Runtimes</h3>
                <p className="text-muted-foreground">
                  Configure serving runtimes with custom containers, accelerators, and resource requirements.
                </p>
              </div>
            </Link>

            {/* Services Card */}
            <Link
              href="/services"
              className="group relative overflow-hidden rounded-2xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-xl transition-all duration-300 animate-in animate-in-delay-2 sm:col-span-2 lg:col-span-1"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-orange-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
              <div className="relative p-8">
                <div className="flex items-center justify-between mb-4">
                  <div className="p-3 rounded-xl bg-orange-500/10 ring-1 ring-orange-500/20">
                    <svg className="h-8 w-8 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                    </svg>
                  </div>
                  <svg
                    className="w-5 h-5 text-muted-foreground group-hover:translate-x-1 group-hover:text-orange-600 transition-all duration-150"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </div>
                <h3 className="text-xl font-semibold mb-2">Inference Services</h3>
                <p className="text-muted-foreground">
                  Deploy and manage InferenceService resources. Monitor status and scale deployments.
                </p>
              </div>
            </Link>
          </div>
        </div>
      </section>

      {/* Stats Section */}
      <section className="relative py-16 sm:py-24 border-t border-border/50">
        <div className="absolute inset-0 bg-gradient-to-b from-muted/20 to-transparent pointer-events-none" />
        <div className="relative mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-1 gap-8 sm:grid-cols-3">
            <div className="text-center">
              <div className="text-4xl font-bold bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
                Fast
              </div>
              <div className="mt-2 text-sm text-muted-foreground">
                Deploy models in seconds
              </div>
            </div>
            <div className="text-center">
              <div className="text-4xl font-bold bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
                Scalable
              </div>
              <div className="mt-2 text-sm text-muted-foreground">
                Auto-scaling infrastructure
              </div>
            </div>
            <div className="text-center">
              <div className="text-4xl font-bold bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
                Reliable
              </div>
              <div className="mt-2 text-sm text-muted-foreground">
                Production-ready deployments
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Footer CTA */}
      <section className="relative py-16">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="relative overflow-hidden rounded-2xl border border-border/50 bg-card/80 backdrop-blur-sm p-12 text-center">
            <div className="absolute inset-0 bg-gradient-to-br from-primary/10 via-transparent to-accent/10 pointer-events-none" />
            <div className="relative">
              <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
                Ready to get started?
              </h2>
              <p className="mt-4 text-lg text-muted-foreground">
                Access your dashboard and start managing your ML infrastructure today.
              </p>
              <div className="mt-8">
                <Link
                  href="/dashboard"
                  className="gradient-border relative rounded-lg bg-gradient-to-r from-primary to-accent px-8 py-4 text-lg font-medium text-white hover:shadow-lg hover:shadow-primary/25 transition-all inline-flex items-center gap-2 group"
                >
                  Open Dashboard
                  <svg
                    className="w-5 h-5 group-hover:translate-x-1 transition-transform duration-150"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}
