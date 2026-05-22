import { renderFrontendMetrics } from '../utils/metrics'

export default defineEventHandler((event) => {
  setHeader(event, 'Content-Type', 'text/plain; version=0.0.4; charset=utf-8')
  return renderFrontendMetrics()
})
