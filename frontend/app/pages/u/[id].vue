<template>
  <div class="profile-page">
    <section v-if="pending" class="panel state-card">
      <h1>Загружаем профиль...</h1>
      <p class="muted">Подготавливаем данные аккаунта.</p>
    </section>

    <section v-else-if="errorMessage" class="panel state-card">
      <h1>Профиль не найден</h1>
      <p class="muted">{{ errorMessage }}</p>
      <NuxtLink class="ghost" to="/">На главную</NuxtLink>
    </section>

    <section v-else-if="profile" class="shell">
      <aside class="left-col">
        <article class="panel identity-card">
          <div class="avatar-box">
            <img class="avatar" :src="profile.avatarUrl || logo" alt="avatar" />
            <span class="presence" :class="{ offline: !profile.isOnline }"></span>
          </div>

          <p class="eyebrow">Профиль игрока</p>
          <h1>{{ profile.displayName }}</h1>
          <p class="muted">@{{ profile.username }}</p>
          <p v-if="isOwner && user?.email" class="muted email">{{ user.email }}</p>

          <div class="chips">
            <span class="chip" v-if="profile.linkedMinecraft">Minecraft: {{ profile.linkedMinecraft }}</span>
            <span class="chip" v-else>Minecraft не верифицирован</span>
            <span class="chip" v-if="fullRPName !== 'Не указан'">RP: {{ fullRPName }}</span>
            <span class="chip" v-if="profile.joinedAt">На сайте с {{ formatDate(profile.joinedAt, true) }}</span>
            <span class="chip" v-else>Дата регистрации обновится после входа</span>
            <span class="chip">{{ onlineStatusLabel }}</span>
          </div>

          <div class="actions" v-if="isOwner">
            <button class="ghost danger" type="button" @click="logoutAndBack">Выйти</button>
          </div>
        </article>

        <article class="panel summary-card">
          <p class="eyebrow">Статус RP-заявки</p>
          <h3>{{ applicationStatusLabel }}</h3>
          <p class="muted" v-if="isOwner && applicationSummary">Ник в заявке: {{ applicationSummary.nickname }}</p>
          <p class="muted" v-if="isOwner && applicationSummary?.updatedAt">
            Обновлено: {{ formatDate(applicationSummary.updatedAt) }}
          </p>
          <button
            v-if="isOwner && applicationSummary && canDeleteApplication"
            class="ghost danger"
            type="button"
            :disabled="deletePending"
            @click="deleteApplication"
          >
            {{ deletePending ? 'Удаляем...' : 'Удалить RP-тикет' }}
          </button>
          <p v-if="deleteMessage" class="status" :class="{ error: deleteError }">{{ deleteMessage }}</p>
          <p class="muted" v-if="!isOwner">Только владелец профиля может подавать и редактировать RP-заявку.</p>
        </article>
      </aside>

      <div class="right-col">
        <section class="panel info-grid">
          <article class="info-card highlight">
            <p class="label">Аккаунт Minecraft</p>
            <strong>{{ profile.linkedMinecraft || 'Ожидает верификации' }}</strong>
            <p class="muted">После успешного ввода кода аккаунт закрепляется за вашим Discord-профилем.</p>
          </article>

          <article class="info-card">
            <p class="label">RP имя и фамилия</p>
            <strong>{{ fullRPName }}</strong>
            <p class="muted">Сервер обновляет эти поля автоматически через API после настройки в MineRP.</p>
          </article>
        </section>

        <section v-if="isOwner" class="panel verify-card">
          <div class="section-head">
            <h3>Шаг 1: Верификация с сервером</h3>
            <span class="badge">Код из кика</span>
          </div>
          <p class="muted">Введите код, который вы получили после первого входа на сервер.</p>

          <form class="row" @submit.prevent="submitVerificationCode">
            <input
              v-model.trim="verificationCode"
              type="text"
              maxlength="16"
              placeholder="Например: A4K8M2Q9"
              required
            />
            <button class="primary" type="submit" :disabled="verifyPending">
              {{ verifyPending ? 'Проверяем...' : 'Подтвердить код' }}
            </button>
          </form>
          <p v-if="verifyMessage" class="status" :class="{ error: verifyError }">{{ verifyMessage }}</p>
        </section>

        <section v-if="isOwner" class="panel application-card">
          <div class="section-head">
            <h3>Шаг 2: RP-заявка</h3>
            <span class="badge">{{ progressCompleted }}/10</span>
          </div>

          <p class="muted">Форма заполняется в модальном окне и отправляется в Discord-канал модерации.</p>

          <div class="application-actions">
            <button class="primary" type="button" :disabled="applicationAccepted" @click="openRpModal">
              {{ applicationSummary ? openApplicationLabel : createApplicationLabel }}
            </button>
            <span v-if="applicationAccepted" class="muted">{{ acceptedHint }}</span>
            <span v-else-if="applicationLocked" class="muted">{{ pendingHint }}</span>
          </div>

          <p v-if="submitMessage" class="status" :class="{ error: submitError }">{{ submitMessage }}</p>
        </section>
      </div>
    </section>

    <Teleport to="body">
      <div v-if="isOwner && rpModalOpen" class="modal-backdrop" @click.self="closeRpModal">
        <section class="panel modal-window" role="dialog" aria-modal="true" aria-label="RP-заявка">
          <header class="modal-head">
            <div>
              <p class="eyebrow">RP-заявка</p>
              <h3>Анкета игрока</h3>
            </div>
            <button class="ghost" type="button" @click="closeRpModal">Закрыть</button>
          </header>

          <div class="progress">
            <span :style="{ width: `${progressPercent}%` }"></span>
          </div>

          <form class="form-grid" @submit.prevent="submitApplication">
            <label>
              <span>1. Ваш ник в игре</span>
              <input v-model.trim="form.nickname" type="text" maxlength="16" required />
            </label>

            <label>
              <span>2. Откуда узнали о сервере (необязательно)</span>
              <input v-model.trim="form.source" type="text" maxlength="200" />
            </label>

            <label>
              <span>3. Имя, фамилия (если имеется)</span>
              <input v-model.trim="form.rpName" type="text" maxlength="120" />
            </label>

            <label>
              <span>4. Дата рождения</span>
              <input v-model="form.birthDate" type="date" required />
            </label>

            <label>
              <span>5. Раса</span>
              <input v-model.trim="form.race" type="text" maxlength="80" required />
            </label>

            <label>
              <span>6. Пол</span>
              <input v-model.trim="form.gender" type="text" maxlength="80" required />
            </label>

            <label class="wide">
              <span>7. Перечисли ключевые навыки персонажа и их пользу для RP</span>
              <textarea v-model.trim="form.skills" rows="4" required></textarea>
            </label>

            <label class="wide">
              <span>8. План развития персонажа в RP</span>
              <textarea v-model.trim="form.plan" rows="4" required></textarea>
            </label>

            <label class="wide">
              <span>9. Биография (минимум 5 предложений)</span>
              <textarea v-model.trim="form.biography" rows="6" required></textarea>
            </label>

            <label class="wide">
              <span>10. Ссылка на скин (только безопасный HTTPS URL на png/jpg/webp)</span>
              <input v-model.trim="form.skinUrl" type="url" required />
            </label>

            <div class="wide actions-row">
              <button class="primary" type="submit" :disabled="submitPending || applicationLocked">
                {{ submitPending ? 'Отправляем...' : 'Отправить RP-заявку' }}
              </button>
              <button class="ghost" type="button" @click="closeRpModal">Отмена</button>
            </div>
          </form>

          <p v-if="submitMessage" class="status" :class="{ error: submitError }">{{ submitMessage }}</p>
        </section>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import logo from '~/assets/amy-logo.png'
