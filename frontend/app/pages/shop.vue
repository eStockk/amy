<template>
  <div class="page">
    <SectionTitle text="Магазин" />
    <div class="grid">
      <ContentPanel title="Подписка" text="Доступ к ранним ивентам и косметическим наградам." />
      <ContentPanel title="Набор строителя" text="Декоративные блоки, питомец и статус." />
      <ContentPanel title="Поддержка проекта" text="Помогает развивать сервер и комьюнити." />
    </div>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import ContentPanel from '~/components/ContentPanel.vue'
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}
</style>