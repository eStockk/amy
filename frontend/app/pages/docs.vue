<template>
  <div class="page">
    <SectionTitle text="Документация" />
    <ContentPanel
      class="image-panel"
      title="Как подключиться"
      text="IP сервера и инструкция по установке лаунчера."
      :style="{ '--panel-image': `url(${docsImage})` }"
    />
    <ContentPanel
      title="Гайд новичка"
      text="С чего начать, как вступить в фракцию и найти команду."
    />
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import ContentPanel from '~/components/ContentPanel.vue'
import docsImage from '~/images/doc/doc1.png'
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.image-panel {
  min-height: 300px;
  align-content: end;
  overflow: hidden;
  background-image: linear-gradient(90deg, rgba(20, 20, 27, 0.96), rgba(20, 20, 27, 0.72) 50%, rgba(20, 20, 27, 0.24)),
    var(--panel-image);
  background-position: center, center;
  background-size: cover, cover;
  box-shadow: var(--shadow);
}

@media (max-width: 640px) {
  .image-panel {
    min-height: 300px;
    background-image: linear-gradient(180deg, rgba(20, 20, 27, 0.66), rgba(20, 20, 27, 0.94)),
      var(--panel-image);
    background-position: center, center;
  }
}
</style>
