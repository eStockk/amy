<template>
  <div class="profile-page">
    <section v-if="authenticated" class="crm-shell">
      <aside class="identity-column">
        <div class="identity-card panel">
          <div class="avatar-wrap">
            <img class="avatar" :src="avatarUrl" alt="avatar" />
            <span class="online-dot" aria-hidden="true"></span>
          </div>
          <p class="eyebrow">ПРОФИЛЬ DISCORD</p>
          <h1>{{ displayName }}</h1>
          <p class="muted">@{{ user?.username }}</p>
          <p v-if="user?.email" class="muted email">{{ user.email }}</p>
          <div class="identity-actions">
            <button class="ghost" type="button" @click="copyProfileLink">
              {{ copied ? 'Скопировано' : 'Копировать ссылку' }}
            </button>
            <NuxtLink class="ghost" :to="profilePath">Публичный профиль</NuxtLink>
          </div>
          <a class="profile-link" :href="profilePath" target="_blank" rel="noreferrer">{{ fullProfileUrl }}</a>
        </div>

        <div class="panel mini-card">
          <p class="mini-label">СТАТУС СВЯЗИ</p>
          <p class="mini-value">{{ user?.linkedMinecraft ? 'Аккаунт связан' : 'Нужна привязка' }}</p>
        </div>
      </aside>

      <div class="workspace-column">
        <header class="workspace-head panel">
          <div>
            <p class="eyebrow">ЛИЧНЫЙ КАБИНЕТ</p>
            <h2>Управление аккаунтом</h2>
          </div>
          <button class="ghost danger" type="button" @click="handleLogout">Выйти</button>
        </header>

        <section class="stats-grid">
          <article class="panel stat-card highlight">
            <p class="card-title">ИГРОВОЙ НИК</p>
            <strong>{{ user?.linkedMinecraft || 'Не привязан' }}</strong>
            <p class="muted">Этот ник используется для связи с сервером и профилем Discord.</p>
          </article>

          <article class="panel stat-card">
            <p class="card-title">ПРОЦЕСС</p>
            <ul>
              <li>Войти через Discord</li>
              <li>Указать ник Minecraft</li>
              <li>Использовать один профиль на сайте</li>
            </ul>
          </article>
        </section>

        <section class="panel form-panel">
          <div class="form-head">
            <h3>Привязать Minecraft аккаунт</h3>
            <span class="badge">3-16</span>
          </div>
          <p class="muted">Допустимы латиница, цифры и символ «_».</p>

          <form class="link-form" @submit.prevent="submitLink">
            <label for="nickname">Ник на сервере</label>
            <div class="row">
              <input
                id="nickname"
                v-model.trim="nickname"
                type="text"
                placeholder="Nickname (3-16, latin letters/digits/_ )"
                minlength="3"
                maxlength="16"
                required
              />
              <button class="primary" type="submit" :disabled="submitting">
                {{ submitting ? 'Сохраняем...' : 'Сохранить' }}
              </button>
            </div>
          </form>

          <p v-if="status" class="status" :class="{ error: statusType === 'error' }">{{ status }}</p>
        </section>
      </div>
    </section>

    <section v-else class="auth-empty panel">
      <h1>Профиль недоступен</h1>
      <p class="muted">Войдите через Discord, чтобы открыть личный кабинет.</p>
      <a class="primary" :href="loginUrl">Войти через Discord</a>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useAuth } from '~/composables/useAuth'

const router = useRouter()
const { authenticated, user, loginUrl, profilePath, linkMinecraft, logout, refresh } = useAuth()

const nickname = ref('')
const status = ref('')
const statusType = ref<'ok' | 'error'>('ok')
const submitting = ref(false)
const copied = ref(false)

const avatarUrl = computed(() => user.value?.avatarUrl || '/favicon.ico')
const displayName = computed(() => user.value?.displayName || user.value?.username || 'Пользователь')
const fullProfileUrl = computed(() => {
  if (!profilePath.value) return ''
  if (process.client) return `${window.location.origin}${profilePath.value}`
  return profilePath.value
})

onMounted(() => {
  void refresh()
})

watch(
  () => user.value?.linkedMinecraft,
  (linked) => {
    if (linked && !nickname.value) nickname.value = linked
  },
  { immediate: true }
)

