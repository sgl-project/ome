'use client'

import Link from 'next/link'
import Image from 'next/image'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { Icons } from '@/components/ui/Icons'

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-background via-background to-muted/20 flex flex-col">
      {/* Hero Section */}
      <section className="relative overflow-hidden flex-1 flex items-center">
        {/* Background decorations */}
        <div className="absolute inset-0 overflow-hidden pointer-events-none">
          <div className="absolute -top-40 -right-40 w-80 h-80 bg-primary/10 rounded-full blur-3xl" />
          <div className="absolute -bottom-40 -left-40 w-80 h-80 bg-accent/10 rounded-full blur-3xl" />
        </div>

        <div className="relative max-w-5xl mx-auto px-6 py-12 sm:py-16 w-full">
          <div className="text-center">
            {/* Logo */}
            <div className="flex justify-center mb-10">
              <div className="relative">
                <div className="absolute inset-0 bg-primary/20 rounded-3xl blur-xl scale-110" />
                <Image
                  src="/logo.png"
                  alt="OME Logo"
                  width={100}
                  height={100}
                  className="relative rounded-3xl shadow-2xl"
                />
              </div>
            </div>

            {/* Main heading */}
            <h1 className="text-5xl font-bold tracking-tight sm:text-6xl lg:text-7xl">
              <span className="text-foreground">OME</span>
              <span className="text-primary"> Console</span>
            </h1>

            <p className="mt-6 text-xl sm:text-2xl text-muted-foreground font-light">
              Model Orchestration Made Simple
            </p>

            {/* Description */}
            <p className="mt-6 text-base text-muted-foreground/80 max-w-xl mx-auto leading-relaxed">
              Deploy and manage AI models, serving runtimes, and inference services with a modern,
              intuitive interface.
            </p>

            {/* CTA Buttons */}
            <div className="mt-12 flex flex-col sm:flex-row items-center justify-center gap-4">
              <Button
                href="/dashboard"
                size="lg"
                icon={ButtonIcons.arrowRight}
                iconPosition="right"
                className="w-full sm:w-auto px-8"
              >
                Open Dashboard
              </Button>
              <Button href="/models" variant="outline" size="lg" className="w-full sm:w-auto px-8">
                Browse Models
              </Button>
            </div>

            {/* Links */}
            <div className="mt-8 flex items-center justify-center gap-6 text-sm">
              <a
                href="https://docs.sglang.io/ome/"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors"
              >
                <Icons.document size="sm" />
                <span>Documentation</span>
              </a>
              <a
                href="https://github.com/sgl-project/ome"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors"
              >
                <Icons.github size="sm" />
                <span>GitHub</span>
              </a>
            </div>
          </div>
        </div>
      </section>

      {/* Feature Cards Section */}
      <section className="relative max-w-5xl mx-auto px-6 pb-8">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
          {/* Models Card */}
          <Link
            href="/models"
            className="group relative overflow-hidden rounded-2xl border border-border/50 bg-card/50 backdrop-blur-sm p-6 hover:border-primary/50 hover:bg-card transition-all duration-300 hover:shadow-lg hover:shadow-primary/5"
          >
            <div className="absolute top-0 right-0 w-32 h-32 bg-primary/5 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2 group-hover:bg-primary/10 transition-colors" />
            <div className="relative">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 mb-5 group-hover:bg-primary/20 transition-colors">
                <Icons.cube size="lg" className="text-primary" />
              </div>
              <h3 className="text-xl font-semibold text-foreground mb-2">Models</h3>
              <p className="text-sm text-muted-foreground leading-relaxed">
                Manage ClusterBaseModel and BaseModel resources. Import directly from HuggingFace.
              </p>
              <div className="mt-4 flex items-center text-sm font-medium text-primary opacity-0 group-hover:opacity-100 transition-opacity">
                <span>Explore models</span>
                <Icons.arrowRight size="sm" className="ml-1" />
              </div>
            </div>
          </Link>

          {/* Runtimes Card */}
          <Link
            href="/runtimes"
            className="group relative overflow-hidden rounded-2xl border border-border/50 bg-card/50 backdrop-blur-sm p-6 hover:border-accent/50 hover:bg-card transition-all duration-300 hover:shadow-lg hover:shadow-accent/5"
          >
            <div className="absolute top-0 right-0 w-32 h-32 bg-accent/5 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2 group-hover:bg-accent/10 transition-colors" />
            <div className="relative">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-accent/10 mb-5 group-hover:bg-accent/20 transition-colors">
                <Icons.server size="lg" className="text-accent" />
              </div>
              <h3 className="text-xl font-semibold text-foreground mb-2">Runtimes</h3>
              <p className="text-sm text-muted-foreground leading-relaxed">
                Configure serving runtimes with custom containers and GPU accelerators.
              </p>
              <div className="mt-4 flex items-center text-sm font-medium text-accent opacity-0 group-hover:opacity-100 transition-opacity">
                <span>View runtimes</span>
                <Icons.arrowRight size="sm" className="ml-1" />
              </div>
            </div>
          </Link>

          {/* Services Card */}
          <Link
            href="/services"
            className="group relative overflow-hidden rounded-2xl border border-border/50 bg-card/50 backdrop-blur-sm p-6 hover:border-success/50 hover:bg-card transition-all duration-300 hover:shadow-lg hover:shadow-success/5"
          >
            <div className="absolute top-0 right-0 w-32 h-32 bg-success/5 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2 group-hover:bg-success/10 transition-colors" />
            <div className="relative">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-success/10 mb-5 group-hover:bg-success/20 transition-colors">
                <Icons.power size="lg" className="text-success" />
              </div>
              <h3 className="text-xl font-semibold text-foreground mb-2">Services</h3>
              <p className="text-sm text-muted-foreground leading-relaxed">
                Deploy, monitor, and scale InferenceService deployments with ease.
              </p>
              <div className="mt-4 flex items-center text-sm font-medium text-success opacity-0 group-hover:opacity-100 transition-opacity">
                <span>Manage services</span>
                <Icons.arrowRight size="sm" className="ml-1" />
              </div>
            </div>
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-border/50 bg-muted/20 mt-auto">
        <div className="max-w-5xl mx-auto px-6 py-5 flex items-center justify-between text-sm text-muted-foreground">
          <div className="flex items-center gap-3">
            <Image
              src="/logo.png"
              alt="OME"
              width={24}
              height={24}
              className="rounded-md opacity-60"
            />
            <span>OME Console</span>
          </div>
          <span className="text-muted-foreground/60">Open Model Engine</span>
        </div>
      </footer>
    </div>
  )
}
