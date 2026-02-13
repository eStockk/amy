<template>
  <div class="support">
    <div class="card">
      <h1>Поддержка</h1>
      <p>Оставьте тикет — мы ответим в ближайшее время.</p>
      <form @submit.prevent="submit">
        <label>
          Имя *
          <input v-model="name" type="text" required />
        </label>
        <label>
          Email *
          <input v-model="email" type="email" required />
        </label>
        <label>
          Тема *
          <input v-model="subject" type="text" required />
        </label>
        <label>
          Категория
          <select v-model="category">
            <option>Общие вопросы</option>
            <option>Техническая проблема</option>
            <option>Оплата</option>
            <option>Другое</option>
          </select>
        </label>
        <label>
          Сообщение *
          <textarea v-model="message" rows="5" required></textarea>
        </label>
        <button type="submit" class="primary">Отправить тикет</button>
        <p v-if="status" class="status">{{ status }}</p>
      </form>
    </div>
    <div class="info">
      <h3>Дополнительно</h3>
      <p>Для быстрых ответов используйте Discord.</p>
      <div class="links">
        <a href="#">Discord</a>
        <a href="#">Email</a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const name = ref('')
const email = ref('')
const subject = ref('')
const category = ref('Общие вопросы')
const message = ref('')
const status = ref('')
const config = useRuntimeConfig()

const submit = async () => {
  status.value = ''
  try {
    await $fetch(`${config.public.apiBase}/support/tickets`, {
      method: 'POST',
      body: {
        name: name.value,
        email: email.value,
        subject: subject.value,
        category: category.value,
        message: message.value
      }
    })
    status.value = 'Тикет отправлен. Мы свяжемся с вами.'
    name.value = ''
    email.value = ''
    subject.value = ''
    message.value = ''
  } catch {
    status.value = 'Не удалось отправить тикет. Попробуйте позже.'
  }
}
</script>

<style scoped>
.support {
  display: grid;
  grid-template-columns: minmax(0, 520px) minmax(0, 1fr);
  gap: 24px;
  min-height: calc(100vh - 140px);
}

.card {
  background: var(--panel);
  border: 1px solid var(--stroke);
  border-radius: var(--radius-md);
  padding: 24px;
  display: grid;
  gap: 12px;
}

p {
  margin: 0;
  color: var(--muted);
}

form {
  display: grid;
  gap: 12px;
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
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 12px;
  padding: 10px 12px;
  color: var(--text);
}

.primary {
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  border: none;
  border-radius: 999px;
  padding: 10px 16px;
  color: #0b0b0f;
  font-weight: 600;
  cursor: pointer;
}

.status {
  color: var(--muted);
  font-size: 13px;
}

.info {
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.04);
  padding: 24px;
  display: grid;
  gap: 12px;
  align-content: start;
}

.links {
  display: flex;
  gap: 14px;
}

.links a {
  color: var(--text);
  text-decoration: none;
}

@media (max-width: 1024px) {
  .support {
    grid-template-columns: 1fr;
  }
}
</style>