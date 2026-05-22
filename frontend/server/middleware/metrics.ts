import { finishFrontendRequest, startFrontendRequest } from '../utils/metrics'

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

    finishFrontendRequest({ method, path: route, status }, elapsedSeconds)
  })
})
