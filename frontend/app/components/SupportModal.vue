<template>
  <Teleport to="body">
    <div v-if="open" class="modal" @keydown.esc="emit('close')" tabindex="-1">
      <div class="backdrop" @click="emit('close')"></div>
      <div class="panel" role="dialog" aria-modal="true">
        <button class="close" type="button" @click="emit('close')">x</button>
        <h2>Поддержка</h2>
        <p>Оставьте тикет и мы ответим в ближайшее время.</p>
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
            <textarea v-model="message" rows="4" required></textarea>
          </label>
          <button type="submit" class="primary" :disabled="sending">Отправить тикет</button>
          <p v-if="status" class="status">{{ status }}</p>
        </form>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const name = ref('')
const email = ref('')
const subject = ref('')
const category = ref('Общие вопросы')
const message = ref('')
const status = ref('')
const sending = ref(false)
const config = useRuntimeConfig()

const submit = async () => {
  status.value = ''
  sending.value = true
  try {
    await $fetch(`${config.public.apiBase}/api/support/tickets`, {
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
  } finally {
    sending.value = false
  }
}
</script>

<style scoped>
.modal {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
}

.backdrop {
  position: absolute;
  inset: 0;
  background: rgba(5, 5, 8, 0.7);
  backdrop-filter: blur(6px);
}

.panel {
  position: relative;
  width: min(92vw, 520px);
  background: linear-gradient(160deg, rgba(28, 28, 36, 0.96), rgba(17, 17, 24, 0.95));
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 24px;
  padding: 24px;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.6);
  display: grid;
  gap: 12px;
  z-index: 1;
}

.close {
  position: absolute;
  top: 14px;
  right: 14px;
  width: 32px;
  height: 32px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
  cursor: pointer;
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

.primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.status {
  color: var(--muted);
  font-size: 13px;
}
</style>
