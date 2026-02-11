<template>
  <header class="topbar">
    <div class="search">
      <svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
        <path
          d="M11 4a7 7 0 1 1 0 14 7 7 0 0 1 0-14Zm0-2a9 9 0 1 0 5.62 16.04l4.17 4.17 1.41-1.41-4.17-4.17A9 9 0 0 0 11 2Z"
          fill="currentColor"
        />
      </svg>
      <input type="text" placeholder="Поиск статьи..." />
    </div>
    <div class="actions">
      <button class="primary" type="button" @click="supportOpen = true">Поддержка</button>
    </div>
    <SupportModal :open="supportOpen" @close="supportOpen = false" />
  </header>
</template>

<script setup lang="ts">
import SupportModal from '~/components/SupportModal.vue'

const supportOpen = ref(false)
</script>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 24px 16px;
}

.search {
  flex: 1;
  max-width: 520px;
  display: flex;
  align-items: center;
  gap: 10px;
  background: var(--panel);
  border: 1px solid var(--stroke);
  border-radius: 999px;
  padding: 10px 16px;
  color: var(--muted);
}

.icon {
  width: 18px;
  height: 18px;
}

.search input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: var(--text);
}

.actions {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.primary {
  border-radius: 999px;
  padding: 10px 18px;
  border: 1px solid transparent;
  cursor: pointer;
  font-weight: 600;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #0b0b0f;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
  box-shadow: 0 10px 22px rgba(240, 90, 60, 0.35);
}

@media (max-width: 1024px) {
  .topbar {
    flex-direction: column;
    align-items: stretch;
    padding: 0 0 16px;
  }

  .search {
    max-width: 100%;
  }
}
</style>

