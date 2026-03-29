import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-14 flex items-center gap-3">
          <Link to="/" className="flex items-center gap-2 text-gray-900 hover:text-indigo-600 transition-colors">
            <span className="text-xl font-bold tracking-tight">AgentLens</span>
          </Link>
          <span className="text-gray-400 text-sm ml-2">AI Agent Catalog</span>
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        {children}
      </main>
    </div>
  )
}
