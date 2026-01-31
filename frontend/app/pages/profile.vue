<template>
  <div class="profile">
    <div class="card">
      <div v-if="authenticated" class="header">
        <img class="avatar" :src="avatarUrl" alt="avatar" />
        <div>
          <h1>{{ user?.username }}</h1>
          <p>{{ user?.email || 'Email не указан' }}</p>
        </div>
      </div>
      <div v-else class="empty">
        <h1>Вы не авторизованы</h1>
        <p>Войдите через Discord, чтобы увидеть свой профиль.</p>
        <a class="primary" :href="loginUrl">Войти через Discord</a>
      </div>
      <div v-if="authenticated" class="details">
        <div class="detail">
          <span class="label">Discord ID</span>
          <span>{{ user?.id }}</span>
        </div>
        <div class="detail">
          <span class="label">Никнейм</span>
          <span>{{ user?.username }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAuth } from '~/composables/useAuth'

const { authenticated, user, loginUrl } = useAuth()
const avatarUrl = computed(() => user.value?.avatarUrl || '/favicon.ico')
</script>

<style scoped>
.profile {
  display: grid;
  place-items: center;
  min-height: calc(100vh - 200px);
}

.card {
  width: min(92vw, 720px);
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
  width: 80px;
  height: 80px;
  border-radius: 22px;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.08);
}

h1 {
  margin: 0 0 6px;
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

.empty {
  text-align: center;
  display: grid;
  gap: 12px;
}

.primary {
  justify-self: center;
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
</style>
