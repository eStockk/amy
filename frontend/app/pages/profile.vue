<template>
  <div class="profile">
    <div class="card">
      <div v-if="authenticated" class="header">
        <img class="avatar" :src="avatarUrl" alt="avatar" />
        <div>
          <h1>{{ displayName }}</h1>
          <p>@{{ user?.username }}<span v-if="user?.email"> · {{ user.email }}</span></p>
        </div>
      </div>

      <div v-if="authenticated" class="details">
        <div class="detail">
          <span class="label">Discord ID</span>
          <span>{{ user?.id }}</span>
        </div>
        <div class="detail">
          <span class="label">Linked Minecraft</span>
          <span>{{ user?.linkedMinecraft || 'Not linked yet' }}</span>
        </div>
      </div>

      <section v-if="authenticated" class="link-panel">
        <h2>Connect Minecraft account</h2>
        <p class="muted">
          Enter your server nickname. This links your Discord profile with in-game account.
        </p>

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
            <button type="submit" class="primary" :disabled="submitting">{{ submitting ? 'Saving...' : 'Connect' }}</button>
          </div>
        </form>

        <p v-if="status" class="status" :class="{ error: statusType === 'error' }">{{ status }}</p>
      </section>

      <div v-else class="empty">
        <h1>You are not authenticated</h1>
        <p>Login via Discord to open your personal page.</p>
        <a class="primary" :href="loginUrl">Login with Discord</a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAuth } from '~/composables/useAuth'

const { authenticated, user, loginUrl, linkMinecraft } = useAuth()

const nickname = ref('')
const status = ref('')
const statusType = ref<'ok' | 'error'>('ok')
const submitting = ref(false)

const avatarUrl = computed(() => user.value?.avatarUrl || '/favicon.ico')
const displayName = computed(() => user.value?.displayName || user.value?.username || 'User')

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
    status.value = 'Nickname must be 3-16 chars and contain only latin letters, digits or _'
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
.profile {
  display: grid;
  place-items: center;
  min-height: calc(100vh - 200px);
}

.card {
  width: min(92vw, 780px);
  background: var(--panel);
  border: 1px solid var(--stroke);
  border-radius: var(--radius-md);
  padding: 28px;
  display: grid;
  gap: 20px;
}

.header {
  display: flex;
  gap: 18px;
  align-items: center;
}

.avatar {
  width: 84px;
  height: 84px;
  border-radius: 22px;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.08);
}

h1 {
  margin: 0 0 6px;
}

h2 {
  margin: 0;
  font-size: 22px;
}

p {
  margin: 0;
  color: var(--muted);
}

.details {
  display: grid;
  gap: 12px;
}

.detail {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 14px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.detail .label {
  color: var(--muted);
}

.link-panel {
  display: grid;
  gap: 12px;
  padding: 18px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
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
  padding: 10px 12px;
  color: var(--text);
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
  gap: 8px;
  color: #0b0b0f;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.status {
  color: #86efac;
}

.status.error {
  color: #fca5a5;
}

.empty {
  text-align: center;
  display: grid;
  gap: 12px;
}

@media (max-width: 720px) {
  .row {
    grid-template-columns: 1fr;
  }
}
</style>
