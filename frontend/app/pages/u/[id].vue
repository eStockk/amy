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
            <span class="presence"></span>
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
          </div>

          <div class="actions">
            <button class="ghost" type="button" @click="copyProfileLink">
              {{ copied ? 'Ссылка скопирована' : 'Копировать ссылку профиля' }}
            </button>
            <button v-if="isOwner" class="ghost danger" type="button" @click="logoutAndBack">Выйти</button>
          </div>
        </article>

        <article class="panel summary-card">
          <p class="eyebrow">Статус RP-заявки</p>
          <h3>{{ applicationStatusLabel }}</h3>
          <p class="muted" v-if="isOwner && applicationSummary">Ник в заявке: {{ applicationSummary.nickname }}</p>
          <p class="muted" v-if="isOwner && applicationSummary?.updatedAt">
            Обновлено: {{ formatDate(applicationSummary.updatedAt) }}
          </p>
          <p class="muted" v-if="!isOwner">Только владелец профиля может подавать и редактировать RP-заявку.</p>
        </article>
      </aside>

      <div class="right-col">
        <header class="panel hero-card">
          <div class="hero-copy">
            <p class="eyebrow">CentralFlow style</p>
            <h2>{{ isOwner ? 'Личный кабинет игрока' : `Публичный профиль ${profile.displayName}` }}</h2>
            <p class="muted">
              Верификация с сервером происходит по коду после кика. После верификации имя и фамилия из MineRP
              синхронизируются автоматически.
            </p>
          </div>
          <NuxtLink class="ghost" to="/">На главную</NuxtLink>
        </header>

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

          <div class="progress">
            <span :style="{ width: `${progressPercent}%` }"></span>
          </div>

          <p class="muted">
            Заполните анкету. Она будет отправлена в Discord-канал модерации с кнопками «Одобрить» и «Отклонить».
          </p>

          <form class="form-grid" @submit.prevent="submitApplication">
            <label>
              <span>1. Ваш ник в игре</span>
              <input v-model.trim="form.nickname" type="text" maxlength="16" required />
            </label>

            <label>
              <span>2. Откуда узнали о сервере (необязательно)</span>
              <input v-model.trim="form.source" type="text" />
            </label>

            <label>
              <span>3. Имя, фамилия (если имеется)</span>
              <input v-model.trim="form.rpName" type="text" />
            </label>

            <label>
              <span>4. Дата рождения</span>
              <input v-model="form.birthDate" type="date" required />
            </label>

            <label>
              <span>5. Раса</span>
              <input v-model.trim="form.race" type="text" required />
            </label>

            <label>
              <span>6. Пол</span>
              <input v-model.trim="form.gender" type="text" required />
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
              <span v-if="applicationLocked" class="muted">У вас уже есть заявка в статусе «на рассмотрении».</span>
            </div>
          </form>

          <p v-if="submitMessage" class="status" :class="{ error: submitError }">{{ submitMessage }}</p>
        </section>
      </div>
    </section>
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
  verifyMinecraftCode,
  logout
} = useAuth()

const pending = ref(true)
const errorMessage = ref('')
const copied = ref(false)

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
  if (!profile.value) return 'Не указан'
  const full = `${profile.value.rpFirstName || ''} ${profile.value.rpLastName || ''}`.trim()
  return full || 'Не указан'
})

const applicationStatusLabel = computed(() => {
  if (!isOwner.value) return 'Недоступно для просмотра'

  const status = applicationSummary.value?.status
  if (status === 'approved') return 'Одобрена'
  if (status === 'rejected') return 'Отклонена'
  if (status === 'pending') return 'На рассмотрении'
  return 'Заявка не отправлена'
})

const applicationLocked = computed(() => applicationSummary.value?.status === 'pending')

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

