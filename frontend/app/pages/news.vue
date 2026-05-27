<template>
  <div class="page">
    <SectionTitle text="Новости" />

    <div class="tabs">
      <button type="button" :class="{ active: category === '' }" @click="category = ''">Все</button>
      <button type="button" :class="{ active: category === 'user' }" @click="category = 'user'">Пользовательские</button>
      <button type="button" :class="{ active: category === 'system' }" @click="category = 'system'">Системные</button>
    </div>

    <div v-if="pending" class="empty">
      <strong>Загружаем новости</strong>
      <p>Проверяем Discord-ленту сервера.</p>
    </div>

    <div v-else-if="error" class="empty">
      <strong>Новости временно недоступны</strong>
      <p>Проверьте доступ бота к каналам Discord и обновите страницу.</p>
    </div>

    <div v-else-if="isEmpty" class="empty">
      <strong>Пока новостей нет</strong>
      <p>Когда бот получит доступ к каналам, посты появятся здесь автоматически.</p>
    </div>

    <div v-else class="grid">
      <NewsCard
        v-for="item in news"
        :id="item.id"
        :key="item.id"
        :title="item.title"
        :intro="item.intro"
        :tags="item.tags"
        :source="item.source"
        :url="item.url"
        :created-at="item.createdAt"
        :variant="item.variant"
        :image-url="item.imageUrl"
        :author="item.author"
        :author-id="item.authorId"
        :author-avatar="item.authorAvatar"
        :like-count="item.likeCount"
        :comment-count="item.commentCount"
        :liked-by-me="item.likedByMe"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import NewsCard from '~/components/NewsCard.vue'
import { useNews } from '~/composables/useNews'

const category = ref('')
const { news, pending, error, refresh } = useNews(12, category)
const isEmpty = computed(() => !pending.value && news.value.length === 0)

watch(category, () => refresh())
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.tabs button {
  border: 1px solid var(--stroke);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
  color: var(--text);
  padding: 8px 13px;
  cursor: pointer;
}

.tabs button.active {
  border-color: rgba(240, 90, 60, 0.5);
  background: rgba(240, 90, 60, 0.16);
}

.empty {
  display: grid;
  gap: 8px;
  padding: 20px 24px;
  border-radius: 8px;
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
