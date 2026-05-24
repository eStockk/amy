<template>
  <div class="landing">
    <div class="hero-row">
      <HeroPanel />
      <div class="side-stack">
        <TrailerPanel />
      </div>
    </div>

    <section class="news">
      <div class="news-header">
        <SectionTitle text="Новости сервера" />
      </div>

      <div v-if="pending" class="empty">
        <div class="empty-icon">⏳</div>
        <div>
          <strong>Загружаем новости</strong>
          <p>Проверяем ленту сервера.</p>
        </div>
      </div>

      <div v-else-if="isEmpty" class="empty">
        <div class="empty-icon">🛰️</div>
        <div>
          <strong>Пока новостей нет</strong>
          <p>Следите за новостями — скоро здесь появятся анонсы.</p>
        </div>
      </div>

      <div v-else class="news-grid">
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
    </section>
  </div>
</template>

<script setup lang="ts">
import HeroPanel from '~/components/HeroPanel.vue'
import TrailerPanel from '~/components/TrailerPanel.vue'
import NewsCard from '~/components/NewsCard.vue'
import SectionTitle from '~/components/SectionTitle.vue'
import { useNews } from '~/composables/useNews'

const { news, pending } = useNews()
const isEmpty = computed(() => !pending.value && news.value.length === 0)
</script>

<style scoped>
.landing {
  display: flex;
  flex-direction: column;
  gap: 24px;
  min-height: 100%;
  flex: 1;
  position: relative;
}

.landing::before {
  content: '';
  position: absolute;
  inset: -80px -40px 0;
  background: radial-gradient(circle at 20% 20%, rgba(240, 90, 60, 0.18), transparent 60%),
    radial-gradient(circle at 80% 0%, rgba(198, 75, 122, 0.15), transparent 50%);
  filter: blur(20px);
  opacity: 0.8;
  z-index: 0;
  pointer-events: none;
}

.hero-row,
.news {
  position: relative;
  z-index: 1;
}

.hero-row {
  display: grid;
  grid-template-columns: minmax(0, 2.2fr) minmax(0, 1fr);
  gap: 24px;
}

.side-stack {
  display: grid;
  gap: 16px;
}

.news {
  display: grid;
  gap: 16px;
}

.news-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.news-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.empty {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 20px 24px;
  border-radius: var(--radius-md);
  border: 1px dashed rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.03);
  color: var(--muted);
}

.empty-icon {
  width: 42px;
  height: 42px;
  border-radius: 9px;
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.16);
  font-size: 18px;
}

@media (max-width: 1200px) {
  .hero-row {
    grid-template-columns: 1fr;
  }

  .news-grid {
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  }

}

@media (prefers-reduced-motion: reduce) {
  .landing::before {
    animation: none;
  }
}
</style>