import type { AuthUser } from '~/composables/useAuth'
import { useAuth } from '~/composables/useAuth'

type PublicProfile = {
  id: string
  username: string
  displayName: string
  avatarUrl?: string
  linkedMinecraft?: string
  rpFirstName?: string
  rpLastName?: string
  joinedAt?: string
  isOnline?: boolean
}

type PublicProfileResponse = {
  profile: PublicProfile
}

const route = useRoute()
const config = useRuntimeConfig()

const {
  authenticated,
  user,
  refresh,
  submitRPApplication,
  deleteRPApplication,
  verifyMinecraftCode,
  logout
} = useAuth()

const pending = ref(true)
const errorMessage = ref('')

const profile = ref<PublicProfile | null>(null)
const profileId = computed(() => String(route.params.id || '').trim())
const isOwner = computed(() => Boolean(authenticated.value && user.value?.id === profile.value?.id))

const applicationSummary = computed(() => (isOwner.value ? user.value?.rpApplication : undefined))

const verificationCode = ref('')
const verifyPending = ref(false)
const verifyMessage = ref('')
const verifyError = ref(false)

const submitPending = ref(false)
const submitMessage = ref('')
const submitError = ref(false)

const deletePending = ref(false)
const deleteMessage = ref('')
const deleteError = ref(false)

