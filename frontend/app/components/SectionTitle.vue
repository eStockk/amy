<template>
  <div class="title">
    <h2>{{ text }}</h2>
    <span class="line"></span>
  </div>
</template>

<script setup lang="ts">
defineProps<{ text: string }>()
</script>

<style scoped>
.title {
  display: flex;
  align-items: center;
  gap: 12px;
}

h2 {
  margin: 0;
  font-size: 20px;
}

.line {
  flex: 1;
  height: 2px;
  background: linear-gradient(90deg, var(--accent), transparent);
}
</style>