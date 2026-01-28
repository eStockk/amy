<template>
  <div class="page">
    <SectionTitle text="Правила сервера" />
    <ContentPanel
      title="Основные правила"
      text="Уважайте других игроков, не используйте читы и соблюдайте атмосферу RP."
    >
      <ul>
        <li>Запрещены токсичность и оскорбления.</li>
        <li>Гриферство и разрушение чужих построек — под запретом.</li>
        <li>Экономика и торговля строятся на честной игре.</li>
      </ul>
    </ContentPanel>
    <ContentPanel
      title="Контент"
      text="Мы поддерживаем оригинальную ванильную атмосферу и бережно относимся к лору."
    />
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

ul {
  margin: 0;
  padding-left: 18px;
  color: var(--muted);
}
</style>