const submitLink = async () => {
  status.value = ''

  if (!nickname.value || !/^[A-Za-z0-9_]{3,16}$/.test(nickname.value)) {
    statusType.value = 'error'
    status.value = 'Ник должен быть от 3 до 16 символов: латиница, цифры или _.'
    return
  }

  submitting.value = true
  try {
    await linkMinecraft(nickname.value)
    statusType.value = 'ok'
    status.value = 'Привязка обновлена.'
  } catch {
    statusType.value = 'error'
    status.value = 'Не удалось сохранить изменения. Повторите попытку.'
  } finally {
    submitting.value = false
  }
}

const copyProfileLink = async () => {
  copied.value = false
  if (!process.client || !fullProfileUrl.value) return

  try {
    await navigator.clipboard.writeText(fullProfileUrl.value)
    copied.value = true
  } catch {
    copied.value = false
  }
}

const handleLogout = async () => {
  await logout()
  await router.push('/')
}
</script>

<style scoped>
.profile-page {
  min-height: calc(100vh - 220px);
  display: grid;
}

.crm-shell {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: 18px;
}

.identity-column,
.workspace-column {
  display: grid;
  gap: 14px;
  align-content: start;
}

.panel {
  border: 1px solid rgba(255, 255, 255, 0.11);
  border-radius: 18px;
  background: linear-gradient(160deg, rgba(18, 19, 30, 0.92), rgba(11, 12, 20, 0.97));
  backdrop-filter: blur(8px);
  box-shadow: 0 18px 34px rgba(0, 0, 0, 0.3);
}

.identity-card {
  padding: 18px;
  display: grid;
  gap: 10px;
}

.avatar-wrap {
  position: relative;
  width: 88px;
  height: 88px;
}

.avatar {
  width: 88px;
  height: 88px;
  border-radius: 22px;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.08);
}

.online-dot {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #35f18c;
  border: 3px solid #161722;
  box-shadow: 0 0 12px rgba(53, 241, 140, 0.65);
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: rgba(255, 255, 255, 0.7);
}

h1,
h2,
h3 {
  margin: 0;
}

h1 {
  font-size: clamp(26px, 3.8vw, 34px);
}

h2 {
  font-size: clamp(20px, 3vw, 28px);
}

.muted {
  margin: 0;
  color: var(--muted);
}

.email {
  font-size: 13px;
}

.identity-actions {
  display: grid;
  gap: 8px;
}

.profile-link {
  color: #f8b7a4;
  text-decoration: none;
  word-break: break-all;
  font-size: 13px;
}

.mini-card {
  padding: 14px 16px;
  display: grid;
  gap: 6px;
}

.mini-label {
  margin: 0;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.09em;
  color: rgba(255, 255, 255, 0.6);
}

.mini-value {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
}

.workspace-head {
  padding: 16px 18px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.stat-card {
  padding: 16px;
  display: grid;
  gap: 10px;
}

.stat-card.highlight {
  background: linear-gradient(145deg, rgba(228, 94, 56, 0.2), rgba(11, 12, 20, 0.95));
}

.card-title {
  margin: 0;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgba(255, 255, 255, 0.62);
}

.stat-card strong {
  font-size: 24px;
  font-family: 'Neue Machine', 'Montserrat', sans-serif;
}

.stat-card ul {
  margin: 0;
  padding-left: 16px;
  display: grid;
  gap: 6px;
  color: var(--muted);
}

.form-panel {
  padding: 16px;
  display: grid;
  gap: 12px;
}

.form-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.badge {
  font-size: 11px;
  border-radius: 999px;
  padding: 6px 10px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.85);
}

.link-form {
  display: grid;
  gap: 8px;
}

.link-form label {
  font-size: 13px;
  color: var(--muted);
}

.row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
}

input {
  border-radius: 12px;
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.06);
  color: var(--text);
  padding: 10px 12px;
}

.primary,
.ghost {
  border-radius: 999px;
  padding: 10px 16px;
  border: 1px solid transparent;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-weight: 600;
}

.primary {
  color: #0b0b0f;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.ghost {
  color: var(--text);
  background: rgba(255, 255, 255, 0.07);
  border-color: rgba(255, 255, 255, 0.14);
}

.ghost.danger {
  color: #ffb9a8;
  border-color: rgba(228, 94, 56, 0.42);
  background: rgba(228, 94, 56, 0.14);
}

.status {
  margin: 0;
  color: #8af4ad;
}

.status.error {
  color: #ff9f9f;
}

.auth-empty {
  margin: auto;
  width: min(600px, 100%);
  display: grid;
  gap: 12px;
  text-align: center;
  padding: 28px;
}

@media (max-width: 1024px) {
  .crm-shell {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .row {
    grid-template-columns: 1fr;
  }

  .workspace-head {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
