import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Separator } from '@/components/ui/separator'

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container flex h-14 items-center gap-3">
          <Link to="/" className="flex items-center gap-2 text-foreground hover:text-primary transition-colors">
            <span className="text-xl font-bold tracking-tight">AgentLens</span>
          </Link>
          <Separator orientation="vertical" className="h-6" />
          <span className="text-muted-foreground text-sm">AI Agent Catalog</span>
        </div>
      </header>
      <main className="container py-6">
        {children}
      </main>
    </div>
  )
}
