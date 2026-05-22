import type { H3Event } from 'h3'
import { finishFrontendRequest, startFrontendRequest } from '../utils/metrics'

const clientIpFor = (event: H3Event) => {
  const forwardedFor = event.node.req.headers['x-forwarded-for']
  const realIp = event.node.req.headers['x-real-ip']
  const cfIp = event.node.req.headers['cf-connecting-ip']
  const raw = (Array.isArray(cfIp) ? cfIp[0] : cfIp)
    || (Array.isArray(realIp) ? realIp[0] : realIp)
    || (Array.isArray(forwardedFor) ? forwardedFor[0] : forwardedFor)
    || event.node.req.socket.remoteAddress
    || 'unknown'

  return raw.split(',')[0].trim() || 'unknown'
}

export default defineEventHandler((event) => {
  const path = event.path || event.node.req.url || '/'
  if (path.startsWith('/metrics')) {
    return
  }

  const startedAt = process.hrtime.bigint()
  startFrontendRequest()

  event.node.res.once('finish', () => {
    const elapsedSeconds = Number(process.hrtime.bigint() - startedAt) / 1e9
    const method = event.node.req.method || 'GET'
    const status = String(event.node.res.statusCode || 200)
    const route = path.split('?')[0] || '/'

    finishFrontendRequest({ method, path: route, status, clientIp: clientIpFor(event) }, elapsedSeconds)
  })
})
