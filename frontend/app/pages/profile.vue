<template>
  <div class="profile-page">
    <section class="profile-card" v-if="authenticated">
      <header class="profile-head">
        <img class="avatar" :src="avatarUrl" alt="avatar" />
        <div class="head-meta">
          <h1>{{ displayName }}</h1>
          <p class="muted">@{{ user?.username }}<span v-if="user?.email"> - {{ user.email }}</span></p>
        </div>
      </header>

      <div class="stats-grid">
        <article class="stat">
          <span class="label">Discord ID</span>
          <strong>{{ user?.id }}</strong>
        </article>
        <article class="stat">
          <span class="label">Linked nickname</span>
          <strong>{{ user?.linkedMinecraft || 'Not linked' }}</strong>
        </article>
      </div>

      <section class="link-panel">
        <h2>Link Minecraft account</h2>
        <p class="muted">Enter your server nickname. This will bind your Discord profile with your in-game account.</p>

        <form class="link-form" @submit.prevent="submitLink">
          <label for="nickname">Minecraft nickname</label>
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
              {{ submitting ? 'Saving...' : 'Link account' }}
            </button>
          </div>
        </form>

        <p v-if="status" class="status" :class="{ error: statusType === 'error' }">{{ status }}</p>
      </section>
    </section>

    <section v-else class="profile-card empty">
      <h1>Profile unavailable</h1>
      <p class="muted">Sign in with Discord to open your personal page.</p>
      <a class="primary" :href="loginUrl">Login with Discord</a>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useAuth } from '~/composables/useAuth'

const { authenticated, user, loginUrl, linkMinecraft, refresh } = useAuth()

const nickname = ref('')
const status = ref('')
const statusType = ref<'ok' | 'error'>('ok')
const submitting = ref(false)

const avatarUrl = computed(() => user.value?.avatarUrl || '/favicon.ico')
const displayName = computed(() => user.value?.displayName || user.value?.username || 'User')

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
    status.value = 'Nickname must be 3-16 chars and contain only latin letters, digits or _.'
    return
  }

  submitting.value = true
  try {
    await linkMinecraft(nickname.value)
    statusType.value = 'ok'
    status.value = 'Minecraft account linked successfully.'
  } catch {
    statusType.value = 'error'
    status.value = 'Failed to link account. Try again.'
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.profile-page {
  min-height: calc(100vh - 220px);
  display: grid;
  place-items: center;
}

.profile-card {
  width: min(92vw, 760px);
  display: grid;
  gap: 18px;
  padding: 26px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: var(--panel);
  box-shadow: var(--shadow);
}

.profile-head {
  display: flex;
  align-items: center;
  gap: 16px;
}

.avatar {
  width: 84px;
  height: 84px;
  border-radius: 20px;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.06);
}

h1,
h2 {
  margin: 0;
}

h2 {
  font-size: 22px;
}

.muted {
  color: var(--muted);
  margin: 0;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.stat {
  border: 1px solid rgba(255, 255, 255, 0.09);
  background: rgba(255, 255, 255, 0.04);
  border-radius: 12px;
  padding: 12px 14px;
  display: grid;
  gap: 4px;
}

.label {
  color: var(--muted);
  font-size: 12px;
}

.link-panel {
  display: grid;
  gap: 12px;
  padding: 16px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
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
  background: rgba(255, 255, 255, 0.05);
  color: var(--text);
  padding: 10px 12px;
}

.primary {
  border-radius: 999px;
  padding: 10px 18px;
  border: 1px solid transparent;
  cursor: pointer;
  font-weight: 600;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #0b0b0f;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.primary:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.status {
  margin: 0;
  color: #8af4ad;
}

.status.error {
  color: #ff9999;
}

.empty {
  text-align: center;
  place-items: center;
}

@media (max-width: 760px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .row {
    grid-template-columns: 1fr;
  }
}
</style>
