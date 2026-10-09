import promClient from 'prom-client'

promClient.collectDefaultMetrics({ prefix: 'agent_' })

export const conversationsTotal = new promClient.Counter({
  name: 'agent_conversations_total',
  help: 'Total conversations created',
})

export const llmCallsTotal = new promClient.Counter({
  name: 'agent_llm_calls_total',
  help: 'Total LLM API calls',
  labelNames: ['model', 'status'] as const,
})

export const llmDuration = new promClient.Histogram({
  name: 'agent_llm_duration_seconds',
  help: 'LLM call duration in seconds',
  labelNames: ['model'] as const,
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30, 60],
})

export const toolCallsTotal = new promClient.Counter({
  name: 'agent_tool_calls_total',
  help: 'Total tool invocations',
  labelNames: ['tool', 'status'] as const,
})

export function metricsHandler(req: any, res: any) {
  res.set('Content-Type', promClient.register.contentType)
  res.end(promClient.register.metrics())
}
