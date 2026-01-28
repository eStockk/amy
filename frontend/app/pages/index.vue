<template>
  <div class="landing">
    <div class="hero-row">
      <HeroPanel />
      <div class="side-stack">
        <TrailerPanel />
      </div>
    </div>

    <section class="news">
      <div class="news-header">
        <SectionTitle text="Новости сервера" />
      </div>

      <div v-if="pending" class="empty">
        <div class="empty-icon">⏳</div>
        <div>
          <strong>Загружаем новости</strong>
          <p>Проверяем ленту сервера.</p>
        </div>
      </div>

      <div v-else-if="isEmpty" class="empty">
        <div class="empty-icon">🛰️</div>
        <div>
          <strong>Пока новостей нет</strong>
          <p>Следите за обновлениями — скоро здесь появятся анонсы.</p>
        </div>
      </div>

      <div v-else class="news-grid">
        <NewsCard
          v-for="item in news"
          :key="item.id"
          :title="item.title"
          :intro="item.intro"
          :tags="item.tags"
          :variant="item.variant"
        />
      </div>
    </section>
    <footer class="footer">
    <div class="footer-top">
      <div class="footer-community">
        <h3>Общение</h3>
        <div class="discord-card">
          <span class="discord-title">Наш Discord сервер</span>
          <p>amy</p>
          <div class="discord-meta">0 сейчас участников в сети</div>
          <button class="discord-cta" type="button">Присоединиться</button>
        </div>
        <div class="socials">
          <a href="https://www.youtube.com" aria-label="YouTube" target="_blank" rel="noreferrer">YT</a>
          <a href="https://t.me" aria-label="Telegram" target="_blank" rel="noreferrer">TG</a>
          <a href="https://www.tiktok.com" aria-label="TikTok" target="_blank" rel="noreferrer">TT</a>
          <a href="https://www.twitch.tv" aria-label="Twitch" target="_blank" rel="noreferrer">TW</a>
          <a href="https://discord.gg" aria-label="Discord" target="_blank" rel="noreferrer">DC</a>
          <a href="https://boosty.to" aria-label="Boosty" target="_blank" rel="noreferrer">BS</a>
        </div>
      </div>

      <div class="footer-about">
        <h3>Играй в MineCraft на сервере Amy!</h3>
        <p>
          Это приватный сервер Minecraft с ванильным выживанием. Открытый мир,
          самописные плагины, голосовой чат и тёплое сообщество. Никаких донатов,
          экономики или телепортаций — только чистый геймплей, творчество и
          стабильность с 2019 года!
        </p>
        <div class="footer-logo">
          <img :src="logo" alt="Amy logo" />
        </div>
      </div>

      <div class="footer-rating">
        <div class="rating-badges">
          <a class="badge" href="https://mcrating.org" target="_blank" rel="noreferrer">mcrating.org</a>
          <a class="badge" href="https://mcmonitoring.net" target="_blank" rel="noreferrer">mcmonitoring.net</a>
        </div>
      </div>
    </div>

    <div class="footer-links">
      <h4>Ссылки</h4>
      <div class="links">
        <a href="#">Пользовательское соглашение</a>
        <a href="#">Политика конфиденциальности</a>
        <a href="#" target="_blank" rel="noreferrer">Магазин</a>
      </div>
    </div>

    <div class="footer-bottom">
      <span>© 2019-2026 Комплекс игровых серверов Minecraft “Amy”.</span>
    </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import HeroPanel from '~/components/HeroPanel.vue'
import TrailerPanel from '~/components/TrailerPanel.vue'
import NewsCard from '~/components/NewsCard.vue'
import SectionTitle from '~/components/SectionTitle.vue'
import { useNews } from '~/composables/useNews'
import logo from '~/assets/amy-logo.png'

const { news, pending } = useNews()
const isEmpty = computed(() => !pending.value && news.value.length === 0)
</script>

<style scoped>
.landing {
  display: flex;
  flex-direction: column;
  gap: 24px;
  min-height: 100%;
  flex: 1;
  position: relative;
}

.landing::before {
  content: '';
  position: absolute;
  inset: -80px -40px 0;
  background: radial-gradient(circle at 20% 20%, rgba(240, 90, 60, 0.18), transparent 60%),
    radial-gradient(circle at 80% 0%, rgba(198, 75, 122, 0.15), transparent 50%);
  filter: blur(20px);
  opacity: 0.8;
  z-index: 0;
}

.hero-row,
.news,
.footer {
  position: relative;
  z-index: 1;
}

.hero-row {
  display: grid;
  grid-template-columns: minmax(0, 2.2fr) minmax(0, 1fr);
  gap: 24px;
}

.side-stack {
  display: grid;
  gap: 16px;
}

.news {
  display: grid;
  gap: 16px;
}

.news-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.news-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.empty {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 20px 24px;
  border-radius: var(--radius-md);
  border: 1px dashed rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.03);
  color: var(--muted);
}

.empty-icon {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.16);
  font-size: 18px;
}

.footer {
  margin-top: auto;
  position: relative;
  z-index: 1;
  width: calc(100% + 96px);
  margin-left: -48px;
  display: grid;
  gap: 24px;
  padding: 32px 24px;
  border-top: 1px solid var(--stroke);
  background: rgba(18, 18, 22, 0.96);
  color: var(--muted);
}

.footer h3,
.footer h4 {
  margin: 0 0 8px;
  color: var(--text);
}

.footer-top {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr) minmax(0, 0.7fr);
  gap: 24px;
}

.footer-community {
  display: grid;
  gap: 14px;
}

.discord-card {
  padding: 16px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  display: grid;
  gap: 8px;
}

.discord-title {
  color: var(--text);
  font-weight: 600;
}

.discord-meta {
  font-size: 12px;
}

.discord-cta {
  width: max-content;
  padding: 8px 14px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  background: rgba(255, 255, 255, 0.05);
  color: var(--text);
  cursor: pointer;
}

.socials {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.socials a {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text);
  text-decoration: none;
  font-size: 12px;
}

.footer-about {
  display: grid;
  gap: 10px;
}

.footer-about p {
  margin: 0;
  line-height: 1.5;
}

.footer-logo {
  width: 96px;
  height: 96px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.1);
  display: grid;
  place-items: center;
}

.footer-logo img {
  width: 72px;
  height: 72px;
  object-fit: contain;
}

.footer-rating {
  display: grid;
  gap: 12px;
  align-content: start;
}

.rating-badges {
  display: grid;
  gap: 10px;
}

.badge {
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text);
  text-decoration: none;
  font-size: 13px;
}

.footer-links {
  display: grid;
  gap: 10px;
}

.links {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 18px;
}

.links a {
  color: var(--muted);
  text-decoration: none;
}

.links a:hover {
  color: var(--text);
}

.footer-bottom {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
  font-size: 12px;
}

@media (max-width: 1200px) {
  .hero-row {
    grid-template-columns: 1fr;
  }

  .news-grid {
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  }

  .footer-top {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .landing::before {
    animation: none;
  }
}
</style>
