<template>
  <div class="support">
    <section class="panel ticket-form">
      <div>
        <h1>Поддержка</h1>
        <p>Оставьте тикет, и ответ появится в чате справа.</p>
      </div>

      <form @submit.prevent="submit">
        <label>
          Discord ник *
          <span class="field-control">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M8 9.5h8M8 14.5h5.5M7.5 4.5h9A3.5 3.5 0 0 1 20 8v6a3.5 3.5 0 0 1-3.5 3.5H13L8.7 20v-2.5H7.5A3.5 3.5 0 0 1 4 14V8a3.5 3.5 0 0 1 3.5-3.5Z" />
            </svg>
            <input v-model="discordNick" type="text" placeholder="username или display name" required />
          </span>
        </label>
        <label>
          Тема *
          <span class="field-control">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M5 6.5h14M5 12h14M5 17.5h8" />
            </svg>
            <input v-model="subject" type="text" required />
          </span>
        </label>
        <label>
          Категория
          <span class="field-control select-control">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4.5 7.5h15M7.5 12h9M10 16.5h4" />
            </svg>
            <select v-model="category">
              <option>Общие вопросы</option>
              <option>Техническая проблема</option>
              <option>Оплата</option>
              <option>Другое</option>
            </select>
            <svg class="chevron" viewBox="0 0 24 24" aria-hidden="true">
              <path d="m8 10 4 4 4-4" />
            </svg>
          </span>
        </label>
        <label>
          Сообщение *
          <span class="field-control textarea-control">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M6 7h12M6 12h12M6 17h7" />
            </svg>
            <textarea v-model="message" rows="5" required></textarea>
          </span>
        </label>
        <button type="submit" class="primary" :disabled="sending">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M4 12 20 4l-4 16-4-6-8-2Z" />
            <path d="m12 14 4-5" />
          </svg>
          {{ sending ? 'Отправляем...' : 'Отправить тикет' }}
        </button>
        <p v-if="status" class="status" :class="{ error: statusError }">{{ status }}</p>
      </form>
    </section>

    <section class="panel ticket-chat">
      <div class="chat-head">
        <div>
          <p class="eyebrow">Чат тикета</p>
          <h2>{{ activeTicket?.subject || 'Выберите тикет' }}</h2>
          <p v-if="activeTicket" class="muted">
            #{{ activeTicket.id }} · {{ statusLabel(activeTicket.status) }} · {{ formatDate(activeTicket.createdAt) }}
          </p>
        </div>
        <button class="ghost" type="button" @click="loadTickets">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M20 11a8 8 0 0 0-14.4-4.8L4 8" />
            <path d="M4 4v4h4" />
            <path d="M4 13a8 8 0 0 0 14.4 4.8L20 16" />
            <path d="M20 20v-4h-4" />
          </svg>
          Обновить
        </button>
      </div>

      <div class="notify-panel">
        <label class="toggle">
          <input v-model="notifyEnabled" type="checkbox" @change="syncNotificationSettings" />
          <span>Уведомления сайта</span>
        </label>
        <label class="toggle">
          <input v-model="soundEnabled" type="checkbox" :disabled="!notifyEnabled" @change="saveNotificationPrefs" />
          <span>Звук</span>
        </label>
        <p v-if="notificationStatus" class="notify-status">{{ notificationStatus }}</p>
      </div>

      <div v-if="tickets.length" class="ticket-tabs">
        <button
          v-for="ticket in tickets"
          :key="ticket.id"
          type="button"
          :class="{ active: ticket.id === activeTicketId }"
          @click="selectTicket(ticket.id)"
        >
          <span>#{{ ticket.id }} {{ ticket.subject }}</span>
          <strong v-if="ticket.unreadAdminCount">{{ ticket.unreadAdminCount }}</strong>
        </button>
      </div>

      <div v-if="!authenticated" class="empty">
        Войдите через Discord, чтобы видеть свои тикеты и получать ответы здесь.
      </div>
      <div v-else-if="!activeTicket" class="empty">
        После отправки тикета здесь откроется переписка с техподдержкой.
      </div>
      <template v-else>
        <div ref="messageList" class="messages">
          <article
            v-for="item in messages"
            :key="item.id"
            class="message"
            :class="{ mine: item.authorType === 'user' }"
          >
            <div class="message-meta">
              <strong>{{ item.authorName || authorLabel(item.authorType) }}</strong>
              <span v-if="item.authorDiscordStatus" class="discord-status">{{ discordStatusLabel(item.authorDiscordStatus) }}</span>
              <time>{{ formatDate(item.createdAt) }}</time>
            </div>
            <p>{{ item.message }}</p>
            <div v-if="item.attachments?.length" class="attachments">
              <a
                v-for="attachment in item.attachments"
                :key="attachment.id"
                :href="`${config.public.apiBase}${attachment.url}`"
                target="_blank"
                rel="noreferrer"
              >
                <img v-if="attachment.mimeType.startsWith('image/')" :src="`${config.public.apiBase}${attachment.url}`" :alt="attachment.fileName" />
                <span>{{ attachment.fileName }}</span>
              </a>
            </div>
          </article>
        </div>

        <form class="reply" @submit.prevent="sendReply">
          <textarea v-model="replyText" rows="3" placeholder="Ответить в тикет..."></textarea>
          <label class="file-picker">
            <input ref="imageInput" type="file" accept="image/*" @change="selectReplyImage" />
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M5 19h14a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2Z" />
              <path d="m8 14 2.2-2.2a1.2 1.2 0 0 1 1.7 0L15 15" />
              <path d="m14 14 1.2-1.2a1.2 1.2 0 0 1 1.7 0L19 15" />
              <path d="M8.5 9.5h.01" />
            </svg>
            <span>{{ replyImageName || 'Изображение до 10 MB' }}</span>
          </label>
          <button v-if="replyImage" type="button" class="remove-file" aria-label="Убрать изображение" @click="clearReplyImage">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M18 6 6 18" />
              <path d="m6 6 12 12" />
            </svg>
          </button>
          <button type="submit" class="primary" :disabled="replySending || (!replyText.trim() && !replyImage)">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4 12 20 4l-4 16-4-6-8-2Z" />
              <path d="m12 14 4-5" />
            </svg>
            {{ replySending ? 'Отправляем...' : 'Ответить' }}
          </button>
          <p v-if="replyError" class="reply-error">{{ replyError }}</p>
        </form>
      </template>
    </section>
  </div>
