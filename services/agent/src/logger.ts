/**
 * Structured logger for the agent service.
 * Production: JSON output (aligned with backend logx format).
 * Development: pretty-printed for readability.
 */
import pino from 'pino'

const isDev = process.env.APP_ENV !== 'production'

const logger = pino({
  name: 'starry-agent',
  level: process.env.LOG_LEVEL || (isDev ? 'debug' : 'info'),
  transport: isDev
    ? {
        target: 'pino-pretty',
        options: { colorize: true, translateTime: 'SYS:standard' },
      }
    : undefined,
})

export default logger
