<template>
  <div class="page">
    <SectionTitle text="F.A.Q" />
    <div class="faq">
      <details open>
        <summary>Какая версия сервера?</summary>
        <p>Мы работаем на актуальной версии с балансными изменениями.</p>
      </details>
      <details>
        <summary>Нужен ли донат?</summary>
        <p>Донат не влияет на баланс, только косметика и поддержка проекта.</p>
      </details>
      <details>
        <summary>Как подать заявку?</summary>
        <p>Нажмите кнопку “Подать заявку” на главной и заполните анкету.</p>
      </details>
    </div>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.faq {
  display: grid;
  gap: 12px;
}

summary {
  cursor: pointer;
  font-weight: 600;
}

details {
  border: 1px solid var(--stroke);
  border-radius: var(--radius-sm);
  padding: 12px 16px;
  background: var(--panel);
}

p {
  margin: 8px 0 0;
  color: var(--muted);
}
</style>