</template>

<script setup lang="ts">
type Ticket = {
  id: number
  name: string
  discordNick: string
  subject: string
  category: string
  message: string
  status: string
  unreadAdminCount: number
  createdAt: string
  resolvedAt?: string
}

type TicketMessage = {
  id: number
  ticketId: number
  authorType: 'user' | 'admin'
  authorName: string
  authorDiscordStatus?: string
  message: string
  attachments?: TicketAttachment[]
  readByUser: boolean
  createdAt: string
}

type TicketAttachment = {
  id: number
  fileName: string
  mimeType: string
  sizeBytes: number
  url: string
}

const config = useRuntimeConfig()
const route = useRoute()
const router = useRouter()
const { authenticated, user, refresh } = useAuth()

const discordNick = ref('')
const subject = ref('')
const category = ref('Общие вопросы')
const message = ref('')
const status = ref('')
const statusError = ref(false)
const sending = ref(false)

const tickets = ref<Ticket[]>([])
const activeTicketId = ref<number | null>(null)
const messages = ref<TicketMessage[]>([])
const replyText = ref('')
const replyImage = ref<File | null>(null)
const replyImageName = ref('')
const replyError = ref('')
const replySending = ref(false)
const imageInput = ref<HTMLInputElement | null>(null)
const messageList = ref<HTMLElement | null>(null)

const notifyEnabled = ref(false)
const soundEnabled = ref(false)
const notificationStatus = ref('')
const lastSeenMessageId = ref(0)
let pollTimer: ReturnType<typeof setInterval> | undefined

const activeTicket = computed(() => tickets.value.find((ticket) => ticket.id === activeTicketId.value) || null)

const submit = async () => {
  status.value = ''
  statusError.value = false
  sending.value = true
  try {
    const response = await $fetch<{ ticket: Ticket }>(`${config.public.apiBase}/support/tickets`, {
      method: 'POST',
      credentials: 'include',
      body: {
        discordNick: discordNick.value,
        subject: subject.value,
        category: category.value,
        message: message.value
      }
    })
    status.value = 'Тикет отправлен. Чат открыт справа.'
    subject.value = ''
    message.value = ''
    await loadTickets(response.ticket.id)
  } catch (error: unknown) {
    statusError.value = true
    status.value = (error as { data?: { error?: string } })?.data?.error || 'Не удалось отправить тикет. Попробуйте позже.'
  } finally {
    sending.value = false
  }
}

