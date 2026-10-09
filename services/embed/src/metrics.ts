import promClient from 'prom-client'

promClient.collectDefaultMetrics({ prefix: 'embed_' })

export const requestsTotal = new promClient.Counter({
  name: 'embed_requests_total',
  help: 'Total embed requests',
  labelNames: ['status'] as const,
})

export const duration = new promClient.Histogram({
  name: 'embed_duration_seconds',
  help: 'Embed request duration in seconds',
  buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5],
})

export const modelState = new promClient.Gauge({
  name: 'embed_model_state',
  help: 'Embedding model state: 0=pending, 1=ready, 2=error',
})

export function setModelState(state: 'pending' | 'ready' | 'error') {
  const val = state === 'ready' ? 1 : state === 'error' ? 2 : 0
  modelState.set(val)
}

export function metricsHandler(req: any, res: any) {
  res.set('Content-Type', promClient.register.contentType)
  res.end(promClient.register.metrics())
}
