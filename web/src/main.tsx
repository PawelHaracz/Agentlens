import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

async function boot(): Promise<void> {
  // Initialize OTel if enabled — best-effort, never blocks app startup
  try {
    const resp = await fetch('/api/v1/telemetry/config')
    if (resp.ok) {
      const cfg = await resp.json()
      if (cfg.enabled && cfg.endpoint) {
        const { initTelemetry } = await import('./telemetry')
        initTelemetry(cfg)
      }
    }
  } catch {
    // Telemetry init failure must not break the app
  }

  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </React.StrictMode>,
  )
}

void boot()
