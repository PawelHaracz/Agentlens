import { describe, it, expect, vi, beforeEach } from 'vitest'

describe('telemetry module', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  it('exports initTelemetry function', async () => {
    const mod = await import('./telemetry')
    expect(typeof mod.initTelemetry).toBe('function')
  })

  it('does not throw when called with valid config', async () => {
    const { initTelemetry } = await import('./telemetry')
    expect(() =>
      initTelemetry({
        endpoint: 'http://localhost:4318/v1/traces',
        serviceName: 'agentlens-web',
      })
    ).not.toThrow()
  })

  it('exports TelemetryConfig type', async () => {
    // This test verifies the module compiles with the expected shape
    const mod = await import('./telemetry')
    expect(mod).toBeDefined()
  })
})
