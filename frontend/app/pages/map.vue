<template>
  <div class="page">
    <SectionTitle text="Карта" />
    <ContentPanel
      title="Интерактивная карта"
      text="Скоро здесь появится интерактивная карта мира с отметками фракций и локаций."
    >
      <div class="map" role="img" aria-label="Карта сервера"></div>
    </ContentPanel>
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

.map {
  height: 260px;
  border-radius: var(--radius-md);
  border: 1px dashed var(--stroke);
  background: linear-gradient(135deg, #2f2e3a, #1c1c28);
}
</style>