const rpModalOpen = ref(false)

const form = reactive({
  nickname: '',
  source: '',
  rpName: '',
  birthDate: '',
  race: '',
  gender: '',
  skills: '',
  plan: '',
  biography: '',
  skinUrl: ''
})

const fullRPName = computed(() => {
  if (!profile.value) return '\u041d\u0435 \u0443\u043a\u0430\u0437\u0430\u043d'
  const full = `${profile.value.rpFirstName || ''} ${profile.value.rpLastName || ''}`.trim()
  return full || '\u041d\u0435 \u0443\u043a\u0430\u0437\u0430\u043d'
})

const onlineStatusLabel = computed(() =>
  profile.value?.isOnline
    ? '\u0421\u0435\u0439\u0447\u0430\u0441 \u043d\u0430 \u0441\u0430\u0439\u0442\u0435'
    : '\u0421\u0435\u0439\u0447\u0430\u0441 \u043d\u0435 \u043d\u0430 \u0441\u0430\u0439\u0442\u0435'
)

const statusLabelPublic = '\u041f\u0443\u0431\u043b\u0438\u0447\u043d\u044b\u0439 \u0432\u0438\u0434 \u043f\u0440\u043e\u0444\u0438\u043b\u044f'
const statusLabelAccepted = '\u041f\u0440\u0438\u043d\u044f\u0442\u0430'
const statusLabelCanceled = '\u041e\u0442\u043c\u0435\u043d\u0435\u043d\u0430'
const statusLabelPending = '\u041d\u0430 \u0440\u0430\u0441\u0441\u043c\u043e\u0442\u0440\u0435\u043d\u0438\u0438'
const statusLabelMissing = '\u0415\u0449\u0435 \u043d\u0435 \u043e\u0442\u043f\u0440\u0430\u0432\u043b\u0435\u043d\u0430'

const applicationStatusLabel = computed(() => {
  if (!isOwner.value) return statusLabelPublic

  const status = applicationSummary.value?.status
  if (status === 'accepted' || status === 'approved') return statusLabelAccepted
  if (status === 'canceled' || status === 'rejected') return statusLabelCanceled
  if (status === 'pending') return statusLabelPending
  return statusLabelMissing
})

const applicationLocked = computed(() => {
  const status = applicationSummary.value?.status
  return status === 'pending' || status === 'accepted' || status === 'approved'
})

const applicationAccepted = computed(() => {
  const status = applicationSummary.value?.status
  return status === 'accepted' || status === 'approved'
})

const canDeleteApplication = computed(() => {
  const status = applicationSummary.value?.status
  return status !== 'accepted' && status !== 'approved'
})

const acceptedHint = '\u0417\u0430\u044f\u0432\u043a\u0430 \u043f\u0440\u0438\u043d\u044f\u0442\u0430. \u041f\u043e\u0432\u0442\u043e\u0440\u043d\u0430\u044f \u043e\u0442\u043f\u0440\u0430\u0432\u043a\u0430 \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u0430.'
const pendingHint = '\u0423 \u0432\u0430\u0441 \u0443\u0436\u0435 \u0435\u0441\u0442\u044c \u0437\u0430\u044f\u0432\u043a\u0430 \u0432 \u0441\u0442\u0430\u0442\u0443\u0441\u0435 \u00ab\u043d\u0430 \u0440\u0430\u0441\u0441\u043c\u043e\u0442\u0440\u0435\u043d\u0438\u0438\u00bb.'
const openApplicationLabel = '\u041e\u0442\u043a\u0440\u044b\u0442\u044c \u0437\u0430\u044f\u0432\u043a\u0443'
const createApplicationLabel = '\u0421\u043e\u0437\u0434\u0430\u0442\u044c RP-\u0437\u0430\u044f\u0432\u043a\u0443'
const acceptedSubmitErrorText = '\u0417\u0430\u044f\u0432\u043a\u0430 \u0443\u0436\u0435 \u043f\u0440\u0438\u043d\u044f\u0442\u0430. \u041f\u043e\u0432\u0442\u043e\u0440\u043d\u0430\u044f \u043e\u0442\u043f\u0440\u0430\u0432\u043a\u0430 \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u0430.'
const pendingSubmitErrorText = '\u0422\u0435\u043a\u0443\u0449\u0430\u044f \u0437\u0430\u044f\u0432\u043a\u0430 \u0435\u0449\u0435 \u0440\u0430\u0441\u0441\u043c\u0430\u0442\u0440\u0438\u0432\u0430\u0435\u0442\u0441\u044f \u0430\u0434\u043c\u0438\u043d\u0438\u0441\u0442\u0440\u0430\u0446\u0438\u0435\u0439.'

