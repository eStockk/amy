<template>
  <div class="page">
    <SectionTitle text="Новости" />

    <div v-if="pending" class="empty">
      <strong>Загружаем новости</strong>
      <p>Проверяем Telegram или Discord-ленту сервера.</p>
    </div>

    <div v-else-if="error" class="empty">
      <strong>Новости временно недоступны</strong>
      <p>Попробуйте обновить страницу позже.</p>
    </div>

    <div v-else-if="isEmpty" class="empty">
      <strong>Пока новостей нет</strong>
      <p>После подключения канала посты появятся здесь автоматически.</p>
    </div>

    <div v-else class="grid">
      <NewsCard
        v-for="item in news"
        :key="item.id"
        :title="item.title"
        :intro="item.intro"
        :tags="item.tags"
        :source="item.source"
        :url="item.url"
        :created-at="item.createdAt"
        :variant="item.variant"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import NewsCard from '~/components/NewsCard.vue'
import { useNews } from '~/composables/useNews'

const { news, pending, error } = useNews(9)
const isEmpty = computed(() => !pending.value && news.value.length === 0)
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

.empty {
  display: grid;
  gap: 8px;
  padding: 20px 24px;
  border-radius: var(--radius-md);
  border: 1px dashed rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.03);
  color: var(--muted);
}

.empty strong {
  color: var(--text);
}

.empty p {
  margin: 0;
}
</style>
