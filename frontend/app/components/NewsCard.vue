<template>
  <article class="card">
    <div class="card-header">
      <div class="tags">
        <span v-for="tag in tags" :key="tag"># {{ tag }}</span>
      </div>
      <button class="like" type="button">♡</button>
    </div>
    <div v-if="source || formattedDate" class="meta">
      <span v-if="source">{{ source }}</span>
      <span v-if="formattedDate">{{ formattedDate }}</span>
    </div>
    <h4>{{ title }}</h4>
    <p>{{ intro }}</p>
    <a v-if="url" class="primary" :href="url" target="_blank" rel="noreferrer">Открыть пост</a>
    <button v-else class="primary" type="button">Подробнее</button>
    <div class="media" :class="variant"></div>
  </article>
</template>

<script setup lang="ts">
const props = defineProps<{
  title: string
  intro: string
  tags: string[]
  source?: string
  url?: string
  createdAt?: string
  variant?: 'pink' | 'blue' | 'green'
}>()

const formattedDate = computed(() => {
  if (!props.createdAt) return ''
  const parsed = new Date(props.createdAt)
  if (Number.isNaN(parsed.getTime())) return ''

  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: 'short'
  }).format(parsed)
})
</script>

<style scoped>
.card {
  padding: 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: var(--panel);
  display: grid;
  gap: 10px;
  transition: transform 0.2s ease, border-color 0.2s ease;
}

.card:hover {
  transform: translateY(-4px);
  border-color: rgba(240, 90, 60, 0.4);
}

.card-header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
}

.tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.tags span {
  border: 1px solid var(--stroke);
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 11px;
  color: var(--muted);
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  color: var(--muted);
  font-size: 12px;
}

.meta span + span::before {
  content: '/';
  margin-right: 8px;
  color: rgba(255, 255, 255, 0.28);
}

.like {
  border-radius: 50%;
  border: 1px solid var(--stroke);
  background: transparent;
  color: var(--muted);
  width: 32px;
  height: 32px;
  cursor: pointer;
}

h4 {
  margin: 0;
  font-size: 18px;
}

p {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}

.primary {
  border-radius: 999px;
  padding: 8px 14px;
  border: none;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  color: #fff;
  width: max-content;
  cursor: pointer;
  text-decoration: none;
  font-size: 13px;
  font-weight: 600;
}

.media {
  height: 140px;
  border-radius: 9px;
  background: linear-gradient(135deg, #725c74, #d59abf);
}

.media.blue {
  background: linear-gradient(135deg, #394a6f, #7ea0ff);
}

.media.green {
  background: linear-gradient(135deg, #2d5e58, #65d2b2);
}
</style>
