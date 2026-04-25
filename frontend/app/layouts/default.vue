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
const router = useRouter()
const { authenticated, user } = useAuth()

let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let stopAuthWatch: (() => void) | null = null

const isTabActive = () => document.visibilityState === 'visible'

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

const handlePageHide = () => {
  void pingPresence(false)
}

const consumePostLoginAction = async () => {
  if (!import.meta.client || !authenticated.value || !user.value?.id) return

  const action = sessionStorage.getItem('amy:post-login-action')
  if (action !== 'rp-application') return

  sessionStorage.removeItem('amy:post-login-action')
  await router.push(`/u/${user.value.id}?apply=1`)
}

onMounted(() => {
  stopAuthWatch = watch(
    authenticated,
    (value) => {
      if (value) {
        startHeartbeat()
        void pingPresence(isTabActive())
        void consumePostLoginAction()
        return
      }
      stopHeartbeat()
    },
    { immediate: true }
  )

  document.addEventListener('visibilitychange', handleVisibility)
  window.addEventListener('pagehide', handlePageHide)
})

watch(
  () => user.value?.id,
  () => {
    void consumePostLoginAction()
  }
)

onBeforeUnmount(() => {
  stopHeartbeat()
  stopAuthWatch?.()
  stopAuthWatch = null
  document.removeEventListener('visibilitychange', handleVisibility)
  window.removeEventListener('pagehide', handlePageHide)
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
    padding: 16px 16px 0 92px;
  }

  .app-main {
    padding: 0;
  }
}

@media (max-width: 900px) {
  .app-content {
    padding: 72px 16px 0;
  }
}
</style>
