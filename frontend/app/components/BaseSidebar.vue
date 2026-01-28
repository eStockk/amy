<template>
  <div>
    <div v-if="isOpen" class="overlay" @click="close"></div>
    <aside class="sidebar" :class="{ open: isOpen }">
      <div class="logo">
        <button class="toggle" type="button" @click="toggle" aria-label="Открыть меню">☰</button>
        <span class="logo-mark">
          <img :src="logo" alt="Amy logo" />
        </span>
        <span class="logo-text">Amy</span>
      </div>
      <nav class="nav">
        <NuxtLink class="nav-link" to="/" @click="close">
          <span class="icon">⌂</span>
          <span class="label">Главная</span>
        </NuxtLink>
        <NuxtLink class="nav-link" to="/news" @click="close">
          <span class="icon">✦</span>
          <span class="label">Новости</span>
        </NuxtLink>
        <NuxtLink class="nav-link" to="/rules" @click="close">
          <span class="icon">◆</span>
          <span class="label">Правила</span>
        </NuxtLink>
        <NuxtLink class="nav-link" to="/shop" @click="close">
          <span class="icon">☰</span>
          <span class="label">Магазин</span>
        </NuxtLink>
        <NuxtLink class="nav-link" to="/faq" @click="close">
          <span class="icon">?</span>
          <span class="label">F.A.Q</span>
        </NuxtLink>
        <NuxtLink class="nav-link" to="/map" @click="close">
          <span class="icon">◎</span>
          <span class="label">Карта</span>
        </NuxtLink>
        <NuxtLink class="nav-link" to="/docs" @click="close">
          <span class="icon">⬚</span>
          <span class="label">Документация</span>
        </NuxtLink>
      </nav>
    <div class="sidebar-footer">
      <a class="pill discord" :href="discordLoginUrl" @click="close">
        <img :src="discordIcon" alt="Discord" />
        <span class="label">Войти через Discord</span>
      </a>
    </div>
      <div class="edge"></div>
    </aside>
  </div>
</template>

<script setup lang="ts">
import logo from '~/assets/amy-logo.png'
import discordIcon from '~/assets/discord.png'

const isOpen = ref(false)
const config = useRuntimeConfig()
const discordLoginUrl = computed(() => `${config.public.apiBase}/api/auth/discord/start`)
const route = useRoute()

const toggle = () => {
  isOpen.value = !isOpen.value
}

const close = () => {
  isOpen.value = false
}

watch(
  () => route.fullPath,
  () => {
    isOpen.value = false
  }
)
</script>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px);
  z-index: 30;
}

.sidebar {
  position: fixed;
  top: 0;
  left: 0;
  height: 100vh;
  width: 72px;
  padding: 16px 12px 20px;
  display: grid;
  grid-template-rows: auto 1fr auto;
  gap: 18px;
  background: linear-gradient(180deg, rgba(38, 36, 48, 0.98), rgba(14, 14, 20, 0.96));
  border-right: 1px solid rgba(255, 255, 255, 0.12);
  box-shadow: inset -12px 0 24px rgba(0, 0, 0, 0.55);
  overflow: hidden;
  transition: width 0.35s ease, box-shadow 0.35s ease;
  z-index: 40;
}

.sidebar:hover,
.sidebar.open {
  width: 260px;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.6);
}

.logo {
  display: grid;
  justify-items: center;
  gap: 6px;
  font-weight: 700;
  position: relative;
}

.toggle {
  display: none;
  position: absolute;
  top: 0;
  left: 0;
  width: 32px;
  height: 32px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
  cursor: pointer;
}

.logo-mark {
  width: 40px;
  height: 40px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.16);
  padding: 6px;
}

.logo-mark img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.logo-text {
  font-size: 14px;
  opacity: 0;
  transform: translateX(-8px);
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.sidebar:hover .logo-text,
.sidebar.open .logo-text {
  opacity: 1;
  transform: translateX(0);
}

.nav {
  display: grid;
  gap: 10px;
}

.nav-link {
  text-decoration: none;
  padding: 10px 12px;
  border-radius: 14px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}

.nav-link.router-link-active {
  color: var(--text);
  border-color: var(--stroke);
  background: var(--panel);
}

.nav-link:hover {
  color: var(--text);
  border-color: var(--stroke);
  background: var(--glass);
}

.icon {
  width: 32px;
  height: 32px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.18);
  font-size: 12px;
  flex-shrink: 0;
}

.label {
  white-space: nowrap;
  opacity: 0;
  transform: translateX(-8px);
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.sidebar:hover .label,
.sidebar.open .label {
  opacity: 1;
  transform: translateX(0);
}

.sidebar-footer {
  display: grid;
  gap: 10px;
}

.pill {
  border: 1px solid var(--stroke);
  background: var(--panel);
  color: var(--text);
  padding: 10px 12px;
  border-radius: 999px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
}

.pill.discord {
  background: rgba(88, 101, 242, 0.16);
  border-color: rgba(88, 101, 242, 0.4);
  color: #dfe2ff;
}

.pill.discord img {
  width: 18px;
  height: 18px;
}

.edge {
  position: absolute;
  top: 14%;
  right: -6px;
  width: 10px;
  height: 160px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(240, 90, 60, 0.85), rgba(198, 75, 122, 0.85));
  box-shadow: 0 0 18px rgba(240, 90, 60, 0.6);
}

@media (max-width: 900px) {
  .sidebar {
    width: 64px;
  }

  .toggle {
    display: grid;
    place-items: center;
  }

  .sidebar:hover {
    width: 64px;
    box-shadow: inset -12px 0 24px rgba(0, 0, 0, 0.55);
  }

  .sidebar.open {
    width: min(82vw, 260px);
  }

  .sidebar:hover .label,
  .sidebar:hover .logo-text {
    opacity: 0;
    transform: translateX(-8px);
  }
}
</style>
