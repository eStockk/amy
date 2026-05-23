<template>
  <div class="dashboard">
    <SectionTitle text="Dashboard" />

    <section v-if="pending" class="panel state">Загрузка dashboard...</section>
    <section v-else-if="error" class="panel state">
      Dashboard доступен только авторизованным модераторам.
    </section>

    <template v-else-if="data">
      <section class="panel players-block">
        <div class="block-head">
          <div>
            <p class="eyebrow">Игроки</p>
            <h2>{{ data.players.total }}</h2>
          </div>
        </div>

        <div class="players-grid">
          <article v-for="player in data.players.items" :key="player.discordId" class="player-card">
            <div class="skin-frame">
              <img v-if="player.skinUrl" :src="player.skinUrl" :alt="player.minecraftNick || player.discordName" />
              <span v-else>Skin</span>
            </div>
            <div class="player-info">
              <h3>{{ player.discordName }}</h3>
              <p>{{ player.minecraftNick || 'Minecraft ник не привязан' }}</p>
              <div class="status-grid">
                <span :class="['status-dot', player.siteOnline ? 'online' : 'offline']">Сайт</span>
                <span :class="['status-dot', player.minecraftOnline ? 'online' : 'offline']">Сервер</span>
                <span class="status-dot unknown">Discord</span>
              </div>
              <div class="meta-row">
                <span>Тикеты: {{ player.supportTickets }}</span>
                <span>Открыто: {{ player.openTickets }}</span>
              </div>
              <div class="meta-row">
                <span>RP: {{ statusLabel(player.rpStatus) }}</span>
              </div>
              <div class="roles">
                <span v-for="role in player.discordRoles" :key="role">{{ role }}</span>
                <span v-if="player.discordRoles.length === 0">Роли не загружены</span>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section class="panel tickets-block">
        <div class="block-head">
          <div>
            <p class="eyebrow">Техподдержка</p>
            <h2>{{ data.support.total }}</h2>
          </div>
          <div class="stats">
            <span>Нерешённых: {{ data.support.open }}</span>
            <span>Решённых: {{ data.support.resolved }}</span>
          </div>
          <div class="filters" aria-label="Фильтр тикетов">
            <button :class="{ active: ticketFilter === 'all' }" type="button" @click="ticketFilter = 'all'">Все</button>
            <button :class="{ active: ticketFilter === 'open' }" type="button" @click="ticketFilter = 'open'">Нерешённые</button>
            <button :class="{ active: ticketFilter === 'resolved' }" type="button" @click="ticketFilter = 'resolved'">Решённые</button>
          </div>
        </div>

        <div class="ticket-list">
          <article v-for="ticket in filteredTickets" :key="ticket.id" class="ticket-card">
            <div>
              <h3>{{ ticket.subject }}</h3>
              <p>{{ ticket.message }}</p>
              <div class="meta-row">
                <span>{{ ticket.discordNick }}</span>
                <span>{{ formatDate(ticket.createdAt) }}</span>
                <span>{{ ticket.category || 'Без категории' }}</span>
              </div>
            </div>
            <button class="status-button" type="button" @click="toggleTicket(ticket)">
              {{ ticket.status === 'resolved' ? 'Вернуть в работу' : 'Решена проблема' }}
            </button>
          </article>
        </div>
      </section>

      <section class="panel rp-block">
        <div class="block-head">
          <div>
            <p class="eyebrow">RP-заявки</p>
            <h2>{{ data.rp.total }}</h2>
          </div>
          <div class="stats">
            <span>Принятых: {{ data.rp.accepted }}</span>
            <span>Не принятых: {{ data.rp.other }}</span>
          </div>
          <div class="filters" aria-label="Фильтр RP-заявок">
            <button :class="{ active: rpFilter === 'all' }" type="button" @click="rpFilter = 'all'">Все</button>
            <button :class="{ active: rpFilter === 'accepted' }" type="button" @click="rpFilter = 'accepted'">Принятые</button>
            <button :class="{ active: rpFilter === 'other' }" type="button" @click="rpFilter = 'other'">Не принятые</button>
          </div>
        </div>

        <div class="application-list">
          <article v-for="application in filteredApplications" :key="application.id" class="application-row">
            <div>
              <h3>{{ application.minecraftNick }}</h3>
              <p>{{ application.discordName }}</p>
            </div>
            <span>{{ formatDate(application.createdAt) }}</span>
            <span>{{ statusLabel(application.status) }}</span>
            <span :class="['status-dot', application.siteOnline ? 'online' : 'offline']">Сайт</span>
            <span class="status-dot unknown">Discord</span>
          </article>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'

type DashboardPlayer = {
  discordId: string
  discordName: string
  minecraftNick: string
  skinUrl: string
  siteOnline: boolean
  minecraftOnline: boolean
  discordOnline: string
  supportTickets: number
  openTickets: number
  rpStatus: string
  discordRoles: string[]
}

