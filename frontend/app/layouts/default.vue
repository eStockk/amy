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
import { useAuth } from '~/composables/useAuth'

const config = useRuntimeConfig()
const { authenticated } = useAuth()

let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let stopAuthWatch: (() => void) | null = null

const isTabActive = () => document.visibilityState === 'visible' && document.hasFocus()

const pingPresence = async (active: boolean) => {
  if (!authenticated.value) return
  try {
    await $fetch(`${config.public.apiBase}/auth/presence`, {
      method: 'POST',
      credentials: 'include',
      body: { active }
    })
  } catch {
    // presence is best-effort, do not break UI
  }
}

const startHeartbeat = () => {
  if (heartbeatTimer) return
  heartbeatTimer = setInterval(() => {
    void pingPresence(isTabActive())
  }, 25000)
}

const stopHeartbeat = () => {
  if (!heartbeatTimer) return
  clearInterval(heartbeatTimer)
  heartbeatTimer = null
}

const handleVisibility = () => {
  void pingPresence(isTabActive())
}

onMounted(() => {
  stopAuthWatch = watch(
    authenticated,
    (value) => {
      if (value) {
        startHeartbeat()
        void pingPresence(isTabActive())
        return
      }
      stopHeartbeat()
    },
    { immediate: true }
  )

  document.addEventListener('visibilitychange', handleVisibility)
  window.addEventListener('focus', handleVisibility)
  window.addEventListener('blur', handleVisibility)
})

onBeforeUnmount(() => {
  stopHeartbeat()
  stopAuthWatch?.()
  stopAuthWatch = null
  document.removeEventListener('visibilitychange', handleVisibility)
  window.removeEventListener('focus', handleVisibility)
  window.removeEventListener('blur', handleVisibility)
  void pingPresence(false)
})
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
