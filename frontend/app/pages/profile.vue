<template>
  <div class="profile-redirect">
    <section v-if="redirecting" class="card">
      <h1>Переходим в профиль...</h1>
      <p>Определяем ваш аккаунт.</p>
    </section>

    <section v-else class="card">
      <h1>Вход в профиль</h1>
      <p>Войдите через Discord, чтобы открыть личную страницу.</p>
      <a class="primary" :href="loginUrl">Войти через Discord</a>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useAuth } from '~/composables/useAuth'

const { authenticated, user, loginUrl, refresh } = useAuth()
const redirecting = ref(true)

onMounted(async () => {
  await refresh()
  if (authenticated.value && user.value?.id) {
    await navigateTo(`/u/${user.value.id}`, { replace: true })
    return
  }
  redirecting.value = false
})
</script>

<style scoped>
.profile-redirect {
  min-height: calc(100vh - 220px);
  display: grid;
  place-items: center;
}

.card {
  width: min(520px, 100%);
  display: grid;
  gap: 12px;
  padding: 28px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(15, 17, 27, 0.92);
}

h1,
p {
  margin: 0;
}

p {
  color: var(--muted);
}

.primary {
  width: fit-content;
  border-radius: 999px;
  padding: 10px 16px;
  text-decoration: none;
  color: #0b0b0f;
  font-weight: 600;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
}
</style>
