<template>
  <section class="panel">
    <h2>{{ title }}</h2>
    <p>{{ text }}</p>
    <slot />
  </section>
</template>

<script setup lang="ts">
defineProps<{ title: string; text: string }>()
</script>

<style scoped>
.panel {
  padding: 24px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: var(--panel);
  display: grid;
  gap: 12px;
}

h2 {
  margin: 0;
  font-size: 22px;
}

p {
  margin: 0;
  color: var(--muted);
}
</style>