const profileLinkAbsolute = computed(() => {
  if (!profile.value?.id) return ''
  if (!process.client) return `/u/${profile.value.id}`
  return `${window.location.origin}/u/${profile.value.id}`
})

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
    await refresh()

    const response = await $fetch<PublicProfileResponse>(`${config.public.apiBase}/profiles/${profileId.value}`, {
      credentials: 'include'
    })

    profile.value = response.profile

    if (isOwner.value && user.value) {
      profile.value = {
        ...response.profile,
        linkedMinecraft: user.value.linkedMinecraft || response.profile.linkedMinecraft,
        rpFirstName: user.value.rpFirstName || response.profile.rpFirstName,
        rpLastName: user.value.rpLastName || response.profile.rpLastName
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

const copyProfileLink = async () => {
  copied.value = false
  if (!process.client || !profileLinkAbsolute.value) return

  try {
    await navigator.clipboard.writeText(profileLinkAbsolute.value)
    copied.value = true
  } catch {
    copied.value = false
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

  if (applicationLocked.value) {
    submitError.value = true
    submitMessage.value = 'Текущая заявка еще рассматривается администрацией.'
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
    await loadProfile()
  } catch (error: unknown) {
    submitError.value = true
    const message = (error as { data?: { error?: string } })?.data?.error
    submitMessage.value = message || 'Не удалось отправить заявку. Попробуйте позже.'
  } finally {
    submitPending.value = false
  }
}

const logoutAndBack = async () => {
  await logout()
  await navigateTo('/')
}

onMounted(() => {
  void loadProfile()
})

watch(
  () => route.params.id,
  () => {
    void loadProfile()
  }
)
</script>

<style scoped>
.profile-page {
  min-height: calc(100vh - 220px);
  display: grid;
}

.shell {
  display: grid;
  grid-template-columns: minmax(280px, 330px) minmax(0, 1fr);
  gap: 18px;
}

.left-col,
.right-col {
  display: grid;
  gap: 14px;
  align-content: start;
}

.panel {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 20px;
  background: radial-gradient(100% 120% at 10% 0%, rgba(150, 118, 248, 0.16), transparent 60%),
    linear-gradient(165deg, rgba(20, 20, 32, 0.95), rgba(11, 12, 20, 0.98));
  backdrop-filter: blur(8px);
  box-shadow: 0 18px 36px rgba(0, 0, 0, 0.36);
}

.state-card {
  margin: auto;
  width: min(620px, 100%);
  display: grid;
  gap: 10px;
  padding: 28px;
}

.identity-card {
  padding: 18px;
  display: grid;
  gap: 10px;
}

.avatar-box {
  position: relative;
  width: 88px;
  height: 88px;
}

.avatar {
  width: 88px;
  height: 88px;
  border-radius: 24px;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.24);
  background: rgba(255, 255, 255, 0.08);
}

.presence {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #36ef88;
  border: 3px solid #171a26;
  box-shadow: 0 0 12px rgba(54, 239, 136, 0.55);
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: rgba(255, 255, 255, 0.72);
}

h1,
h2,
h3,
p {
  margin: 0;
}

h1 {
  font-size: clamp(24px, 3vw, 34px);
}

h2 {
  font-size: clamp(20px, 3vw, 30px);
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
  border-radius: 999px;
  padding: 6px 10px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  background: rgba(255, 255, 255, 0.08);
  font-size: 12px;
}

.actions {
  display: grid;
  gap: 8px;
}

.summary-card {
  padding: 14px 16px;
  display: grid;
  gap: 8px;
}

.hero-card {
  padding: 18px;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.hero-copy {
  display: grid;
  gap: 8px;
}

.info-grid {
  padding: 14px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.info-card {
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
  padding: 14px;
  display: grid;
  gap: 8px;
}

.info-card.highlight {
  background: linear-gradient(145deg, rgba(157, 118, 248, 0.25), rgba(12, 13, 22, 0.95));
}

.label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgba(255, 255, 255, 0.68);
}

.info-card strong {
  font-size: 21px;
  font-family: 'Neue Machine', 'Montserrat', sans-serif;
}

.verify-card,
.application-card {
  padding: 16px;
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
  border-radius: 999px;
  border: 1px solid rgba(157, 118, 248, 0.36);
  background: rgba(157, 118, 248, 0.16);
  color: #e2d5ff;
  font-size: 11px;
  padding: 6px 10px;
}

.progress {
  width: 100%;
  height: 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.progress span {
  display: block;
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #9d76f8, #cf89ff);
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
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.15);
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

.actions-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.ghost,
.primary {
  border-radius: 999px;
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
  background: rgba(255, 255, 255, 0.07);
  border-color: rgba(255, 255, 255, 0.14);
}

.ghost.danger {
  color: #ffbaa9;
  border-color: rgba(228, 94, 56, 0.42);
  background: rgba(228, 94, 56, 0.16);
}

.primary {
  color: #0b0b0f;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.status {
  color: #8af4ad;
}

.status.error {
  color: #ff9f9f;
}

@media (max-width: 1080px) {
  .shell {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .hero-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .info-grid,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .row {
    grid-template-columns: 1fr;
  }
}
</style>




