self.addEventListener('push', (event) => {
  let payload = {
    title: 'Ответ поддержки',
    body: 'В тикете появился новый ответ.',
    url: '/support'
  }

  if (event.data) {
    try {
      payload = { ...payload, ...event.data.json() }
    } catch {
      payload.body = event.data.text()
    }
  }

  event.waitUntil(self.registration.showNotification(payload.title, {
    body: payload.body,
    tag: payload.ticketId ? `support-ticket-${payload.ticketId}` : 'support-ticket',
    data: { url: payload.url || '/support' }
  }))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const targetUrl = new URL(event.notification.data?.url || '/support', self.location.origin).href

  event.waitUntil((async () => {
    const windowClients = await clients.matchAll({ type: 'window', includeUncontrolled: true })
    for (const client of windowClients) {
      if ('focus' in client) {
        await client.navigate(targetUrl)
        return client.focus()
      }
    }
    return clients.openWindow(targetUrl)
  })())
})
