<template>
  <div class="page">
    <SectionTitle text="Новости" />
    <div class="grid">
      <NewsCard
        title="Новые механики"
        intro="Вступление бла бла бла"
        :tags="['Ролевая игра', 'Лор']"
        variant="pink"
      />
      <NewsCard
        title="Сюжетный ивент"
        intro="Вступление бла бла бла"
        :tags="['События', 'Истории']"
        variant="blue"
      />
      <NewsCard
        title="Редкие артефакты"
        intro="Вступление бла бла бла"
        :tags="['Экспедиции', 'Легенды']"
        variant="green"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import NewsCard from '~/components/NewsCard.vue'
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