const loadTickets = async (preferredId?: number) => {
  if (!authenticated.value) return
  const response = await $fetch<{ tickets: Ticket[] }>(`${config.public.apiBase}/support/tickets`, {
    credentials: 'include'
  })
  tickets.value = response.tickets
  const routeTicketId = Number(route.query.ticket || 0)
  const nextId = preferredId || routeTicketId || activeTicketId.value || response.tickets[0]?.id
  if (nextId) await selectTicket(nextId, false)
}

const selectTicket = async (ticketId: number, updateUrl = true) => {
  activeTicketId.value = ticketId
  if (updateUrl) {
    await router.replace({ query: { ...route.query, ticket: String(ticketId) } })
  }
  await loadMessages()
}

const loadMessages = async () => {
  if (!activeTicketId.value) return
  const previousLastId = lastSeenMessageId.value
  const response = await $fetch<{ ticket: Ticket; messages: TicketMessage[] }>(
    `${config.public.apiBase}/support/tickets/${activeTicketId.value}/messages`,
    { credentials: 'include' }
  )
  messages.value = response.messages
  const index = tickets.value.findIndex((ticket) => ticket.id === response.ticket.id)
  if (index >= 0) tickets.value[index] = response.ticket
  const lastMessage = response.messages.at(-1)
  if (lastMessage && lastMessage.id > previousLastId && lastMessage.authorType === 'admin') {
    notifyInPage(lastMessage)
  }
  if (lastMessage) lastSeenMessageId.value = lastMessage.id
  await nextTick()
  messageList.value?.scrollTo({ top: messageList.value.scrollHeight })
}

const sendReply = async () => {
  if (!activeTicketId.value || (!replyText.value.trim() && !replyImage.value)) return
  replyError.value = ''
  replySending.value = true
  try {
    const body = new FormData()
    body.append('message', replyText.value)
    if (replyImage.value) body.append('image', replyImage.value, replyImage.value.name)
    const response = await $fetch<{ messages: TicketMessage[] }>(
      `${config.public.apiBase}/support/tickets/${activeTicketId.value}/messages`,
      {
        method: 'POST',
        credentials: 'include',
        body
      }
    )
    replyText.value = ''
    clearReplyImage()
    messages.value = response.messages
    await nextTick()
    messageList.value?.scrollTo({ top: messageList.value.scrollHeight, behavior: 'smooth' })
  } catch (error: unknown) {
    replyError.value = (error as { data?: { error?: string }; message?: string })?.data?.error || 'Не удалось отправить сообщение или изображение.'
  } finally {
    replySending.value = false
  }
}

const selectReplyImage = (event: Event) => {
  const file = (event.target as HTMLInputElement).files?.[0]
  replyError.value = ''
  if (!file) {
    clearReplyImage()
    return
  }
  if (!file.type.startsWith('image/')) {
    replyError.value = 'Можно прикреплять только изображения.'
    clearReplyImage()
    return
  }
  if (file.size > 10 * 1024 * 1024) {
    replyError.value = 'Изображение должно быть не больше 10 MB.'
    clearReplyImage()
    return
  }
  replyImage.value = file
  replyImageName.value = file.name
}

const clearReplyImage = () => {
  replyImage.value = null
  replyImageName.value = ''
  if (imageInput.value) imageInput.value.value = ''
}

const syncNotificationSettings = async () => {
  notificationStatus.value = ''
  if (!notifyEnabled.value) {
    await disablePushNotifications()
    saveNotificationPrefs()
    notificationStatus.value = 'Уведомления выключены.'
    return
  }
  try {
    await enablePushNotifications()
    notificationStatus.value = 'Уведомления включены.'
  } catch (error: unknown) {
    notifyEnabled.value = false
    notificationStatus.value = (error as Error).message || 'Не удалось включить уведомления.'
  } finally {
    saveNotificationPrefs()
  }
}

