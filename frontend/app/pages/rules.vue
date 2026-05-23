<template>
  <div class="page">
    <SectionTitle text="Правила сервера" />
    <ContentPanel
      class="image-panel"
      title="Основные правила"
      text="Уважайте других игроков, не используйте читы и соблюдайте атмосферу RP."
      :style="{ '--panel-image': `url(${rulesImage})` }"
    >
      <ul>
        <li>Запрещены токсичность и оскорбления.</li>
        <li>Гриферство и разрушение чужих построек - под запретом.</li>
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
import rulesImage from '~/images/rules/rules1.png'
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.image-panel {
  min-height: 280px;
  align-content: end;
  overflow: hidden;
  background-image: linear-gradient(90deg, rgba(20, 20, 27, 0.96), rgba(20, 20, 27, 0.74) 52%, rgba(20, 20, 27, 0.26)),
    var(--panel-image);
  background-position: center, center;
  background-size: cover, cover;
  box-shadow: var(--shadow);
}

ul {
  margin: 0;
  padding-left: 18px;
  color: var(--muted);
}

@media (max-width: 640px) {
  .image-panel {
    min-height: 300px;
    background-image: linear-gradient(180deg, rgba(20, 20, 27, 0.72), rgba(20, 20, 27, 0.94)),
      var(--panel-image);
    background-position: center, center;
  }
}
</style>