const progressCompleted = computed(() => {
  const fields = [
    form.nickname,
    form.source,
    form.rpName,
    form.birthDate,
    form.race,
    form.gender,
    form.skills,
    form.plan,
    form.biography,
    form.skinUrl
  ]
  return fields.filter((item) => String(item || '').trim().length > 0).length
})

const progressPercent = computed(() => Math.round((progressCompleted.value / 10) * 100))

const fillFromCurrentState = (authUser?: AuthUser | null) => {
  if (!authUser) return

  if (authUser.linkedMinecraft) form.nickname = authUser.linkedMinecraft
  if (authUser.rpApplication?.nickname) form.nickname = authUser.rpApplication.nickname
  if (authUser.rpApplication?.rpName) form.rpName = authUser.rpApplication.rpName
  if (authUser.rpApplication?.birthDate) form.birthDate = authUser.rpApplication.birthDate
  if (authUser.rpApplication?.race) form.race = authUser.rpApplication.race
  if (authUser.rpApplication?.gender) form.gender = authUser.rpApplication.gender
}

const formatDate = (raw?: string, dateOnly = false) => {
  if (!raw) return '-'
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw

  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: 'long',
    year: 'numeric',
    ...(dateOnly
      ? {}
      : {
          hour: '2-digit',
          minute: '2-digit'
        })
  }).format(parsed)
}

const loadProfile = async () => {
  if (!profileId.value) {
    errorMessage.value = 'Некорректная ссылка профиля.'
    pending.value = false
    return
  }

  pending.value = true
  errorMessage.value = ''

  try {
    try {
      await refresh()
    } catch {
      // Even if auth session expired, public profile must still load.
    }

    const response = await $fetch<PublicProfileResponse>(`${config.public.apiBase}/profiles/${profileId.value}`, {
      credentials: 'include'
    })

    profile.value = response.profile

    if (isOwner.value && user.value) {
      profile.value = {
        ...response.profile,
        linkedMinecraft: user.value.linkedMinecraft || response.profile.linkedMinecraft,
        rpFirstName: user.value.rpFirstName || response.profile.rpFirstName,
        rpLastName: user.value.rpLastName || response.profile.rpLastName,
        joinedAt: response.profile.joinedAt
      }
      fillFromCurrentState(user.value)
    }
  } catch (error: unknown) {
    const message = (error as { data?: { error?: string } })?.data?.error
    errorMessage.value = message || 'Не удалось загрузить профиль.'
    profile.value = null
  } finally {
    pending.value = false
  }
}

const submitVerificationCode = async () => {
  verifyMessage.value = ''
  verifyError.value = false

  const code = verificationCode.value.trim().toUpperCase()
  if (code.length < 6) {
    verifyError.value = true
    verifyMessage.value = 'Введите корректный код верификации.'
    return
  }

  verifyPending.value = true
  try {
    await verifyMinecraftCode(code)
    verifyMessage.value = 'Код принят, аккаунт сервера привязан.'
    verificationCode.value = ''
    await loadProfile()
  } catch (error: unknown) {
    verifyError.value = true
    const message = (error as { data?: { error?: string } })?.data?.error
    verifyMessage.value = message || 'Не удалось подтвердить код. Проверьте его и повторите попытку.'
  } finally {
    verifyPending.value = false
  }
}

const validateSkinUrl = (raw: string) => {
  try {
    const parsed = new URL(raw)
    if (parsed.protocol !== 'https:') return false

    const host = parsed.hostname.toLowerCase()
    if (!host || host === 'localhost' || host.includes('..')) return false

    const isIPv4 = /^\d{1,3}(\.\d{1,3}){3}$/.test(host)
    if (isIPv4) {
      const octets = host.split('.').map(Number)
      if (octets.some((item) => Number.isNaN(item) || item < 0 || item > 255)) return false

      const [a, b] = octets
      const privateRange =
        a === 10 || a === 127 || (a === 192 && b === 168) || (a === 172 && b >= 16 && b <= 31)
      if (privateRange) return false
    }

    return /(\.png|\.jpg|\.jpeg|\.webp)$/i.test(parsed.pathname)
  } catch {
    return false
  }
}