const enablePushNotifications = async () => {
  if (!import.meta.client || !('Notification' in window) || !('serviceWorker' in navigator) || !('PushManager' in window)) {
    throw new Error('Браузер не поддерживает push-уведомления.')
  }
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') throw new Error('Разрешение на уведомления не выдано.')

  const response = await $fetch<{ configured: boolean; publicKey: string }>(`${config.public.apiBase}/support/notifications`, {
    credentials: 'include'
  })
  if (!response.configured || !response.publicKey) {
    throw new Error('Push-уведомления не настроены на сервере.')
  }

  const registration = await navigator.serviceWorker.register('/support-notifications-sw.js')
  const existing = await registration.pushManager.getSubscription()
  const subscription = existing || await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(response.publicKey)
  })
  await $fetch(`${config.public.apiBase}/support/notifications`, {
    method: 'POST',
    credentials: 'include',
    body: subscription.toJSON()
  })
}

const disablePushNotifications = async () => {
  if (!import.meta.client) return
  let endpoint = ''
  if ('serviceWorker' in navigator) {
    const registration = await navigator.serviceWorker.getRegistration('/support-notifications-sw.js')
    const subscription = await registration?.pushManager.getSubscription()
    endpoint = subscription?.endpoint || ''
    await subscription?.unsubscribe()
  }
  await $fetch(`${config.public.apiBase}/support/notifications${endpoint ? `?endpoint=${encodeURIComponent(endpoint)}` : ''}`, {
    method: 'DELETE',
    credentials: 'include'
  }).catch(() => undefined)
}

const notifyInPage = (item: TicketMessage) => {
  if (!notifyEnabled.value) return
  if (soundEnabled.value) playNotificationSound()
  if (import.meta.client && document.visibilityState !== 'visible' && Notification.permission === 'granted') {
    const notification = new Notification('Ответ поддержки', {
      body: `${item.authorName}: ${item.message}`,
      tag: `support-ticket-${item.ticketId}`
    })
    notification.onclick = () => {
      window.focus()
      router.push(`/support?ticket=${item.ticketId}`)
    }
  }
}

const playNotificationSound = () => {
  const audioContext = new AudioContext()
  const oscillator = audioContext.createOscillator()
  const gain = audioContext.createGain()
  oscillator.frequency.value = 880
  gain.gain.value = 0.04
  oscillator.connect(gain)
  gain.connect(audioContext.destination)
  oscillator.start()
  oscillator.stop(audioContext.currentTime + 0.14)
}

const saveNotificationPrefs = () => {
  if (!import.meta.client) return
  localStorage.setItem('supportNotifyEnabled', notifyEnabled.value ? '1' : '0')
  localStorage.setItem('supportSoundEnabled', soundEnabled.value ? '1' : '0')
}

const restoreNotificationPrefs = () => {
  if (!import.meta.client) return
  notifyEnabled.value = localStorage.getItem('supportNotifyEnabled') === '1'
  soundEnabled.value = localStorage.getItem('supportSoundEnabled') === '1'
}

const urlBase64ToUint8Array = (base64: string) => {
  const padding = '='.repeat((4 - base64.length % 4) % 4)
  const raw = atob((base64 + padding).replace(/-/g, '+').replace(/_/g, '/'))
  return Uint8Array.from([...raw].map((char) => char.charCodeAt(0)))
}

const formatDate = (raw: string) => new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  hour: '2-digit',
  minute: '2-digit'
}).format(new Date(raw))

const statusLabel = (raw: string) => raw === 'resolved' ? 'решён' : 'открыт'
const authorLabel = (raw: string) => raw === 'admin' ? 'Техподдержка' : 'Вы'
const discordStatusLabel = (raw: string) => ({
  online: 'online',
  idle: 'idle',
  dnd: 'dnd',
  offline: 'offline',
  unknown: 'unknown'
}[raw] || raw)

