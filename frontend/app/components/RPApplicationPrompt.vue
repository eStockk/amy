<template>
  <Teleport to="body">
    <Transition name="prompt-slide">
      <aside v-if="showLoginNotice" class="login-notice" aria-live="polite">
        <button class="icon-close" type="button" aria-label="Закрыть" @click="dismissNotice">x</button>
        <span class="prompt-icon"><AppIcon name="apply" /></span>
        <div class="notice-copy">
          <strong>Хотите попасть на сервер?</strong>
          <p>Авторизуйтесь через Discord, чтобы заполнить RP-анкету для входа.</p>
          <button class="primary-button" type="button" @click="startDiscordLogin">Войти через Discord</button>
        </div>
      </aside>
    </Transition>

    <div v-if="showApplicationModal" class="application-modal" @keydown.esc="closeModal" tabindex="-1">
      <div class="modal-backdrop" @click="closeModal"></div>
      <section class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="rp-prompt-title">
        <button class="icon-close" type="button" aria-label="Закрыть" @click="closeModal">x</button>
        <span class="prompt-icon"><AppIcon name="apply" /></span>
        <p class="eyebrow">Доступ на Amy</p>
        <h2 id="rp-prompt-title">У вас ещё нет RP-анкеты</h2>
        <p class="modal-description">Заполните анкету персонажа, чтобы команда могла рассмотреть вашу заявку на сервер.</p>
        <div class="modal-actions">
          <button class="secondary-button" type="button" @click="closeModal">Позже</button>
          <button class="primary-button" type="button" @click="openApplication">Написать анкету</button>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import AppIcon from '~/components/AppIcon.vue'
import { useAuth } from '~/composables/useAuth'

const router = useRouter()
const { authenticated, user, pending, loginUrl } = useAuth()
const noticeDismissed = ref(false)
const showApplicationModal = ref(false)
const showLoginNotice = computed(() => !pending.value && !authenticated.value && !noticeDismissed.value)

const dismissNotice = () => {
  noticeDismissed.value = true
  sessionStorage.setItem('amy:rp-login-notice-dismissed', '1')
}

const startDiscordLogin = () => {
  sessionStorage.setItem('amy:post-login-action', 'rp-application')
  window.location.assign(loginUrl.value)
}

const closeModal = () => {
  showApplicationModal.value = false
  sessionStorage.setItem('amy:rp-application-prompted', '1')
}

const openApplication = async () => {
  if (!user.value?.id) return
  showApplicationModal.value = false
  sessionStorage.removeItem('amy:post-login-action')
  sessionStorage.setItem('amy:rp-application-prompted', '1')
  await router.push(`/u/${user.value.id}?apply=1`)
}

const syncPrompt = () => {
  if (pending.value || !authenticated.value || !user.value?.id || user.value.rpApplication) return
  const returnedFromLogin = sessionStorage.getItem('amy:post-login-action') === 'rp-application'
  const prompted = sessionStorage.getItem('amy:rp-application-prompted') === '1'
  if (returnedFromLogin) sessionStorage.removeItem('amy:post-login-action')
  if (returnedFromLogin || !prompted) showApplicationModal.value = true
}

onMounted(() => {
  noticeDismissed.value = sessionStorage.getItem('amy:rp-login-notice-dismissed') === '1'
  syncPrompt()
})

watch([pending, authenticated, () => user.value?.id, () => user.value?.rpApplication?.id], syncPrompt)
</script>

<style scoped>
.login-notice {
  position: fixed;
  right: 24px;
  bottom: 24px;
  z-index: 70;
  width: min(390px, calc(100vw - 32px));
  display: grid;
  grid-template-columns: 46px 1fr;
  gap: 14px;
  padding: 18px;
  color: var(--text);
  background: rgba(20, 20, 27, 0.96);
  border: 1px solid rgba(240, 90, 60, 0.48);
  border-left: 4px solid var(--accent);
  border-radius: 8px;
  box-shadow: 0 22px 60px rgba(0, 0, 0, 0.52);
  backdrop-filter: blur(16px);
}

.prompt-icon {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border-radius: 8px;
  color: var(--accent);
  background: rgba(240, 90, 60, 0.12);
  border: 1px solid rgba(240, 90, 60, 0.28);
  font-size: 23px;
}

.notice-copy {
  display: grid;
  gap: 8px;
  padding-right: 18px;
}

.notice-copy strong { font-size: 17px; }
.notice-copy p,
.modal-description { margin: 0; color: var(--muted); line-height: 1.5; }

.icon-close {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 30px;
  height: 30px;
  border: 0;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  font-size: 18px;
}

.primary-button,
.secondary-button {
  min-height: 40px;
  border-radius: 8px;
  padding: 9px 14px;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.primary-button {
  border: 0;
  color: #0b0b0f;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.application-modal {
  position: fixed;
  inset: 0;
  z-index: 90;
  display: grid;
  place-items: center;
  padding: 16px;
}

.modal-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(5, 5, 8, 0.76);
  backdrop-filter: blur(7px);
}

.modal-panel {
  position: relative;
  z-index: 1;
  width: min(480px, 100%);
  display: grid;
  gap: 14px;
  padding: 26px;
  color: var(--text);
  background: rgba(20, 20, 27, 0.98);
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-top: 3px solid var(--accent);
  border-radius: 8px;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.68);
}

.modal-panel h2,
.eyebrow { margin: 0; }
.eyebrow { color: var(--accent); font-size: 12px; font-weight: 800; text-transform: uppercase; }
.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 6px; }
.secondary-button { color: var(--text); background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.14); }

.prompt-slide-enter-active,
.prompt-slide-leave-active { transition: opacity 180ms ease, transform 180ms ease; }
.prompt-slide-enter-from,
.prompt-slide-leave-to { opacity: 0; transform: translateY(14px); }

@media (max-width: 640px) {
  .login-notice { right: 16px; bottom: 16px; }
  .modal-actions { flex-direction: column-reverse; }
}
</style>
