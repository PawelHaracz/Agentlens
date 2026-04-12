import { WebTracerProvider } from '@opentelemetry/sdk-trace-web'
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http'
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch'
import { resourceFromAttributes } from '@opentelemetry/resources'
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-web'
import { registerInstrumentations } from '@opentelemetry/instrumentation'

export interface TelemetryConfig {
  endpoint: string
  serviceName: string
}

export function initTelemetry(config: TelemetryConfig): void {
  const resource = resourceFromAttributes({
    'service.name': config.serviceName,
  })

  const exporter = new OTLPTraceExporter({
    url: config.endpoint,
  })

  const provider = new WebTracerProvider({
    resource,
    spanProcessors: [new BatchSpanProcessor(exporter)],
  })

  provider.register()

  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/\/api\/.*/],
      }),
    ],
  })
}
