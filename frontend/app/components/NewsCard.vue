<template>
  <article class="card">
    <div class="card-header">
      <div class="tags">
        <span v-for="tag in tags" :key="tag"># {{ tag }}</span>
      </div>
      <button class="like" type="button">♡</button>
    </div>
    <h4>{{ title }}</h4>
    <p>{{ intro }}</p>
    <button class="primary" type="button">Подробнее</button>
    <div class="media" :class="variant"></div>
  </article>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  intro: string
  tags: string[]
  variant?: 'pink' | 'blue' | 'green'
}>()
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