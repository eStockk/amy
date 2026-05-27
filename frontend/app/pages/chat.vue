<template>
  <div class="chat-page">
    <section class="panel chat-shell">
      <header class="chat-head">
        <div>
          <p class="eyebrow">Общий чат</p>
          <h1>Игроки Amy</h1>
          <p class="muted">Сообщения синхронизируются с Discord-каналом.</p>
        </div>
        <button class="ghost" type="button" :disabled="loading" @click="loadMessages">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M20 11a8 8 0 0 0-14.4-4.8L4 8" />
            <path d="M4 4v4h4" />
            <path d="M4 13a8 8 0 0 0 14.4 4.8L20 16" />
            <path d="M20 20v-4h-4" />
          </svg>
          Обновить
        </button>
      </header>

      <div v-if="!authenticated" class="empty">
        Войдите через Discord, чтобы открыть чат игроков.
      </div>
      <div v-else-if="!hasChatAccess" class="empty">
        Чат доступен игрокам с принятой RP-заявкой.
      </div>
      <template v-else>
        <div ref="messageList" class="messages">
          <article v-for="item in messages" :key="item.id" class="message">
            <img class="avatar" :src="item.avatarUrl || logo" alt="" />
            <div class="bubble">
              <div class="meta">
                <strong>{{ item.author }}</strong>
                <time>{{ formatDate(item.createdAt) }}</time>
              </div>
              <p v-if="item.message">{{ item.message }}</p>
              <img v-if="item.imageUrl" class="attached" :src="item.imageUrl" alt="" loading="lazy" referrerpolicy="no-referrer" />
              <img v-if="item.gifUrl" class="attached" :src="item.gifUrl" alt="" loading="lazy" referrerpolicy="no-referrer" />
            </div>
          </article>
        </div>

        <form class="composer" @submit.prevent="sendMessage">
          <textarea v-model.trim="messageText" rows="3" maxlength="1000" placeholder="Сообщение в чат..."></textarea>
          <input v-model.trim="gifUrl" type="url" placeholder="Ссылка на Tenor GIF" />
          <button class="primary" type="submit" :disabled="sending || (!messageText && !gifUrl)">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4 12 20 4l-4 16-4-6-8-2Z" />
              <path d="m12 14 4-5" />
            </svg>
            Отправить
          </button>
          <p v-if="errorText" class="error">{{ errorText }}</p>
        </form>
      </template>
    </section>
  </div>
</template>

<script setup lang="ts">
import logo from '~/assets/amy-logo.png'
import { useAuth } from '~/composables/useAuth'

type ChatMessage = {
  id: string
  author: string
  avatarUrl?: string
  message: string
  imageUrl?: string
  gifUrl?: string
  createdAt: string
}

const config = useRuntimeConfig()
const { authenticated, user, refresh } = useAuth()

const messages = ref<ChatMessage[]>([])
const messageText = ref('')
const gifUrl = ref('')
const errorText = ref('')
const loading = ref(false)
const sending = ref(false)
const messageList = ref<HTMLElement | null>(null)
let pollTimer: ReturnType<typeof setInterval> | undefined

const hasChatAccess = computed(() => {
  const status = user.value?.rpApplication?.status
  return status === 'accepted' || status === 'approved'
})

const loadMessages = async () => {
  if (!authenticated.value || !hasChatAccess.value) return
  loading.value = true
  try {
    const response = await $fetch<{ messages: ChatMessage[] }>(`${config.public.apiBase}/community/chat`, {
      credentials: 'include'
    })
    messages.value = response.messages
    await nextTick()
    messageList.value?.scrollTo({ top: messageList.value.scrollHeight })
  } catch (error: unknown) {
    errorText.value = (error as { data?: { error?: string } })?.data?.error || 'Не удалось загрузить чат.'
  } finally {
    loading.value = false
  }
}

const sendMessage = async () => {
  if (!messageText.value && !gifUrl.value) return
  errorText.value = ''
  sending.value = true
  try {
    const response = await $fetch<{ messages: ChatMessage[] }>(`${config.public.apiBase}/community/chat`, {
      method: 'POST',
      credentials: 'include',
      body: {
        message: messageText.value,
        gifUrl: gifUrl.value
      }
    })
    messageText.value = ''
    gifUrl.value = ''
    messages.value = response.messages
    await nextTick()
    messageList.value?.scrollTo({ top: messageList.value.scrollHeight, behavior: 'smooth' })
  } catch (error: unknown) {
    errorText.value = (error as { data?: { error?: string } })?.data?.error || 'Не удалось отправить сообщение.'
  } finally {
    sending.value = false
  }
}

const formatDate = (raw: string) => new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  hour: '2-digit',
  minute: '2-digit'
}).format(new Date(raw))

onMounted(async () => {
  await refresh()
  await loadMessages()
  pollTimer = setInterval(loadMessages, 10000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.chat-page {
  min-height: calc(100vh - 190px);
  display: grid;
}

.panel {
  border: 1px solid var(--stroke);
  border-radius: 8px;
  background: var(--panel);
}

.chat-shell {
  display: grid;
  gap: 14px;
  padding: 18px;
}

.chat-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}

.eyebrow,
.muted,
p {
  margin: 0;
}

.eyebrow {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0;
  color: var(--muted);
}

h1 {
  margin: 0;
  font-family: 'Neue Machine', 'Montserrat', sans-serif;
}

.muted,
.empty {
  color: var(--muted);
}

.empty {
  min-height: 260px;
  display: grid;
  place-items: center;
  text-align: center;
}

.messages {
  display: grid;
  gap: 10px;
  min-height: 360px;
  max-height: 58vh;
  overflow: auto;
  padding-right: 4px;
}

.message {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  gap: 10px;
}

.avatar {
  width: 38px;
  height: 38px;
  border-radius: 8px;
  object-fit: cover;
}

.bubble {
  display: grid;
  gap: 7px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.05);
  padding: 10px 12px;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  color: var(--muted);
  font-size: 12px;
}

.meta strong {
  color: var(--text);
}

.attached {
  max-width: min(420px, 100%);
  max-height: 320px;
  object-fit: contain;
  border-radius: 8px;
}

.composer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(180px, 280px) auto;
  gap: 10px;
  align-items: end;
}

textarea,
input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text);
  padding: 10px 12px;
  font: inherit;
}

textarea {
  resize: none;
}

.ghost,
.primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: 999px;
  padding: 10px 14px;
  border: 0;
  cursor: pointer;
  font-weight: 700;
}

.ghost {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
}

.primary {
  color: #0b0b0f;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.ghost svg,
.primary svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.primary:disabled,
.ghost:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error {
  grid-column: 1 / -1;
  color: #ff9f9f;
}

@media (max-width: 820px) {
  .chat-head,
  .composer {
    grid-template-columns: 1fr;
    display: grid;
  }

  .messages {
    max-height: none;
  }
}
</style>
