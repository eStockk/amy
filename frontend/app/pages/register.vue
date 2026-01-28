<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="brand">
        <img :src="logo" alt="Amy logo" />
        <div>
          <h1>Регистрация в Amy</h1>
          <p>Создайте аккаунт и подайте заявку на сервер.</p>
        </div>
      </div>
      <form class="form" @submit.prevent="submit">
        <label>
          Имя *
          <input v-model="name" type="text" required />
        </label>
        <label>
          Email *
          <input v-model="email" type="email" required />
        </label>
        <label>
          Пароль *
          <input v-model="password" type="password" required minlength="6" />
        </label>
        <button type="submit" class="primary">Создать аккаунт</button>
        <p v-if="message" class="message">{{ message }}</p>
      </form>
      <div class="switch">
        Уже есть аккаунт?
        <NuxtLink to="/login">Войти</NuxtLink>
      </div>
    </div>

    <aside class="side">
      <div class="side-card">
        <h3>Сезонные события</h3>
        <p>Получайте доступ к сюжетам и редким наградам.</p>
      </div>
      <div class="side-card">
        <h3>Безопасное хранение</h3>
        <p>Мы не сохраняем пароль в открытом виде.</p>
      </div>
      <div class="side-card">
        <h3>Личный кабинет</h3>
        <p>Управляйте профилем и настройками сервера.</p>
      </div>
    </aside>
  </div>
</template>

<script setup lang="ts">
import logo from '~/assets/amy-logo.png'

const name = ref('')
const email = ref('')
const password = ref('')
const message = ref('')
const config = useRuntimeConfig()

const submit = async () => {
  message.value = ''
  try {
    await $fetch(`${config.public.apiBase}/api/auth/register`, {
      method: 'POST',
      body: { name: name.value, email: email.value, password: password.value }
    })
    message.value = 'Регистрация успешна. Теперь войдите.'
  } catch {
    message.value = 'Не удалось зарегистрироваться. Проверьте данные.'
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: grid;
  grid-template-columns: minmax(0, 420px) minmax(0, 1fr);
  gap: 48px;
  padding: 48px 24px 60px 120px;
  align-items: center;
  position: relative;
}

.auth-page::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 15% 20%, rgba(228, 94, 56, 0.22), transparent 40%),
    radial-gradient(circle at 70% 0%, rgba(180, 67, 44, 0.2), transparent 45%),
    var(--bg);
  z-index: 0;
}

.auth-card {
  background: rgba(20, 20, 26, 0.92);
  color: var(--text);
  border-radius: 18px;
  padding: 28px;
  display: grid;
  gap: 20px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 30px 60px rgba(0, 0, 0, 0.45);
  position: relative;
  z-index: 1;
}

.brand {
  display: grid;
  grid-template-columns: 40px 1fr;
  gap: 12px;
  align-items: center;
}

.brand img {
  width: 40px;
  height: 40px;
  object-fit: contain;
}

h1 {
  margin: 0;
  font-size: 22px;
}

p {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.form {
  display: grid;
  gap: 14px;
}

label {
  display: grid;
  gap: 6px;
  font-size: 13px;
  color: var(--muted);
}

input {
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 14px;
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
}

input:focus {
  outline: 2px solid rgba(228, 94, 56, 0.35);
  border-color: #E45E38;
}

.primary {
  border: none;
  border-radius: 10px;
  padding: 12px 16px;
  background: #E45E38;
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.message {
  color: var(--muted);
  font-size: 13px;
}

.switch {
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding-top: 12px;
  font-size: 13px;
  color: var(--muted);
  display: flex;
  justify-content: space-between;
}

.switch a {
  color: #E45E38;
  text-decoration: none;
}

.side {
  display: grid;
  gap: 16px;
  position: relative;
  z-index: 1;
}

.side-card {
  padding: 18px 20px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #d7d7e0;
  backdrop-filter: blur(6px);
}

.side-card h3 {
  margin: 0 0 8px;
  color: #f3f3f7;
  font-size: 16px;
}

.side-card p {
  margin: 0;
  color: #b7b7c6;
}

@media (max-width: 1024px) {
  .auth-page {
    grid-template-columns: 1fr;
    padding: 32px 16px 40px;
  }

  .side {
    order: -1;
  }
}
</style>