type DashboardTicket = {
  id: number
  name: string
  discordNick: string
  subject: string
  category: string
  message: string
  status: 'open' | 'resolved'
  createdAt: string
}

type DashboardApplication = {
  id: string
  discordId: string
  discordName: string
  minecraftNick: string
  status: string
  siteOnline: boolean
  discordOnline: string
  createdAt: string
}

type DashboardData = {
  players: { total: number; items: DashboardPlayer[] }
  support: { total: number; open: number; resolved: number; items: DashboardTicket[] }
  rp: { total: number; accepted: number; other: number; items: DashboardApplication[] }
}

const config = useRuntimeConfig()
const ticketFilter = ref<'all' | 'open' | 'resolved'>('all')
const rpFilter = ref<'all' | 'accepted' | 'other'>('all')

const { data, pending, error, refresh } = useFetch<DashboardData>(() => `${config.public.apiBase}/dashboard`, {
  credentials: 'include',
  server: false
})

const filteredTickets = computed(() => {
  const tickets = data.value?.support.items ?? []
  if (ticketFilter.value === 'all') return tickets
  return tickets.filter((ticket) => ticket.status === ticketFilter.value)
})

const filteredApplications = computed(() => {
  const applications = data.value?.rp.items ?? []
  if (rpFilter.value === 'all') return applications
  return applications.filter((application) => {
    const accepted = normalizeStatus(application.status) === 'accepted'
    return rpFilter.value === 'accepted' ? accepted : !accepted
  })
})

const toggleTicket = async (ticket: DashboardTicket) => {
  const nextStatus = ticket.status === 'resolved' ? 'open' : 'resolved'
  await $fetch(`${config.public.apiBase}/dashboard/support/tickets/${ticket.id}`, {
    method: 'PATCH',
    credentials: 'include',
    body: { status: nextStatus }
  })
  await refresh()
}

const normalizeStatus = (status: string) => {
  if (status === 'approved') return 'accepted'
  if (status === 'rejected') return 'canceled'
  return status
}

const statusLabel = (status: string) => {
  const labels: Record<string, string> = {
    accepted: 'Принята',
    approved: 'Принята',
    pending: 'На рассмотрении',
    call: 'Созвон',
    canceled: 'Отменена',
    rejected: 'Отменена',
    missing: 'Нет заявки'
  }
  return labels[status] || status || '-'
}

const formatDate = (raw: string) => {
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return '-'
  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(parsed)
}
</script>

<style scoped>
.dashboard {
  display: grid;
  gap: 20px;
}

.panel {
  padding: 22px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: rgba(26, 27, 34, 0.92);
  display: grid;
  gap: 18px;
}

.state {
  color: var(--muted);
}

.block-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.eyebrow,
h2,
h3,
p {
  margin: 0;
}

.eyebrow {
  color: var(--accent);
  text-transform: uppercase;
  font-size: 12px;
  font-weight: 700;
}

h2 {
  font-size: 34px;
}

p {
  color: var(--muted);
}

.stats,
.filters,
.meta-row,
.roles {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.stats span,
.meta-row span,
.roles span {
  padding: 6px 9px;
  border-radius: 999px;
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.05);
  color: var(--muted);
  font-size: 12px;
}

.filters button,
.status-button {
  border: 1px solid var(--stroke);
  border-radius: 999px;
  padding: 8px 12px;
  color: var(--text);
  background: rgba(255, 255, 255, 0.06);
  cursor: pointer;
}

.filters button.active,
.status-button {
  color: #0b0b0f;
  border-color: transparent;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  font-weight: 700;
}

.players-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 14px;
}

.player-card {
  display: grid;
  grid-template-columns: 96px 1fr;
  gap: 14px;
  padding: 14px;
  min-height: 190px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.04);
}

.skin-frame {
  height: 164px;
  border-radius: 8px;
  display: grid;
  place-items: center;
  overflow: hidden;
  background: radial-gradient(circle at 50% 20%, rgba(228, 94, 56, 0.22), transparent 60%), #111118;
}

.skin-frame img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.player-info {
  display: grid;
  align-content: start;
  gap: 9px;
  min-width: 0;
}

.status-grid {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.status-dot {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
}

.status-dot::before {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: currentColor;
}

.status-dot.online {
  color: #7ddf9b;
}

.status-dot.offline {
  color: #ff8d7a;
}

.status-dot.unknown {
  color: #d9bd68;
}

.ticket-list,
.application-list {
  display: grid;
  gap: 10px;
}

.ticket-card,
.application-row {
  border-radius: var(--radius-sm);
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.04);
}

.ticket-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 14px;
  padding: 14px;
}

.application-row {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) 150px 130px 90px 90px;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
}

@media (max-width: 900px) {
  .ticket-card,
  .application-row {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .player-card {
    grid-template-columns: 1fr;
  }

  .skin-frame {
    height: 220px;
  }
}
</style>