onMounted(async () => {
  restoreNotificationPrefs()
  await refresh()
  if (user.value) {
    discordNick.value = user.value.username || user.value.displayName || ''
  }
  await loadTickets()
  pollTimer = setInterval(async () => {
    await loadTickets()
  }, 15000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.support {
  display: grid;
  grid-template-columns: minmax(320px, 440px) minmax(0, 1fr);
  gap: 20px;
  align-items: start;
  min-height: calc(100vh - 140px);
}

.panel {
  border: 1px solid var(--stroke);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.045);
  padding: 20px;
  height: fit-content;
}

.ticket-form,
.ticket-chat,
form {
  display: grid;
  gap: 14px;
  align-content: start;
}

h1,
h2 {
  margin: 0;
  font-family: 'Neue Machine', 'Montserrat', sans-serif;
}

p {
  margin: 0;
  color: var(--muted);
}

label {
  display: grid;
  gap: 6px;
  color: var(--muted);
  font-size: 13px;
}

input,
select,
textarea {
  width: 100%;
  box-sizing: border-box;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 10px 12px;
  color: var(--text);
}

.field-control {
  display: grid;
  grid-template-columns: 20px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 42px;
  box-sizing: border-box;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 0 12px;
  color: var(--text);
}

.field-control svg,
.primary svg,
.ghost svg,
.file-picker svg,
.remove-file svg {
  width: 18px;
  height: 18px;
  flex: 0 0 auto;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.field-control > input,
.field-control > select,
.field-control > textarea {
  min-width: 0;
  background: transparent;
  border: 0;
  border-radius: 0;
  padding: 10px 0;
  outline: 0;
}

.field-control > input:focus,
.field-control > select:focus,
.field-control > textarea:focus {
  outline: 0;
}

.textarea-control {
  align-items: start;
  padding-top: 10px;
  padding-bottom: 10px;
}

.textarea-control svg {
  margin-top: 2px;
}

.textarea-control > textarea {
  padding: 0;
}

.select-control {
  grid-template-columns: 20px minmax(0, 1fr) 18px;
}

.select-control .chevron {
  pointer-events: none;
}

textarea {
  resize: none;
}

select {
  appearance: none;
}

select option {
  background: #1e202a;
  color: #f7f4e8;
}

.primary,
.ghost,
.ticket-tabs button {
  border: 0;
  cursor: pointer;
  font-weight: 700;
}

.primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  border-radius: 999px;
  padding: 10px 16px;
  color: #0b0b0f;
}

.primary:disabled {
  cursor: not-allowed;
  opacity: 0.62;
}

.status.error {
  color: #ff9090;
}

.chat-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}

.eyebrow,
.notify-status {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0;
}

.ghost {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border-radius: 999px;
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
}

.notify-panel {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
}

.toggle {
  display: inline-flex;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 8px;
}

.toggle input {
  width: auto;
}

.ticket-tabs {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
}

.ticket-tabs button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 160px;
  max-width: 240px;
  border-radius: 8px;
  padding: 9px 10px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
  text-align: left;
}

.ticket-tabs button span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ticket-tabs button.active {
  outline: 1px solid var(--accent);
}

.ticket-tabs strong {
  display: grid;
  place-items: center;
  min-width: 20px;
  height: 20px;
  border-radius: 999px;
  background: var(--accent);
  color: #0b0b0f;
  font-size: 12px;
}

.empty {
  min-height: 260px;
  display: grid;
  place-items: center;
  color: var(--muted);
  text-align: center;
}

.messages {
  display: grid;
  align-content: start;
  gap: 10px;
  min-height: 360px;
  max-height: 52vh;
  overflow: auto;
  padding-right: 4px;
}

.message {
  max-width: min(680px, 84%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.07);
}

.message.mine {
  justify-self: end;
  background: rgba(247, 201, 72, 0.12);
}

.message-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 6px;
  color: var(--muted);
  font-size: 12px;
}

.message-meta strong {
  color: var(--text);
}

.discord-status {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  padding: 1px 7px;
}

.reply {
  grid-template-columns: minmax(0, 1fr) minmax(180px, 240px) auto auto;
  align-items: end;
}

.file-picker {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 42px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 10px 12px;
  color: var(--text);
  background: rgba(255, 255, 255, 0.06);
  cursor: pointer;
}

.file-picker input {
  display: none;
}

.file-picker span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.remove-file {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
  cursor: pointer;
}

.reply-error {
  grid-column: 1 / -1;
  color: #ff9090;
}

.attachments {
  display: grid;
  gap: 8px;
  margin-top: 8px;
}

.attachments a {
  color: var(--text);
  text-decoration: none;
}

.attachments img {
  max-width: min(320px, 100%);
  max-height: 260px;
  object-fit: contain;
  border-radius: 8px;
}

@media (max-width: 1024px) {
  .support {
    grid-template-columns: 1fr;
  }

  .messages {
    max-height: none;
  }
}

@media (max-width: 620px) {
  .reply {
    grid-template-columns: 1fr;
  }

  .chat-head {
    display: grid;
  }
}
</style>
