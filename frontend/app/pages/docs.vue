<template>
  <div class="page">
    <SectionTitle text="Документация" />
    <ContentPanel
      title="Как подключиться"
      text="IP сервера и инструкция по установке лаунчера."
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
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}
</style>