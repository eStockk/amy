<template>
  <div class="app-layout">
    <BaseSidebar />
    <div class="app-content">
      <TopBar />
      <main class="app-main">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import BaseSidebar from '~/components/BaseSidebar.vue'
import TopBar from '~/components/TopBar.vue'
</script>

<style scoped>
.app-layout {
  min-height: 100%;
}

.app-content {
  min-height: 100%;
  padding: 24px 24px 0 92px;
  display: flex;
  flex-direction: column;
}

.app-main {
  padding: 0 24px 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}

@media (max-width: 1024px) {
  .app-content {
    padding: 16px;
  }

  .app-main {
    padding: 0;
  }
}
</style>