const countSentences = (text: string) => (text.match(/[.!?]/g) || []).length

const submitApplication = async () => {
  submitMessage.value = ''
  submitError.value = false

  if (applicationAccepted.value) {
    submitError.value = true
    submitMessage.value = acceptedSubmitErrorText
    return
  }

  if (applicationLocked.value) {
    submitError.value = true
    submitMessage.value = pendingSubmitErrorText
    return
  }

  if (!/^[A-Za-z0-9_]{3,16}$/.test(form.nickname)) {
    submitError.value = true
    submitMessage.value = 'Ник должен содержать 3-16 символов: латиница, цифры или _.'
    return
  }

  if (!form.birthDate || !form.race || !form.gender || !form.skills || !form.plan || !form.biography || !form.skinUrl) {
    submitError.value = true
    submitMessage.value = 'Заполните все обязательные поля заявки.'
    return
  }

  if (countSentences(form.biography) < 5) {
    submitError.value = true
    submitMessage.value = 'Биография должна содержать минимум 5 предложений.'
    return
  }

  if (!validateSkinUrl(form.skinUrl)) {
    submitError.value = true
    submitMessage.value = 'Ссылка на скин не прошла проверку безопасности.'
    return
  }

  submitPending.value = true
  try {
    await submitRPApplication({
      nickname: form.nickname,
      source: form.source,
      rpName: form.rpName,
      birthDate: form.birthDate,
      race: form.race,
      gender: form.gender,
      skills: form.skills,
      plan: form.plan,
      biography: form.biography,
      skinUrl: form.skinUrl
    })

    submitMessage.value = 'Заявка отправлена. Ожидайте решения администрации.'
    rpModalOpen.value = false
    await loadProfile()
  } catch (error: unknown) {
    submitError.value = true
    const message = (error as { data?: { error?: string } })?.data?.error
    submitMessage.value = message || 'Не удалось отправить заявку. Попробуйте позже.'
  } finally {
    submitPending.value = false
  }
}

const deleteApplication = async () => {
  if (!applicationSummary.value?.id) return

  deleteMessage.value = ''
  deleteError.value = false
  deletePending.value = true

  try {
    await deleteRPApplication(applicationSummary.value.id)
    deleteMessage.value = 'RP-тикет удален с сайта и из Discord.'
    await loadProfile()
  } catch (error: unknown) {
    deleteError.value = true
    const message = (error as { data?: { error?: string } })?.data?.error
    deleteMessage.value = message || 'Не удалось удалить RP-тикет.'
  } finally {
    deletePending.value = false
  }
}

const logoutAndBack = async () => {
  await logout()
  await navigateTo('/')
}


const openRpModal = () => {
  submitMessage.value = ''
  submitError.value = false
  rpModalOpen.value = true
}

const closeRpModal = () => {
  rpModalOpen.value = false
}

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && rpModalOpen.value) {
    rpModalOpen.value = false
  }
}

onMounted(() => {
  if (import.meta.client) {
    window.addEventListener('keydown', handleEscape)
  }
  void loadProfile()
})

onBeforeUnmount(() => {
  if (import.meta.client) {
    window.removeEventListener('keydown', handleEscape)
    document.body.style.overflow = ''
  }
})

watch(
  () => route.params.id,
  () => {
    void loadProfile()
  }
)

watch(rpModalOpen, (opened) => {
  if (!import.meta.client) return
  document.body.style.overflow = opened ? 'hidden' : ''
})
</script>

<style scoped>
.profile-page {
  min-height: calc(100vh - 220px);
  display: grid;
}

.shell {
  display: grid;
  grid-template-columns: minmax(280px, 330px) minmax(0, 1fr);
  gap: 16px;
}

.left-col,
.right-col {
  display: grid;
  gap: 12px;
  align-content: start;
}

.panel {
  border: 1px solid rgba(228, 94, 56, 0.24);
  border-radius: 8px;
  background: radial-gradient(circle at 0% 0%, rgba(228, 94, 56, 0.2), transparent 45%),
    linear-gradient(160deg, rgba(22, 16, 15, 0.95), rgba(12, 11, 14, 0.98));
  backdrop-filter: blur(10px);
  box-shadow: 0 14px 32px rgba(0, 0, 0, 0.4);
}

.state-card {
  margin: auto;
  width: min(620px, 100%);
  display: grid;
  gap: 10px;
  padding: 20px;
}

.identity-card,
.summary-card,
.verify-card,
.application-card {
  padding: 14px;
}

