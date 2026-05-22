type Labels = {
  method: string
  path: string
  status: string
}

const buckets = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
const requests = new Map<string, number>()
const durationBuckets = new Map<string, number[]>()
const durationSums = new Map<string, number>()
const durationCounts = new Map<string, number>()
let inFlight = 0

const keyFor = (labels: Labels) => `${labels.method}\u0000${labels.path}\u0000${labels.status}`

const labelsFor = (key: string) => {
  const [method, path, status] = key.split('\u0000')
  return `method="${escapeLabel(method)}",path="${escapeLabel(path)}",status="${escapeLabel(status)}"`
}

const escapeLabel = (value = '') => value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n')

export const startFrontendRequest = () => {
  inFlight += 1
}

export const finishFrontendRequest = (labels: Labels, seconds: number) => {
  inFlight = Math.max(0, inFlight - 1)

  const key = keyFor(labels)
  requests.set(key, (requests.get(key) || 0) + 1)
  durationSums.set(key, (durationSums.get(key) || 0) + seconds)
  durationCounts.set(key, (durationCounts.get(key) || 0) + 1)

  const values = durationBuckets.get(key) || buckets.map(() => 0)
  for (let index = 0; index < buckets.length; index += 1) {
    if (seconds <= buckets[index]) {
      values[index] += 1
    }
  }
  durationBuckets.set(key, values)
}

export const renderFrontendMetrics = () => {
  const lines: string[] = [
    '# HELP amy_frontend_http_requests_total Total frontend HTTP requests.',
    '# TYPE amy_frontend_http_requests_total counter'
  ]

  for (const [key, value] of requests) {
    lines.push(`amy_frontend_http_requests_total{${labelsFor(key)}} ${value}`)
  }

  lines.push(
    '# HELP amy_frontend_http_request_duration_seconds Frontend HTTP request duration.',
    '# TYPE amy_frontend_http_request_duration_seconds histogram'
  )

  for (const [key, values] of durationBuckets) {
    const baseLabels = labelsFor(key)
    let cumulative = 0
    for (let index = 0; index < buckets.length; index += 1) {
      cumulative += values[index]
      lines.push(`amy_frontend_http_request_duration_seconds_bucket{${baseLabels},le="${buckets[index]}"} ${cumulative}`)
    }
    lines.push(`amy_frontend_http_request_duration_seconds_bucket{${baseLabels},le="+Inf"} ${durationCounts.get(key) || 0}`)
    lines.push(`amy_frontend_http_request_duration_seconds_sum{${baseLabels}} ${durationSums.get(key) || 0}`)
    lines.push(`amy_frontend_http_request_duration_seconds_count{${baseLabels}} ${durationCounts.get(key) || 0}`)
  }

  const memory = process.memoryUsage()
  lines.push(
    '# HELP amy_frontend_http_requests_in_flight Frontend HTTP requests currently in flight.',
    '# TYPE amy_frontend_http_requests_in_flight gauge',
    `amy_frontend_http_requests_in_flight ${inFlight}`,
    '# HELP amy_frontend_nodejs_heap_size_used_bytes Node.js heap used by the frontend process.',
    '# TYPE amy_frontend_nodejs_heap_size_used_bytes gauge',
    `amy_frontend_nodejs_heap_size_used_bytes ${memory.heapUsed}`,
    '# HELP amy_frontend_nodejs_heap_size_total_bytes Node.js heap total for the frontend process.',
    '# TYPE amy_frontend_nodejs_heap_size_total_bytes gauge',
    `amy_frontend_nodejs_heap_size_total_bytes ${memory.heapTotal}`,
    '# HELP amy_frontend_process_resident_memory_bytes Resident memory size for the frontend process.',
    '# TYPE amy_frontend_process_resident_memory_bytes gauge',
    `amy_frontend_process_resident_memory_bytes ${memory.rss}`
  )

  return `${lines.join('\n')}\n`
}
