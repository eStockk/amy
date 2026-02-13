<template>
  <div class="cabinet-page">
    <section v-if="authenticated" class="cabinet-shell">
      <header class="hero-card">
        <img class="avatar" :src="avatarUrl" alt="avatar" />
        <div class="hero-meta">
          <p class="kicker">Личный кабинет</p>
          <h1>{{ displayName }}</h1>
          <p class="muted">@{{ user?.username }}<span v-if="user?.email"> - {{ user.email }}</span></p>
        </div>
      </header>

      <section class="cards-grid">
        <article class="info-card">
          <p class="card-title">Игровая привязка</p>
          <strong class="card-value">{{ user?.linkedMinecraft || 'Не привязано' }}</strong>
          <p class="muted">Этот ник используется для связи профиля Discord с серверным аккаунтом.</p>
        </article>

        <article class="info-card">
          <p class="card-title">Публичная ссылка</p>
          <a class="profile-link" :href="profilePath" target="_blank" rel="noreferrer">{{ fullProfileUrl }}</a>
          <div class="inline-actions">
            <button class="ghost" type="button" @click="copyProfileLink">{{ copied ? 'Скопировано' : 'Копировать' }}</button>
            <NuxtLink class="ghost" :to="profilePath">Открыть</NuxtLink>
          </div>
        </article>
      </section>

      <section class="link-panel">
        <h2>Привязка Minecraft аккаунта</h2>
        <p class="muted">Укажите свой ник на сервере для синхронизации с профилем.</p>

        <form class="link-form" @submit.prevent="submitLink">
          <label for="nickname">Ник в Minecraft</label>
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
              {{ submitting ? 'Сохраняем...' : 'Привязать ник' }}
            </button>
          </div>
        </form>

        <p v-if="status" class="status" :class="{ error: statusType === 'error' }">{{ status }}</p>
      </section>

      <footer class="cabinet-actions">
        <button class="ghost danger" type="button" @click="handleLogout">Выйти из аккаунта</button>
      </footer>
    </section>

    <section v-else class="cabinet-shell empty">
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
  if (!profilePath.value) {
    return ''
  }
  if (process.client) {
    return `${window.location.origin}${profilePath.value}`
  }
  return profilePath.value
})

onMounted(() => {
  void refresh()
})

watch(
  () => user.value?.linkedMinecraft,
  (linked) => {
    if (linked && !nickname.value) {
      nickname.value = linked
    }
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
  if (!process.client || !fullProfileUrl.value) {
    return
  }

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
.cabinet-page {
  min-height: calc(100vh - 220px);
  display: grid;
  place-items: center;
}

.cabinet-shell {
  width: min(940px, 100%);
  display: grid;
  gap: 18px;
  padding: 24px;
  border-radius: 24px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(145deg, rgba(20, 22, 34, 0.96), rgba(12, 13, 22, 0.98));
  box-shadow: 0 24px 50px rgba(0, 0, 0, 0.35);
}

.hero-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 18px;
  border-radius: 18px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: linear-gradient(120deg, rgba(228, 94, 56, 0.22), rgba(28, 30, 48, 0.6));
}

.avatar {
  width: 84px;
  height: 84px;
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.08);
  object-fit: cover;
}

.kicker {
  margin: 0 0 8px;
  font-size: 12px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: rgba(255, 255, 255, 0.82);
}

h1,
h2 {
  margin: 0;
}

h1 {
  font-size: clamp(28px, 4vw, 38px);
}

h2 {
  font-size: 22px;
}

.muted {
  margin: 0;
  color: var(--muted);
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.info-card {
  display: grid;
  gap: 10px;
  padding: 16px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
}

.card-title {
  margin: 0;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgba(255, 255, 255, 0.65);
}

.card-value {
  font-size: 24px;
  font-family: 'Neue Machine', 'Montserrat', sans-serif;
}

.profile-link {
  color: #ffd2c4;
  text-decoration: none;
  word-break: break-all;
}

.inline-actions {
  display: flex;
  gap: 10px;
}

.link-panel {
  display: grid;
  gap: 12px;
  padding: 16px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(14, 16, 28, 0.75);
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
  color: #ffb1a0;
  border-color: rgba(228, 94, 56, 0.42);
  background: rgba(228, 94, 56, 0.14);
}

.cabinet-actions {
  display: flex;
  justify-content: flex-end;
}

.status {
  margin: 0;
  color: #8af4ad;
}

.status.error {
  color: #ff9f9f;
}

.empty {
  text-align: center;
  place-items: center;
}

@media (max-width: 900px) {
  .cards-grid {
    grid-template-columns: 1fr;
  }

  .row {
    grid-template-columns: 1fr;
  }

  .cabinet-actions {
    justify-content: stretch;
  }

  .cabinet-actions .ghost {
    width: 100%;
  }
}
</style>