.identity-card {
  display: grid;
  gap: 10px;
}

.avatar-box {
  position: relative;
  width: 84px;
  height: 84px;
}

.avatar {
  width: 84px;
  height: 84px;
  border-radius: 8px;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.24);
  background: rgba(255, 255, 255, 0.08);
}

.presence {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #36ef88;
  border: 3px solid #171a26;
  box-shadow: 0 0 10px rgba(54, 239, 136, 0.5);
}

.presence.offline {
  background: #e49a38;
  box-shadow: 0 0 10px rgba(228, 154, 56, 0.5);
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgba(255, 255, 255, 0.75);
}

h1,
h2,
h3,
p {
  margin: 0;
}

h1 {
  font-size: clamp(26px, 3vw, 36px);
}

.muted {
  color: var(--muted);
}

.email {
  font-size: 13px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.chip {
  border-radius: 6px;
  padding: 6px 10px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  font-size: 12px;
}

.actions {
  display: grid;
  gap: 8px;
}

.summary-card {
  display: grid;
  gap: 8px;
}

.info-grid {
  padding: 12px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.info-card {
  border-radius: 8px;
  border: 1px solid rgba(228, 94, 56, 0.26);
  background: rgba(228, 94, 56, 0.08);
  padding: 12px;
  display: grid;
  gap: 8px;
}

.info-card.highlight {
  background: linear-gradient(145deg, rgba(228, 94, 56, 0.22), rgba(20, 16, 16, 0.95));
  border-color: rgba(228, 94, 56, 0.34);
}

.label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgba(255, 255, 255, 0.7);
}

.info-card strong {
  font-size: 24px;
  font-family: 'Neue Machine', 'Montserrat', sans-serif;
}

.verify-card,
.application-card {
  display: grid;
  gap: 12px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.badge {
  border-radius: 6px;
  border: 1px solid rgba(228, 94, 56, 0.45);
  background: rgba(228, 94, 56, 0.16);
  color: #ffd4c8;
  font-size: 11px;
  padding: 6px 10px;
}

.progress {
  width: 100%;
  height: 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.progress span {
  display: block;
  height: 100%;
  border-radius: 6px;
  background: linear-gradient(90deg, #e45e38, #b8432c);
  transition: width 0.25s ease;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

label {
  display: grid;
  gap: 6px;
}

label span {
  font-size: 13px;
  color: var(--muted);
}

.wide {
  grid-column: 1 / -1;
}

input,
textarea {
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  background: rgba(255, 255, 255, 0.05);
  color: var(--text);
  padding: 10px 12px;
  font: inherit;
}

textarea {
  resize: vertical;
}

.row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
}

.application-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.actions-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.ghost,
.primary {
  border-radius: 6px;
  padding: 10px 16px;
  border: 1px solid transparent;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-weight: 600;
}

.ghost {
  color: var(--text);
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(255, 255, 255, 0.16);
}

.ghost.danger {
  color: #ffd3c9;
  border-color: rgba(228, 94, 56, 0.5);
  background: rgba(228, 94, 56, 0.15);
}

.primary {
  color: #0f0f12;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.status {
  color: #9ff4be;
}

.primary:disabled,
.ghost:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}


.status.error {
  color: #ff9f9f;
}


.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(7, 6, 9, 0.8);
  backdrop-filter: blur(6px);
  z-index: 60;
  display: grid;
  place-items: center;
  padding: 16px;
}

.modal-window {
  width: min(980px, 100%);
  max-height: calc(100dvh - 32px);
  overflow: auto;
  padding: 14px;
  display: grid;
  gap: 12px;
}

.modal-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

@media (max-width: 1100px) {
  .shell {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .profile-page {
    min-height: auto;
  }

  .info-grid,
  .form-grid,
  .row {
    grid-template-columns: 1fr;
  }

  .actions-row,
  .application-actions {
    align-items: stretch;
  }

  .actions-row .primary,
  .application-actions .primary,
  .row .primary {
    width: 100%;
  }

  .section-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .modal-window {
    max-height: 100dvh;
    border-radius: 0;
  }
}

@media (max-width: 640px) {
  .left-col,
  .right-col {
    gap: 10px;
  }

  .identity-card,
  .summary-card,
  .verify-card,
  .application-card,
  .info-grid {
    padding: 12px;
  }

  h1 {
    font-size: 30px;
  }

  .chips {
    gap: 6px;
  }

  .chip {
    font-size: 11px;
  }

  label span {
    font-size: 12px;
  }
}
</style>
