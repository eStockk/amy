<template>
  <div class="public-profile-page">
    <section class="profile-shell" v-if="pending">
      <p class="muted">Загружаем профиль...</p>
    </section>

    <section class="profile-shell" v-else-if="errorMessage">
      <h1>Профиль не найден</h1>
      <p class="muted">{{ errorMessage }}</p>
      <NuxtLink class="ghost" to="/">На главную</NuxtLink>
    </section>

    <section class="profile-shell" v-else-if="profile">
      <header class="profile-hero">
        <img class="avatar" :src="profile.avatarUrl || logo" alt="avatar" />
        <div>
          <p class="kicker">Публичный профиль</p>
          <h1>{{ profile.displayName }}</h1>
          <p class="muted">@{{ profile.username }}</p>
        </div>
      </header>

      <div class="stats-grid">
        <article class="stat-card">
          <p class="label">Игровой ник</p>
          <strong>{{ profile.linkedMinecraft || 'Не указан' }}</strong>
        </article>
        <article class="stat-card">
          <p class="label">В сообществе с</p>
          <strong>{{ joinedAt }}</strong>
        </article>
      </div>

      <section class="about-card">
        <h2>Об аккаунте</h2>
        <p class="muted">Профиль Discord, подключенный к серверу Amy.</p>
      </section>

      <footer class="actions">
        <NuxtLink v-if="isOwner" class="ghost" to="/profile">Открыть личный кабинет</NuxtLink>
        <NuxtLink class="ghost" to="/">На главную</NuxtLink>
      </footer>
    </section>
  </div>
</template>

<script setup lang="ts">
import logo from '~/assets/amy-logo.png'
import { useAuth } from '~/composables/useAuth'

type PublicProfile = {
  id: string
  username: string
  displayName: string
  avatarUrl: string
  linkedMinecraft?: string
  joinedAt?: string
}

type PublicProfileResponse = {
  profile: PublicProfile
}

const route = useRoute()
const config = useRuntimeConfig()
const { user, authenticated, refresh } = useAuth()

const pending = ref(true)
const errorMessage = ref('')
const profile = ref<PublicProfile | null>(null)

const profileId = computed(() => String(route.params.id || '').trim())
const isOwner = computed(() => Boolean(authenticated.value && user.value?.id && user.value.id === profile.value?.id))
const joinedAt = computed(() => {
  if (!profile.value?.joinedAt) {
    return 'Недавно'
  }

  const date = new Date(profile.value.joinedAt)
  if (Number.isNaN(date.getTime())) {
    return 'Недавно'
  }

  return new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: 'long', year: 'numeric' }).format(date)
})

const loadProfile = async () => {
  if (!profileId.value) {
    errorMessage.value = 'Некорректная ссылка профиля.'
    pending.value = false
    return
  }

  pending.value = true
  errorMessage.value = ''

  try {
    const response = await $fetch<PublicProfileResponse>(`${config.public.apiBase}/profiles/${profileId.value}`, {
      credentials: 'include'
    })
    profile.value = response.profile
  } catch {
    profile.value = null
    errorMessage.value = 'Проверьте ссылку или попробуйте позже.'
  } finally {
    pending.value = false
  }
}

onMounted(async () => {
  await refresh()
  await loadProfile()
})

watch(profileId, () => {
  void loadProfile()
})
</script>

<style scoped>
.public-profile-page {
  min-height: calc(100vh - 220px);
  display: grid;
  place-items: center;
}

.profile-shell {
  width: min(960px, 100%);
  display: grid;
  gap: 18px;
  padding: 24px;
  border-radius: 24px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(145deg, rgba(19, 22, 34, 0.95), rgba(10, 11, 20, 0.98));
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.35);
}

.profile-hero {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 18px;
  border-radius: 18px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: linear-gradient(120deg, rgba(228, 94, 56, 0.22), rgba(23, 25, 40, 0.62));
}

.avatar {
  width: 96px;
  height: 96px;
  border-radius: 24px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.08);
  object-fit: cover;
}

.kicker {
  margin: 0 0 8px;
  font-size: 12px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: rgba(255, 255, 255, 0.78);
}

h1,
h2 {
  margin: 0;
}

h1 {
  font-size: clamp(28px, 4vw, 40px);
}

.muted {
  margin: 0;
  color: var(--muted);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.stat-card,
.about-card {
  display: grid;
  gap: 8px;
  padding: 16px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
}

.stat-card strong {
  font-size: 24px;
  font-family: 'Neue Machine', 'Montserrat', sans-serif;
}

.label {
  margin: 0;
  color: rgba(255, 255, 255, 0.65);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.ghost {
  border-radius: 999px;
  padding: 10px 16px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  color: var(--text);
  background: rgba(255, 255, 255, 0.07);
  text-decoration: none;
  font-weight: 600;
}

@media (max-width: 900px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
